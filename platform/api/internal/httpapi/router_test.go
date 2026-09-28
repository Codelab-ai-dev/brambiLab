package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

type panicModule struct{}

func (panicModule) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/panic", func(http.ResponseWriter, *http.Request) { panic("boom") })
}

func do(t *testing.T, req *http.Request) (*httptest.ResponseRecorder, httpapi.Error) {
	t.Helper()
	router := httpapi.NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), "https://example.test", panicModule{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	var body httpapi.Error
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec, body
}

func TestUnknownRouteReturnsErrorShape(t *testing.T) {
	rec, body := do(t, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	if rec.Code != http.StatusNotFound || body.Code != "not_found" {
		t.Fatalf("got %d %+v", rec.Code, body)
	}
	if body.RequestID == "" || body.RequestID != rec.Header().Get("X-Request-ID") {
		t.Fatalf("request_id %q must match header %q", body.RequestID, rec.Header().Get("X-Request-ID"))
	}
}

func TestValidUpstreamRequestIDIsKept(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil)
	req.Header.Set("X-Request-ID", "proxy-123")
	_, body := do(t, req)
	if body.RequestID != "proxy-123" {
		t.Fatalf("request_id = %q, want proxy-123", body.RequestID)
	}
}

func TestUnsafeUpstreamRequestIDIsReplaced(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil)
	req.Header.Set("X-Request-ID", "bad id\nforged-log-line")
	_, body := do(t, req)
	if body.RequestID == "" || body.RequestID == "bad id\nforged-log-line" {
		t.Fatalf("unsafe request id was not replaced: %q", body.RequestID)
	}
}

func TestPanicBecomesInternalError(t *testing.T) {
	rec, body := do(t, httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil))
	if rec.Code != http.StatusInternalServerError || body.Code != "internal" {
		t.Fatalf("got %d %+v", rec.Code, body)
	}
}
