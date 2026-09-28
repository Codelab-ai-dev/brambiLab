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

func NewRouter(logger *slog.Logger, modules ...Module) http.Handler {
	mux := http.NewServeMux()
	for _, m := range modules {
		m.Register(mux)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
	})
	return withRequestContext(logger, mux)
}
