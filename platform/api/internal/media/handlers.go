package media

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/content"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/ops"
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// FileStorage is the local backend plus what uploads need from it (staging dir, free space).
type FileStorage interface {
	Storage
	TempDir() string
	FreeBytes() (uint64, error)
}

// Handler serves the media library API. Delivery under /media lives in serve.go.
type Handler struct {
	store        Store
	storage      FileStorage
	limits       Limits
	logger       *slog.Logger
	requireOwner func(http.Handler) http.Handler
	actor        func(*http.Request) string
	isOwner      func(*http.Request) bool
	// Gate pauses the staging cleanup (maintenance, BACKGROUND_JOBS=off).
	Gate ops.Gate

	large  chan struct{} // one large upload at a time
	decode chan struct{} // one image decode/re-encode at a time (bounded memory)
	mu     sync.Mutex
	active map[string]bool // staging files in use, never swept
}

func NewHandler(store Store, storage FileStorage, limits Limits, logger *slog.Logger,
	requireOwner func(http.Handler) http.Handler, actor func(*http.Request) string, isOwner func(*http.Request) bool) *Handler {
	return &Handler{store: store, storage: storage, limits: limits, logger: logger,
		requireOwner: requireOwner, actor: actor, isOwner: isOwner,
		large: make(chan struct{}, 1), decode: make(chan struct{}, 1), active: map[string]bool{}}
}

func (h *Handler) Register(mux *http.ServeMux) {
	owner := func(pattern string, fn http.HandlerFunc) { mux.Handle(pattern, h.requireOwner(fn)) }
	owner("GET /api/v1/admin/assets", h.list)
	owner("POST /api/v1/admin/assets", h.upload)
	owner("GET /api/v1/admin/assets/{assetId}", h.detail)
	owner("PATCH /api/v1/admin/assets/{assetId}", h.update)
	owner("DELETE /api/v1/admin/assets/{assetId}", h.delete)
	h.registerDelivery(mux)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, rej *Rejection) {
	if rej.Status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "30")
	}
	httpapi.WriteError(w, r, rej.Status, rej.Code, rej.Message)
}

func (h *Handler) internal(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.Error(op, "request_id", httpapi.RequestID(r.Context()), "error", err)
	httpapi.WriteError(w, r, http.StatusInternalServerError, "internal", "Internal server error")
}

// SanitizeName keeps a display name only: base name, no control or path characters, ≤ 255 bytes.
func SanitizeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == utf8.RuneError, unicode.IsControl(r), strings.ContainsRune(`/\:*?"<>|`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimLeft(strings.TrimSpace(b.String()), ".")
	for len(out) > 255 {
		_, size := utf8.DecodeLastRuneInString(out)
		out = out[:len(out)-size]
	}
	if out == "" || out == "_" {
		return "archivo"
	}
	return out
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	// --- Checks that need no body ------------------------------------------------------------
	if r.ContentLength < 0 {
		h.fail(w, r, reject(http.StatusLengthRequired, "length_required", "La subida necesita Content-Length."))
		return
	}
	if r.ContentLength == 0 {
		h.fail(w, r, reject(http.StatusUnprocessableEntity, "invalid_media", "El archivo está vacío."))
		return
	}
	name := SanitizeName(r.URL.Query().Get("filename"))
	kind, rej := KindForName(r.URL.Query().Get("filename"))
	if rej != nil {
		h.fail(w, r, rej)
		return
	}
	if limit := h.limits.For(kind); r.ContentLength > limit {
		h.fail(w, r, reject(http.StatusRequestEntityTooLarge, "too_large", "El archivo supera el límite de %d MiB para este tipo.", limit>>20))
		return
	}
	if free, err := h.storage.FreeBytes(); err != nil {
		h.internal(w, r, "check free space", err)
		return
	} else if uint64(r.ContentLength)+h.limits.FreeReserve > free {
		h.fail(w, r, reject(http.StatusInsufficientStorage, "insufficient_storage", "No hay espacio suficiente en el servidor para este archivo."))
		return
	}
	if r.ContentLength > h.limits.LargeThreshold {
		select {
		case h.large <- struct{}{}:
			defer func() { <-h.large }()
		default:
			h.fail(w, r, reject(http.StatusTooManyRequests, "upload_busy", "Ya hay una subida grande en curso; inténtalo cuando termine."))
			return
		}
	}

	// --- Stream, validate, store ---------------------------------------------------------------
	key := NewKey()
	id, err := h.store.CreatePending(r.Context(), kind, h.storage.Name(), key, name)
	if err != nil {
		h.internal(w, r, "create pending asset", err)
		return
	}
	asset, rej, err := h.receive(r, id, key, kind)
	if rej != nil || err != nil {
		reason := "internal"
		if rej != nil {
			reason = rej.Code
		}
		// Use a fresh context: the request may be gone, the row must not stay pending.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = h.storage.Delete(ctx, key)
		if mErr := h.store.MarkFailed(ctx, id, reason); mErr != nil {
			h.logger.Error("mark asset failed", "asset_id", id, "error", mErr)
		}
		if rej != nil {
			h.fail(w, r, rej)
		} else {
			h.internal(w, r, "store upload", err)
		}
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, asset)
}

// receive writes the body to a staging file, validates and sanitizes it, and moves it into
// storage. It returns a Rejection for client problems (bad type, too large, disk full…).
func (h *Handler) receive(r *http.Request, id, key string, kind Kind) (Asset, *Rejection, error) {
	staged, err := os.CreateTemp(h.storage.TempDir(), "upload-*")
	if err != nil {
		if isNoSpace(err) {
			return Asset{}, noSpace(), nil
		}
		return Asset{}, nil, err
	}
	h.track(staged.Name(), true)
	defer h.track(staged.Name(), false)
	defer os.Remove(staged.Name())
	defer staged.Close()

	body := http.MaxBytesReader(nil, r.Body, r.ContentLength)
	n, err := io.Copy(staged, body)
	switch {
	case isNoSpace(err):
		return Asset{}, noSpace(), nil
	case err != nil:
		return Asset{}, reject(http.StatusBadRequest, "upload_incomplete", "La subida se interrumpió; inténtalo de nuevo."), nil
	case n != r.ContentLength:
		return Asset{}, reject(http.StatusBadRequest, "upload_incomplete", "Se recibieron %d de %d bytes.", n, r.ContentLength), nil
	}
	if err := staged.Sync(); err != nil {
		if isNoSpace(err) {
			return Asset{}, noSpace(), nil
		}
		return Asset{}, nil, err
	}

	detected, err := Detect(staged, n, kind, h.limits)
	if err != nil {
		var rej *Rejection
		if errors.As(err, &rej) {
			return Asset{}, rej, nil
		}
		return Asset{}, nil, err
	}

	final := staged
	if detected.Kind == KindImage {
		final, err = h.sanitizeStaged(staged, n, detected)
		if err != nil {
			var rej *Rejection
			if errors.As(err, &rej) {
				return Asset{}, rej, nil
			}
			if isNoSpace(err) {
				return Asset{}, noSpace(), nil
			}
			return Asset{}, nil, err
		}
		defer os.Remove(final.Name())
		defer final.Close()
		// Width/height of the stored (possibly rotated) pixels.
		if st, err := final.Stat(); err == nil {
			if d2, err := Detect(final, st.Size(), KindImage, h.limits); err == nil {
				detected.Width, detected.Height = d2.Width, d2.Height
			}
		}
	}

	// Hash and size of the bytes actually stored.
	if _, err := final.Seek(0, io.SeekStart); err != nil {
		return Asset{}, nil, err
	}
	sum := sha256.New()
	size, err := io.Copy(sum, final)
	if err != nil {
		return Asset{}, nil, err
	}
	if _, err := final.Seek(0, io.SeekStart); err != nil {
		return Asset{}, nil, err
	}
	if err := h.storage.Put(r.Context(), key, final, size); err != nil {
		if isNoSpace(err) {
			return Asset{}, noSpace(), nil
		}
		return Asset{}, nil, err
	}
	// Never report ready unless the object is really there with the expected size.
	if info, err := h.storage.Stat(r.Context(), key); err != nil || info.Size != size {
		return Asset{}, nil, fmt.Errorf("stored object missing or size mismatch: %v", err)
	}
	if err := h.store.MarkReady(r.Context(), id, detected, size, sum.Sum(nil)); err != nil {
		return Asset{}, nil, err
	}
	a, err := h.store.Get(r.Context(), id)
	return a, nil, err
}

func noSpace() *Rejection {
	return reject(http.StatusInsufficientStorage, "insufficient_storage", "El servidor se quedó sin espacio durante la subida.")
}

// sanitizeStaged strips image metadata into a second staging file. One image at a time is
// decoded/re-encoded so memory stays bounded.
func (h *Handler) sanitizeStaged(staged *os.File, size int64, d Detected) (*os.File, error) {
	h.decode <- struct{}{}
	defer func() { <-h.decode }()
	data := make([]byte, size)
	if _, err := staged.ReadAt(data, 0); err != nil {
		return nil, err
	}
	clean, err := SanitizeImage(data, d.format, h.limits)
	if err != nil {
		return nil, err
	}
	out, err := os.CreateTemp(h.storage.TempDir(), "clean-*")
	if err != nil {
		return nil, err
	}
	h.track(out.Name(), true)
	defer h.track(out.Name(), false)
	if _, err := out.Write(clean); err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, err
	}
	return out, nil
}

func (h *Handler) track(path string, on bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if on {
		h.active[path] = true
	} else {
		delete(h.active, path)
	}
}

// Cleanup removes staging files idle for longer than maxAge (never active uploads) and fails
// pending rows older than maxAge, deleting any bytes they left behind.
func (h *Handler) Cleanup(ctx context.Context, maxAge time.Duration) error {
	entries, err := os.ReadDir(h.storage.TempDir())
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(h.storage.TempDir(), e.Name())
		h.mu.Lock()
		busy := h.active[path]
		h.mu.Unlock()
		info, err := e.Info()
		if busy || err != nil || time.Since(info.ModTime()) < maxAge {
			continue
		}
		_ = os.Remove(path)
	}
	stale, err := h.store.StalePending(ctx, maxAge)
	if err != nil {
		return err
	}
	for _, a := range stale {
		_ = h.storage.Delete(ctx, a.objectKey)
		if err := h.store.MarkFailed(ctx, a.ID, "abandoned"); err != nil {
			return err
		}
	}
	return nil
}

// RunCleanup calls Cleanup periodically until ctx ends.
func (h *Handler) RunCleanup(ctx context.Context, every, maxAge time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if done, ok := h.Gate.Try(); ok {
			if err := h.Cleanup(ctx, maxAge); err != nil && !errors.Is(err, context.Canceled) {
				h.logger.Warn("media cleanup failed", "error", err)
			}
			done()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// --- Library ------------------------------------------------------------------------------------

type page struct {
	Items    []Asset `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	Total    int     `json:"total"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := Kind(q.Get("kind"))
	if kind != "" && kind != KindImage && kind != KindVideo && kind != KindResource {
		httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "validation_failed", Message: "Request is invalid",
			Fields: map[string]string{"kind": "must be image, video or resource"}, RequestID: httpapi.RequestID(r.Context())})
		return
	}
	p, size := 1, 24
	if v, err := strconv.Atoi(q.Get("page")); err == nil && v > 0 {
		p = v
	}
	if v, err := strconv.Atoi(q.Get("page_size")); err == nil && v > 0 && v <= 100 {
		size = v
	}
	items, total, err := h.store.List(r.Context(), kind, p, size)
	if err != nil {
		h.internal(w, r, "list assets", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page{Items: items, Page: p, PageSize: size, Total: total})
}

func (h *Handler) assetID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("assetId")
	if !idPattern.MatchString(id) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return "", false
	}
	return id, true
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	id, ok := h.assetID(w, r)
	if !ok {
		return
	}
	d, err := h.store.Detail(r.Context(), id)
	if errors.Is(err, errNotFound) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	if err != nil {
		h.internal(w, r, "asset detail", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, d)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.assetID(w, r)
	if !ok {
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	var u Update
	if err == nil {
		err = content.DecodeStrict(raw, &u)
	}
	fields := map[string]string{}
	if err != nil {
		fields["body"] = "invalid JSON: " + err.Error()
	}
	for locale, t := range u.Texts {
		if locale != "es" && locale != "en" {
			fields["texts"] = "locales must be es or en"
		}
		if utf8.RuneCountInString(t.Alt) > 300 || utf8.RuneCountInString(t.Caption) > 500 {
			fields["texts."+locale] = "alt ≤ 300 and caption ≤ 500 characters"
		}
	}
	if len(fields) > 0 {
		httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "validation_failed", Message: "Request is invalid", Fields: fields, RequestID: httpapi.RequestID(r.Context())})
		return
	}
	a, err := h.store.Apply(r.Context(), id, h.actor(r), u)
	if errors.Is(err, errNotFound) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	if err != nil {
		h.internal(w, r, "update asset", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, a)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.assetID(w, r)
	if !ok {
		return
	}
	key, err := h.store.Delete(r.Context(), id, h.actor(r))
	switch {
	case errors.Is(err, errNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, errInUse):
		httpapi.WriteError(w, r, http.StatusConflict, "asset_in_use", "Una o más revisiones usan este archivo; no se puede borrar.")
	case err != nil:
		h.internal(w, r, "delete asset", err)
	default:
		if err := h.storage.Delete(r.Context(), key); err != nil {
			h.logger.Warn("asset row deleted but bytes remain", "asset_id", id, "error", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
