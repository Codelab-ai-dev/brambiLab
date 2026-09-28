package media

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

// registerDelivery mounts GET/HEAD /media/{id} and /media/{id}/download (web-v1.md §12.1).
// Go 1.22 patterns: "GET" also matches HEAD.
func (h *Handler) registerDelivery(mux *http.ServeMux) {
	mux.HandleFunc("GET /media/{assetId}", h.serve(false))
	mux.HandleFunc("GET /media/{assetId}/download", h.serve(true))
}

// serve checks authorization on every request:
//   - owner session: any ready asset;
//   - anonymous: ready + public_enabled + referenced by a published revision of visible content
//     (and downloadable for /download).
//
// Resources (PDF, ZIP, STL) are never served inline, so /download's permission cannot be bypassed.
// Anything not allowed gets the same 404, without metadata, also for HEAD and Range requests.
func (h *Handler) serve(download bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Cookie")
		notFound := func() {
			w.Header().Set("Cache-Control", "no-store")
			httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		}
		id := r.PathValue("assetId")
		if !idPattern.MatchString(id) {
			notFound()
			return
		}
		a, err := h.store.Get(r.Context(), id)
		if err != nil || a.Status != "ready" || (!download && a.Kind == KindResource) {
			if err != nil && !errors.Is(err, errNotFound) {
				h.logger.Error("load asset for delivery", "request_id", httpapi.RequestID(r.Context()), "error", err)
			}
			notFound()
			return
		}
		owner := h.isOwner(r)
		if !owner {
			visible, err := h.store.PubliclyVisible(r.Context(), id)
			if err != nil {
				h.logger.Error("check public visibility", "request_id", httpapi.RequestID(r.Context()), "error", err)
			}
			if err != nil || !visible || !a.PublicEnabled || (download && !a.Downloadable) {
				notFound()
				return
			}
		}
		obj, err := h.storage.Open(r.Context(), a.objectKey)
		if err != nil {
			h.logger.Error("open asset object", "asset_id", id, "error", err)
			notFound()
			return
		}
		defer obj.Close()

		hdr := w.Header()
		hdr.Set("Content-Type", a.MIME) // set explicitly: ServeContent must not sniff
		hdr.Set("Content-Security-Policy", "default-src 'none'; sandbox")
		hdr.Set("Cross-Origin-Resource-Policy", "same-origin")
		hdr.Set("ETag", `"`+a.SHA256+`"`)
		disposition := "inline"
		if download {
			disposition = "attachment"
		}
		hdr.Set("Content-Disposition", contentDisposition(disposition, a.OriginalName))
		if owner {
			hdr.Set("Cache-Control", "private, no-store")
		} else {
			// Revalidate every time: withdrawing or revoking stops delivery immediately (no CDN).
			hdr.Set("Cache-Control", "no-cache")
		}
		// ServeContent reads from the file: HEAD, Range/206, Content-Range, Accept-Ranges,
		// 416 for unsatisfiable or malformed ranges, multipart/byteranges, If-None-Match.
		http.ServeContent(w, r, "", obj.Info().ModTime, obj)
	}
}

// contentDisposition builds an RFC 6266 header with an ASCII fallback and a UTF-8 filename*.
func contentDisposition(kind, name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' || r == ';' {
			return '_'
		}
		return r
	}, name)
	return kind + `; filename="` + ascii + `"; filename*=UTF-8''` + url.PathEscape(name)
}

// PubliclyVisible: at least one reference from a revision that is published and visible, using
// the shared rule of the visible_translations view (web-v1.md §8.1).
func (s Store) PubliclyVisible(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM revision_assets ra
			JOIN visible_translations v ON v.revision_id = ra.revision_id
			WHERE ra.asset_id = $1)`, id).Scan(&ok)
	return ok, err
}
