// Package httpapi wires the HTTP surface of the Go API under /api/v1 (web-v1.md §10).
package httpapi

import (
	"log/slog"
	"net/http"
)

// Module registers its routes on the shared mux. Each internal package (auth, content, ...)
// will expose one as it is implemented.
type Module interface {
	Register(mux *http.ServeMux)
}

// NewRouter mounts modules and rejects unsafe requests not coming from publicOrigin.
// wrap (optional) runs inside the origin check, e.g. the maintenance gate for writes.
func NewRouter(logger *slog.Logger, publicOrigin string, wrap func(http.Handler) http.Handler, modules ...Module) http.Handler {
	mux := http.NewServeMux()
	for _, m := range modules {
		m.Register(mux)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
	})
	var h http.Handler = mux
	if wrap != nil {
		h = wrap(mux)
	}
	return withRequestContext(logger, sameOrigin(publicOrigin, h))
}
