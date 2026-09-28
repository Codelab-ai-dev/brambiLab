package migrations_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/migrations"
)

// Integration test: requires a disposable PostgreSQL in TEST_DATABASE_URL.
func TestUpIsIdempotentAndSafeConcurrently(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = migrations.Up(ctx, url, logger)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent run %d: %v", i, err)
		}
	}
	if err := migrations.Up(ctx, url, logger); err != nil {
		t.Fatalf("second run: %v", err)
	}

	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var applied int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM goose_db_version WHERE version_id = 1 AND is_applied`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("migration 1 recorded %d times, want 1", applied)
	}
	if _, err := db.ExecContext(ctx, `SELECT id, actor, action FROM audit_events LIMIT 0`); err != nil {
		t.Fatalf("audit_events missing: %v", err)
	}
}
