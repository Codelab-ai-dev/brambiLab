package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/health"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func serve(t *testing.T, db health.Pinger, path string) *httptest.ResponseRecorder {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpapi.NewRouter(logger, "https://example.test", nil, health.Handler{DB: db, Logger: logger})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestLiveDoesNotTouchDatabase(t *testing.T) {
	rec := serve(t, fakeDB{err: errors.New("down")}, "/api/v1/health/live")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyReportsDatabaseUp(t *testing.T) {
	rec := serve(t, fakeDB{}, "/api/v1/health/ready")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Checks["database"] != "up" {
		t.Fatalf("database check = %q, want up", body.Checks["database"])
	}
}

func TestReadyHidesFailureDetails(t *testing.T) {
	secret := "password authentication failed for user brambilab"
	rec := serve(t, fakeDB{err: errors.New(secret)}, "/api/v1/health/ready")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("response leaks error details: %s", rec.Body.String())
	}
}
