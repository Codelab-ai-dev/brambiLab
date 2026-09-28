package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"
)

type ctxKey struct{}

const requestIDHeader = "X-Request-ID"

// Accept upstream IDs (e.g. from the proxy) only when they are short and log-safe.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// withRequestContext assigns a request ID, recovers panics and writes one structured log line
// per request. It never logs bodies, query strings or headers that may carry secrets.
func withRequestContext(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, id))
		w.Header().Set(requestIDHeader, id)
		w.Header().Set("X-Content-Type-Options", "nosniff")

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		defer func() {
			if p := recover(); p != nil {
				logger.Error("panic", "request_id", id, "panic", p)
				WriteError(rec, r, http.StatusInternalServerError, "internal", "Internal server error")
			}
			logger.Info("request",
				"request_id", id,
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}()
		next.ServeHTTP(rec, r)
	})
}
