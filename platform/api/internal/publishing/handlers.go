package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

const maxBodyBytes = 16 << 10

var (
	idPattern          = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
	slugPattern        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

func validLocale(l string) bool { return l == "es" || l == "en" }

// Handler serves the owner-only publication API and the anonymous public reader.
type Handler struct {
	svc          *Service
	logger       *slog.Logger
	requireOwner func(http.Handler) http.Handler
	actor        func(*http.Request) string
}

func NewHandler(svc *Service, logger *slog.Logger, requireOwner func(http.Handler) http.Handler, actor func(*http.Request) string) *Handler {
	return &Handler{svc: svc, logger: logger, requireOwner: requireOwner, actor: actor}
}

func (h *Handler) Register(mux *http.ServeMux) {
	route := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, h.requireOwner(fn)) }
	const p = "/api/v1/admin/contents/{id}/translations/{locale}/publication"
	route("GET "+p, h.getState)
	route("POST "+p+"/publish", h.publish)
	route("POST "+p+"/schedule", h.schedule)
	route("POST "+p+"/cancel", h.simple(h.svc.Cancel))
	route("POST "+p+"/withdraw", h.simple(h.svc.Withdraw))
	route("GET "+p+"/jobs", h.listJobs)
	route("POST "+p+"/jobs/{jobId}/retry", h.retry)

	mux.HandleFunc("GET /api/v1/public/{locale}/projects/{slug}", h.publicProject)
	mux.HandleFunc("GET /api/v1/public/{locale}/articles/{slug}", h.publicArticle)
	mux.HandleFunc("GET /api/v1/public/{locale}/projects/{projectSlug}/logs/{slug}", h.publicLog)
}

// target validates the path; false means a 404 was written.
func (h *Handler) target(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	id, locale := r.PathValue("id"), r.PathValue("locale")
	if !idPattern.MatchString(id) || !validLocale(locale) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return "", "", false
	}
	return id, locale, true
}

func (h *Handler) getState(w http.ResponseWriter, r *http.Request) {
	id, locale, ok := h.target(w, r)
	if !ok {
		return
	}
	st, err := h.svc.State(r.Context(), id, locale)
	if err != nil {
		h.serviceError(w, r, "get editorial state", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, st)
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	id, locale, ok := h.target(w, r)
	if !ok {
		return
	}
	page, size := 1, 20
	fields := map[string]string{}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 {
			fields["page"] = "must be a positive integer"
		} else {
			page = n
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 100 {
			fields["page_size"] = "must be between 1 and 100"
		} else {
			size = n
		}
	}
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	jobs, total, err := h.svc.Jobs(r.Context(), id, locale, page, size)
	if err != nil {
		h.serviceError(w, r, "list publication jobs", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": jobs, "page": page, "page_size": size, "total": total})
}

// request parses the shared parts of a mutation: path, Idempotency-Key and a strict JSON body.
func (h *Handler) request(w http.ResponseWriter, r *http.Request, body any) (Request, bool) {
	id, locale, ok := h.target(w, r)
	if !ok {
		return Request{}, false
	}
	key := r.Header.Get("Idempotency-Key")
	if key != "" && !idempotencyPattern.MatchString(key) {
		invalid(w, r, map[string]string{"Idempotency-Key": "use 8-128 letters, digits, '-' or '_'"})
		return Request{}, false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body is too large")
			return Request{}, false
		}
		invalid(w, r, map[string]string{"body": "must be a JSON object with the documented fields"})
		return Request{}, false
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		invalid(w, r, map[string]string{"body": "must contain a single JSON object"})
		return Request{}, false
	}
	return Request{ContentID: id, Locale: locale, Actor: h.actor(r), IdempotencyKey: key}, true
}

type expected struct {
	ExpectedEditorialVersion *int `json:"expected_editorial_version"`
}

func (e expected) check(fields map[string]string) int {
	if e.ExpectedEditorialVersion == nil || *e.ExpectedEditorialVersion < 0 {
		fields["expected_editorial_version"] = "is required (integer ≥ 0)"
		return 0
	}
	return *e.ExpectedEditorialVersion
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		expected
		RevisionVersion *int `json:"revision_version"`
	}
	req, ok := h.request(w, r, &body)
	if !ok {
		return
	}
	fields := map[string]string{}
	req.ExpectedEditorialVersion = body.check(fields)
	if body.RevisionVersion == nil || *body.RevisionVersion < 1 {
		fields["revision_version"] = "is required (integer ≥ 1)"
	}
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	res, err := h.svc.Publish(r.Context(), req, *body.RevisionVersion)
	h.respond(w, r, "publish", res, err)
}

func (h *Handler) schedule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		expected
		RevisionVersion *int   `json:"revision_version"`
		RunAtLocal      string `json:"run_at_local"`
		TimeZone        string `json:"time_zone"`
		Replace         bool   `json:"replace"`
	}
	req, ok := h.request(w, r, &body)
	if !ok {
		return
	}
	fields := map[string]string{}
	req.ExpectedEditorialVersion = body.check(fields)
	if body.RevisionVersion == nil || *body.RevisionVersion < 1 {
		fields["revision_version"] = "is required (integer ≥ 1)"
	}
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	res, err := h.svc.Schedule(r.Context(), req, *body.RevisionVersion, body.RunAtLocal, body.TimeZone, body.Replace)
	h.respond(w, r, "schedule", res, err)
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if !idPattern.MatchString(jobID) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	h.simple(func(ctx context.Context, req Request) (Result, error) { return h.svc.Retry(ctx, req, jobID) })(w, r)
}

// simple handles the actions whose body is only expected_editorial_version.
func (h *Handler) simple(op func(context.Context, Request) (Result, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body expected
		req, ok := h.request(w, r, &body)
		if !ok {
			return
		}
		fields := map[string]string{}
		req.ExpectedEditorialVersion = body.check(fields)
		if len(fields) > 0 {
			invalid(w, r, fields)
			return
		}
		res, err := op(r.Context(), req)
		h.respond(w, r, "editorial action", res, err)
	}
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, op string, res Result, err error) {
	if err != nil {
		h.serviceError(w, r, op, err)
		return
	}
	if res.Replay != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(res.Replay.status)
		_, _ = w.Write(res.Replay.body)
		return
	}
	httpapi.WriteJSON(w, res.Status, res.State)
}

// editorialError is httpapi.Error plus the current state on editorial_conflict.
type editorialError struct {
	httpapi.Error
	State *State `json:"state,omitempty"`
}

func (h *Handler) serviceError(w http.ResponseWriter, r *http.Request, op string, err error) {
	rid := httpapi.RequestID(r.Context())
	var (
		conflict conflictError
		blocked  blockedError
		deps     dependencyError
		taken    slugTakenError
		bad      invalidError
	)
	switch {
	case errors.Is(err, errNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
	case errors.As(err, &conflict):
		v := conflict.state.EditorialVersion
		httpapi.WriteJSON(w, http.StatusConflict, editorialError{Error: httpapi.Error{Code: "editorial_conflict",
			Message: "The publication state changed since expected_editorial_version", CurrentVersion: &v, RequestID: rid}, State: conflict.state})
	case errors.As(err, &blocked):
		fields := map[string]string{}
		for _, p := range blocked.problems {
			if prev, ok := fields[p.Field]; ok {
				fields[p.Field] = prev + "; " + p.Message
			} else {
				fields[p.Field] = p.Message
			}
		}
		httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "publish_blocked",
			Message: "This revision cannot be published yet", Fields: fields, RequestID: rid})
	case errors.As(err, &deps):
		fields := map[string]string{}
		for _, d := range deps.logs {
			fields["logs."+d.ContentID] = d.Title
		}
		httpapi.WriteJSON(w, http.StatusConflict, httpapi.Error{Code: "has_published_logs",
			Message: "Withdraw the published logs of this project in this locale first", Fields: fields, RequestID: rid})
	case errors.As(err, &taken):
		httpapi.WriteJSON(w, http.StatusConflict, httpapi.Error{Code: "slug_taken",
			Message: "Another content already uses this public route", Fields: map[string]string{"slug": taken.slug}, RequestID: rid})
	case errors.As(err, &bad):
		invalid(w, r, map[string]string{bad.field: bad.message})
	case errors.Is(err, errIdempotencyReuse):
		httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "idempotency_key_reused",
			Message: "This Idempotency-Key was used with a different request", RequestID: rid})
	case errors.Is(err, errScheduleExists):
		httpapi.WriteError(w, r, http.StatusConflict, "schedule_exists", "A publication is already scheduled; replace it explicitly")
	case errors.Is(err, errNoActiveJob):
		httpapi.WriteError(w, r, http.StatusConflict, "no_active_schedule", "There is no scheduled publication to cancel")
	case errors.Is(err, errJobNotFailed):
		httpapi.WriteError(w, r, http.StatusConflict, "job_not_failed", "Only failed jobs can be retried")
	case errors.Is(err, errNothingToWithdraw):
		httpapi.WriteError(w, r, http.StatusConflict, "not_published", "This translation is not published or scheduled")
	default:
		h.logger.Error(op, "request_id", rid, "error", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal", "Internal server error")
	}
}

func invalid(w http.ResponseWriter, r *http.Request, fields map[string]string) {
	httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{
		Code: "validation_failed", Message: "Request is invalid", Fields: fields, RequestID: httpapi.RequestID(r.Context())})
}
