package ops_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/ops"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func setActive(t *testing.T, pool *pgxpool.Pool, active bool) {
	t.Helper()
	sql := `UPDATE app_maintenance SET active = false, reason = ''`
	if active {
		sql = `UPDATE app_maintenance SET active = true, reason = 'test', activated_at = now(), acknowledged_at = NULL`
	}
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Fatal(err)
	}
}

func acknowledged(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var ok bool
	if err := pool.QueryRow(context.Background(), `SELECT acknowledged_at IS NOT NULL AND acknowledged_at >= activated_at FROM app_maintenance`).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

type module struct{ hold chan struct{} }

func (m module) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/thing", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /api/v1/thing", func(w http.ResponseWriter, _ *http.Request) {
		if m.hold != nil {
			<-m.hold
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestMaintenanceRejectsWritesAndAcknowledgesWhenQuiet(t *testing.T) {
	pool := testdb.New(t)
	m := ops.NewMaintenance(pool, quiet, func(string) string { return "" })
	hold := make(chan struct{})
	srv := httptest.NewServer(httpapi.NewRouter(quiet, "http://site.test", m.Middleware, module{hold: hold}))
	t.Cleanup(srv.Close)
	post := func() *http.Response {
		req, _ := http.NewRequest("POST", srv.URL+"/api/v1/thing", strings.NewReader("{}"))
		req.Header.Set("Origin", "http://site.test")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}

	// A write in flight before maintenance starts: acknowledgement waits for it.
	done := make(chan *http.Response, 1)
	go func() { done <- post() }()
	time.Sleep(100 * time.Millisecond)
	setActive(t, pool, true)
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !m.Active() || acknowledged(t, pool) {
		t.Fatal("acknowledged with a write in flight")
	}
	close(hold)
	if r := <-done; r.StatusCode != http.StatusNoContent {
		t.Fatalf("in-flight write: %d", r.StatusCode)
	}
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !acknowledged(t, pool) {
		t.Fatal("not acknowledged once quiet")
	}

	// During maintenance: writes 503 with Retry-After, reads keep working.
	if r := post(); r.StatusCode != http.StatusServiceUnavailable || r.Header.Get("Retry-After") != "60" {
		t.Fatalf("write during maintenance: %d %v", r.StatusCode, r.Header)
	}
	resp, err := http.Get(srv.URL + "/api/v1/thing")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read during maintenance: %d", resp.StatusCode)
	}

	setActive(t, pool, false)
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := post(); r.StatusCode != http.StatusNoContent {
		t.Fatalf("write after maintenance: %d", r.StatusCode)
	}
}

func TestGatePausesPassesAndIsCountedInFlight(t *testing.T) {
	pool := testdb.New(t)
	m := ops.NewMaintenance(pool, quiet, func(string) string { return "" })
	gate := ops.Gate(m.Allow)
	done, ok := gate.Try()
	if !ok {
		t.Fatal("gate closed without maintenance")
	}
	setActive(t, pool, true)
	_ = m.Refresh(context.Background())
	if acknowledged(t, pool) {
		t.Fatal("acknowledged while a pass runs")
	}
	if _, ok := gate.Try(); ok {
		t.Fatal("new pass started during maintenance")
	}
	done()
	_ = m.Refresh(context.Background())
	if !acknowledged(t, pool) {
		t.Fatal("not acknowledged after the pass ended")
	}

	off := ops.NewMaintenance(pool, quiet, func(k string) string {
		if k == "BACKGROUND_JOBS" {
			return "off"
		}
		return ""
	})
	setActive(t, pool, false)
	_ = off.Refresh(context.Background())
	if _, ok := ops.Gate(off.Allow).Try(); ok || !off.BackgroundDisabled() {
		t.Fatal("BACKGROUND_JOBS=off still allows passes")
	}
	if _, ok := ops.Gate(nil).Try(); !ok {
		t.Fatal("nil gate must allow")
	}
}
