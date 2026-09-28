// Package health serves liveness and readiness probes for Docker and Coolify.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	DB      Pinger
	Logger  *slog.Logger
	Timeout time.Duration
}

type status struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

func (h Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/health/live", h.live)
	mux.HandleFunc("GET /api/v1/health/ready", h.ready)
}

// live reports that the process serves HTTP; it never touches dependencies.
func (h Handler) live(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	httpapi.WriteJSON(w, http.StatusOK, status{Status: "ok"})
}

// ready checks dependencies. Failure details go to the log, never to the response.
func (h Handler) ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	timeout := h.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	if err := h.DB.Ping(ctx); err != nil {
		h.Logger.Warn("readiness: database unavailable",
			"request_id", httpapi.RequestID(r.Context()), "error", err)
		httpapi.WriteJSON(w, http.StatusServiceUnavailable,
			status{Status: "unavailable", Checks: map[string]string{"database": "down"}})
		return
	}
	httpapi.WriteJSON(w, http.StatusOK,
		status{Status: "ok", Checks: map[string]string{"database": "up"}})
}
