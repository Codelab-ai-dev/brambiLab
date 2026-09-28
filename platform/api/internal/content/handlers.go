package content

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

const maxBodyBytes = 1 << 20 // web-v1.md §7.1

var (
	idPattern          = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
	contentKinds       = set("project", "article", "log")
	locales            = set("es", "en")
)

// Handler serves /api/v1/admin/contents, categories and tags. Every route is wrapped by
// requireOwner (session + CSRF on mutations), and actor(r) names the owner for audit events.
type Handler struct {
	store        Store
	logger       *slog.Logger
	requireOwner func(http.Handler) http.Handler
	actor        func(*http.Request) string
}

func NewHandler(store Store, logger *slog.Logger, requireOwner func(http.Handler) http.Handler, actor func(*http.Request) string) *Handler {
	return &Handler{store: store, logger: logger, requireOwner: requireOwner, actor: actor}
}

func (h *Handler) Register(mux *http.ServeMux) {
	route := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, h.requireOwner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fn(w, r.WithContext(WithActor(r.Context(), h.actor(r))))
		})))
	}
	const tr = "/api/v1/admin/contents/{id}/translations/{locale}"
	route("GET /api/v1/admin/contents", h.listContents)
	route("POST /api/v1/admin/contents", h.createContent)
	route("GET /api/v1/admin/contents/{id}", h.getContent)
	route("POST /api/v1/admin/contents/{id}/archive", h.archive(true))
	route("POST /api/v1/admin/contents/{id}/unarchive", h.archive(false))
	route("POST /api/v1/admin/contents/{id}/translations", h.createTranslation)
	route("GET "+tr, h.getTranslation)
	route("GET "+tr+"/revisions", h.listRevisions)
	route("POST "+tr+"/revisions", h.saveRevision)
	route("GET "+tr+"/revisions/{version}", h.getRevision)
	route("POST "+tr+"/revisions/{version}/restore", h.restoreRevision)
	for _, kind := range []string{"categories", "tags"} {
		route("GET /api/v1/admin/"+kind, h.listTerms(kind))
		route("POST /api/v1/admin/"+kind, h.createTerm(kind))
		route("PATCH /api/v1/admin/"+kind+"/{termId}", h.updateTerm(kind))
	}
}

type page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

func (h *Handler) listContents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := ListFilter{Kind: q.Get("kind"), ProjectID: q.Get("project_id"), Archived: q.Get("archived") == "true"}
	errs := FieldErrors{}
	if f.Kind != "" && !contentKinds[f.Kind] {
		errs["kind"] = "must be project, article or log"
	}
	if f.ProjectID != "" && !idPattern.MatchString(f.ProjectID) {
		errs["project_id"] = "must be a UUID"
	}
	var ok bool
	if f.Page, f.PageSize, ok = paging(r, errs); !ok || len(errs) > 0 {
		h.invalid(w, r, errs)
		return
	}
	items, total, err := h.store.ListContents(r.Context(), f)
	if err != nil {
		h.fail(w, r, "list contents", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page[Content]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total})
}

func (h *Handler) createContent(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind      string `json:"kind"`
		ProjectID string `json:"project_id"`
		Locale    string `json:"locale"`
	}
	if !h.decode(w, r, &in) {
		return
	}
	errs := FieldErrors{}
	if !contentKinds[in.Kind] {
		errs["kind"] = "must be project, article or log"
	}
	if !locales[in.Locale] {
		errs["locale"] = "must be es or en"
	}
	switch {
	case in.Kind == "log" && !idPattern.MatchString(in.ProjectID):
		errs["project_id"] = "a log needs the id of its project"
	case in.Kind != "log" && in.ProjectID != "":
		errs["project_id"] = "only logs belong to a project"
	}
	if len(errs) > 0 {
		h.invalid(w, r, errs)
		return
	}
	c, err := h.store.CreateContent(r.Context(), in.Kind, in.ProjectID, in.Locale)
	if err != nil {
		h.storeError(w, r, "create content", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, c)
}

func (h *Handler) getContent(w http.ResponseWriter, r *http.Request) {
	id, ok := h.contentID(w, r)
	if !ok {
		return
	}
	c, err := h.store.GetContent(r.Context(), id)
	if err != nil {
		h.storeError(w, r, "get content", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) archive(archived bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.contentID(w, r)
		if !ok {
			return
		}
		c, err := h.store.SetArchived(r.Context(), id, archived)
		if err != nil {
			h.storeError(w, r, "archive content", err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, c)
	}
}

func (h *Handler) createTranslation(w http.ResponseWriter, r *http.Request) {
	id, ok := h.contentID(w, r)
	if !ok {
		return
	}
	var in struct {
		Locale         string `json:"locale"`
		CopyFromLocale string `json:"copy_from_locale"`
	}
	if !h.decode(w, r, &in) {
		return
	}
	errs := FieldErrors{}
	if !locales[in.Locale] {
		errs["locale"] = "must be es or en"
	}
	if in.CopyFromLocale != "" && (!locales[in.CopyFromLocale] || in.CopyFromLocale == in.Locale) {
		errs["copy_from_locale"] = "must be the other locale"
	}
	if len(errs) > 0 {
		h.invalid(w, r, errs)
		return
	}
	t, err := h.store.CreateTranslation(r.Context(), id, in.Locale, in.CopyFromLocale)
	if err != nil {
		h.storeError(w, r, "create translation", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, t)
}

func (h *Handler) getTranslation(w http.ResponseWriter, r *http.Request) {
	id, locale, ok := h.translationKey(w, r)
	if !ok {
		return
	}
	t, err := h.store.GetTranslation(r.Context(), id, locale)
	if err != nil {
		h.storeError(w, r, "get translation", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) listRevisions(w http.ResponseWriter, r *http.Request) {
	id, locale, ok := h.translationKey(w, r)
	if !ok {
		return
	}
	errs := FieldErrors{}
	p, size, ok := paging(r, errs)
	if !ok {
		h.invalid(w, r, errs)
		return
	}
	items, total, err := h.store.ListRevisions(r.Context(), id, locale, p, size)
	if err != nil {
		h.storeError(w, r, "list revisions", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page[RevisionSummary]{Items: items, Page: p, PageSize: size, Total: total})
}

type saveResponse struct {
	Created  bool      `json:"created"`
	Revision *Revision `json:"revision"`
}

func (h *Handler) saveRevision(w http.ResponseWriter, r *http.Request) {
	id, locale, ok := h.translationKey(w, r)
	if !ok {
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key != "" && !idempotencyPattern.MatchString(key) {
		h.invalid(w, r, FieldErrors{"Idempotency-Key": "8-128 characters from A-Z a-z 0-9 _ -"})
		return
	}
	var in struct {
		ExpectedVersion *int     `json:"expected_version"`
		Kind            string   `json:"kind"`
		Snapshot        Snapshot `json:"snapshot"`
	}
	if !h.decode(w, r, &in) {
		return
	}
	errs := FieldErrors{}
	if in.ExpectedVersion == nil || *in.ExpectedVersion < 0 {
		errs["expected_version"] = "is required"
	}
	if in.Kind != "manual" && in.Kind != "auto" {
		errs["kind"] = "must be manual or auto"
	}
	if len(errs) > 0 {
		h.invalid(w, r, errs)
		return
	}
	c, err := h.store.GetContent(r.Context(), id)
	if err != nil {
		h.storeError(w, r, "load content", err)
		return
	}
	// Media nodes are allowed; the store checks each asset exists, is ready and has the right kind.
	snap, plain, err := in.Snapshot.Normalize(c.Kind, DocumentOptions{AllowMedia: true})
	if err != nil {
		h.storeError(w, r, "validate snapshot", err)
		return
	}
	rev, created, err := h.store.SaveRevision(r.Context(), SaveInput{
		ContentID: id, Locale: locale, ExpectedVersion: *in.ExpectedVersion, Kind: in.Kind,
		Snapshot: snap, PlainText: plain, IdempotencyKey: key,
	})
	if err != nil {
		h.storeError(w, r, "save revision", err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpapi.WriteJSON(w, status, saveResponse{Created: created, Revision: rev})
}

func (h *Handler) getRevision(w http.ResponseWriter, r *http.Request) {
	id, locale, ok := h.translationKey(w, r)
	if !ok {
		return
	}
	v, ok := h.version(w, r)
	if !ok {
		return
	}
	rev, err := h.store.GetRevision(r.Context(), id, locale, v)
	if err != nil {
		h.storeError(w, r, "get revision", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, rev)
}

func (h *Handler) restoreRevision(w http.ResponseWriter, r *http.Request) {
	id, locale, ok := h.translationKey(w, r)
	if !ok {
		return
	}
	v, ok := h.version(w, r)
	if !ok {
		return
	}
	var in struct {
		ExpectedVersion *int `json:"expected_version"`
	}
	if !h.decode(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil || *in.ExpectedVersion < 0 {
		h.invalid(w, r, FieldErrors{"expected_version": "is required"})
		return
	}
	rev, created, err := h.store.RestoreRevision(r.Context(), id, locale, v, *in.ExpectedVersion)
	if err != nil {
		h.storeError(w, r, "restore revision", err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	httpapi.WriteJSON(w, status, saveResponse{Created: created, Revision: rev})
}

func (h *Handler) listTerms(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := h.store.ListTerms(r.Context(), kind)
		if err != nil {
			h.fail(w, r, "list terms", err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string][]Term{"items": items})
	}
}

type termInput struct {
	Slug   string `json:"slug"`
	Labels struct {
		ES string `json:"es"`
		EN string `json:"en"`
	} `json:"labels"`
}

func (t *termInput) validate() FieldErrors {
	errs := FieldErrors{}
	if !slugPattern.MatchString(t.Slug) || len(t.Slug) > 60 {
		errs["slug"] = "use 1-60 lowercase letters, digits and single hyphens"
	}
	checkText(errs, "labels.es", t.Labels.ES, 1, 60)
	checkText(errs, "labels.en", t.Labels.EN, 1, 60)
	return errs
}

func (h *Handler) createTerm(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in termInput
		if !h.decode(w, r, &in) {
			return
		}
		if errs := in.validate(); len(errs) > 0 {
			h.invalid(w, r, errs)
			return
		}
		t, err := h.store.CreateTerm(r.Context(), kind, in.Slug, in.Labels.ES, in.Labels.EN)
		if err != nil {
			h.storeError(w, r, "create term", err)
			return
		}
		httpapi.WriteJSON(w, http.StatusCreated, t)
	}
}

func (h *Handler) updateTerm(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("termId")
		if !idPattern.MatchString(id) {
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
		var in termInput
		if !h.decode(w, r, &in) {
			return
		}
		if errs := in.validate(); len(errs) > 0 {
			h.invalid(w, r, errs)
			return
		}
		t, err := h.store.UpdateTerm(r.Context(), kind, id, in.Slug, in.Labels.ES, in.Labels.EN)
		if err != nil {
			h.storeError(w, r, "update term", err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, t)
	}
}

// --- Helpers --------------------------------------------------------------------------------------

// decode reads at most 1 MiB of strict JSON; it answers 413 or 422 itself on failure.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body exceeds 1 MiB")
		return false
	}
	if err == nil {
		err = DecodeStrict(body, v)
	}
	if err != nil {
		h.invalid(w, r, FieldErrors{"body": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func (h *Handler) contentID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return "", false
	}
	return id, true
}

func (h *Handler) translationKey(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	id, ok := h.contentID(w, r)
	if !ok {
		return "", "", false
	}
	locale := r.PathValue("locale")
	if !locales[locale] {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return "", "", false
	}
	return id, locale, true
}

func (h *Handler) version(w http.ResponseWriter, r *http.Request) (int, bool) {
	v, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || v < 1 {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return 0, false
	}
	return v, true
}

func paging(r *http.Request, errs FieldErrors) (int, int, bool) {
	p, size := 1, 20
	q := r.URL.Query()
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			errs["page"] = "must be a positive integer"
			return 0, 0, false
		}
		p = n
	}
	if v := q.Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			errs["page_size"] = "must be between 1 and 100"
			return 0, 0, false
		}
		size = n
	}
	return p, size, true
}

func (h *Handler) invalid(w http.ResponseWriter, r *http.Request, errs FieldErrors) {
	httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{
		Code: "validation_failed", Message: "Request is invalid", Fields: errs, RequestID: httpapi.RequestID(r.Context()),
	})
}

// storeError maps domain errors to HTTP answers; anything unexpected is logged and hidden.
func (h *Handler) storeError(w http.ResponseWriter, r *http.Request, op string, err error) {
	var fe FieldErrors
	var ve *ValidationError
	var ce conflictError
	switch {
	case errors.As(err, &fe):
		h.invalid(w, r, fe)
	case errors.As(err, &ve) && ve.Code == "media_not_available":
		httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: ve.Code, Message: ve.Message,
			Fields: map[string]string{ve.Path: ve.Message}, RequestID: httpapi.RequestID(r.Context())})
	case errors.As(err, &ce):
		current := ce.current
		httpapi.WriteJSON(w, http.StatusConflict, httpapi.Error{Code: "version_conflict",
			Message: "The translation changed since expected_version", CurrentVersion: &current, RequestID: httpapi.RequestID(r.Context())})
	case errors.Is(err, errNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, errPublished):
		httpapi.WriteError(w, r, http.StatusConflict, "published_content", "Published content must be withdrawn first (WEB-005)")
	case errors.Is(err, errArchived):
		httpapi.WriteError(w, r, http.StatusConflict, "content_archived", "Archived content is read-only; unarchive it first")
	case errors.Is(err, errLocaleExists):
		httpapi.WriteError(w, r, http.StatusConflict, "locale_exists", "This translation already exists")
	case errors.Is(err, errTermSlugTaken):
		httpapi.WriteError(w, r, http.StatusConflict, "slug_taken", "Slug already used")
	case errors.Is(err, errNothingToCopy):
		h.invalid(w, r, FieldErrors{"copy_from_locale": "that translation has no saved revision"})
	case errors.Is(err, errIdempotencyMismatch):
		h.invalid(w, r, FieldErrors{"Idempotency-Key": "already used for a different snapshot"})
	default:
		h.fail(w, r, op, err)
	}
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.Error(op, "request_id", httpapi.RequestID(r.Context()), "error", err)
	httpapi.WriteError(w, r, http.StatusInternalServerError, "internal", "Internal server error")
}
