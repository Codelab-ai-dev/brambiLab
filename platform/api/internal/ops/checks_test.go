package ops_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/ops"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

var t0 = time.Date(2026, 9, 28, 16, 0, 0, 0, time.UTC)

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func checker(pool *pgxpool.Pool, free uint64) *ops.Checker {
	c := ops.NewChecker(pool, "/data/media", true, true)
	c.Now = func() time.Time { return t0 }
	c.Free = func(string) (uint64, uint64, error) { return free, 100 << 30, nil }
	return c
}

func status(r ops.Report, name string) (ops.Status, string) {
	for _, c := range r.Checks {
		if c.Name == name {
			return c.Status, c.Detail
		}
	}
	return "", ""
}

func TestChecksThresholds(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	// Nothing recorded yet: backups warn, everything else ok.
	r := checker(pool, 50<<30).Run(ctx)
	if s, _ := status(r, "backup"); s != ops.StatusWarn || r.Status != ops.StatusWarn {
		t.Fatalf("initial %+v", r)
	}
	// A recent success is ok; an old one or a failed last run is critical.
	exec(t, pool, `INSERT INTO ops_backup_runs (status, finished_at) VALUES ('succeeded', $1)`, t0.Add(-2*time.Hour))
	if s, _ := status(checker(pool, 50<<30).Run(ctx), "backup"); s != ops.StatusOK {
		t.Fatalf("recent backup: %s", s)
	}
	exec(t, pool, `UPDATE ops_backup_runs SET finished_at = $1`, t0.Add(-27*time.Hour))
	if s, d := status(checker(pool, 50<<30).Run(ctx), "backup"); s != ops.StatusCritical || !strings.Contains(d, "27.0 h") {
		t.Fatalf("stale backup: %s %s", s, d)
	}
	exec(t, pool, `INSERT INTO ops_backup_runs (status, finished_at, step) VALUES ('failed', $1, 'upload')`, t0)
	if s, d := status(checker(pool, 50<<30).Run(ctx), "backup"); s != ops.StatusCritical || !strings.Contains(d, "upload") {
		t.Fatalf("failed backup: %s %s", s, d)
	}

	// Disk.
	for free, want := range map[uint64]ops.Status{50 << 30: ops.StatusOK, 1 << 30: ops.StatusWarn, 100 << 20: ops.StatusCritical} {
		if s, _ := status(checker(pool, free).Run(ctx), "disk"); s != want {
			t.Errorf("free %d: %s, want %s", free, s, want)
		}
	}
	c := checker(pool, 0)
	c.Free = func(string) (uint64, uint64, error) { return 0, 0, errors.New("no such dir") }
	if s, _ := status(c.Run(ctx), "disk"); s != ops.StatusCritical {
		t.Error("unreadable disk must be critical")
	}

	// A due publication job older than the lag means the scheduler is stuck; not while paused.
	exec(t, pool, `WITH c AS (INSERT INTO contents (kind) VALUES ('article') RETURNING id),
		tr AS (INSERT INTO translations (content_id, locale, latest_version) SELECT id, 'es', 1 FROM c RETURNING id),
		r AS (INSERT INTO revisions (translation_id, version, kind, title, slug, body_json, body_schema_version, plain_text, snapshot_hash)
		      SELECT id, 1, 'manual', 'x', 'x', '{"type":"doc","content":[]}', 1, '', '\x01' FROM tr RETURNING id, translation_id)
		INSERT INTO publication_jobs (translation_id, revision_id, run_at, next_attempt_at, created_by) SELECT translation_id, id, $1, $1, 'test' FROM r`, t0.Add(-10*time.Minute))
	if s, _ := status(checker(pool, 50<<30).Run(ctx), "publication_jobs"); s != ops.StatusCritical {
		t.Fatalf("stuck scheduler: %s", s)
	}
	exec(t, pool, `UPDATE app_maintenance SET active = true, activated_at = $1`, t0.Add(-time.Minute))
	r = checker(pool, 50<<30).Run(ctx)
	if s, _ := status(r, "publication_jobs"); s != ops.StatusWarn {
		t.Fatalf("paused scheduler should only warn: %s", s)
	}
	if s, _ := status(r, "maintenance"); s != ops.StatusWarn {
		t.Fatalf("short maintenance: %s", s)
	}
	exec(t, pool, `UPDATE app_maintenance SET activated_at = $1`, t0.Add(-45*time.Minute))
	if s, _ := status(checker(pool, 50<<30).Run(ctx), "maintenance"); s != ops.StatusCritical {
		t.Fatalf("maintenance left on: %s", s)
	}

	off := checker(pool, 50<<30)
	off.Background = false
	if s, _ := status(off.Run(ctx), "background_jobs"); s != ops.StatusCritical {
		t.Fatal("BACKGROUND_JOBS=off must be critical")
	}
	noContact := checker(pool, 50<<30)
	noContact.ContactEnabled = false
	if s, _ := status(noContact.Run(ctx), "contact_jobs"); s != "" {
		t.Fatal("contact checks run while contact is disabled")
	}
}

func TestMonitorNotifiesOnlyOnChangesAndRecovers(t *testing.T) {
	pool := testdb.New(t)
	var mu sync.Mutex
	var got []map[string]any
	fail := false
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		got = append(got, m)
	}))
	t.Cleanup(hook.Close)
	url, err := ops.WebhookURL(hook.URL)
	if err != nil {
		t.Fatal(err)
	}
	free := uint64(50 << 30)
	c := checker(pool, 0)
	c.Free = func(string) (uint64, uint64, error) { return free, 100 << 30, nil }
	exec(t, pool, `INSERT INTO ops_backup_runs (status, finished_at) VALUES ('succeeded', $1)`, t0.Add(-time.Hour))
	m := ops.NewMonitor(c, url, "https://brambilab.dev", quiet)
	ctx := context.Background()

	if _, sent := m.Tick(ctx); sent {
		t.Fatal("notified while everything is ok")
	}
	free = 100 << 20 // disk critical
	if _, sent := m.Tick(ctx); !sent {
		t.Fatal("no notification for a new problem")
	}
	if _, sent := m.Tick(ctx); sent {
		t.Fatal("repeated notification for the same problem")
	}
	free = 50 << 30
	fail = true
	if _, sent := m.Tick(ctx); sent {
		t.Fatal("reported sent although the webhook failed")
	}
	fail = false
	if _, sent := m.Tick(ctx); !sent {
		t.Fatal("recovery not retried after a failed delivery")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || got[0]["event"] != "problem" || got[1]["event"] != "recovered" || !strings.Contains(got[0]["text"].(string), "disk") {
		t.Fatalf("payloads %+v", got)
	}

	for _, bad := range []string{"http://hooks.example.com/x", "ftp://x", "not a url"} {
		if _, err := ops.WebhookURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if u, err := ops.WebhookURL(""); err != nil || u != "" {
		t.Error("empty must disable notifications")
	}
}
