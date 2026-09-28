package db_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/db"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/health"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/migrations"
)

// Integration test: TEST_DATABASE_URL must point to a disposable PostgreSQL with a superuser.
// It creates a role whose password contains URI-reserved characters, then migrates and checks
// readiness through the PG* variables used in Compose, asserting the password never reaches logs.
func TestReservedPasswordMigratesAndIsReady(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)

	name := fmt.Sprintf("bl_reserved_%d", time.Now().UnixNano())
	quotedPW := strings.ReplaceAll(reservedPassword, "'", "''")
	for _, stmt := range []string{
		fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s'`, name, quotedPW),
		fmt.Sprintf(`CREATE DATABASE %s OWNER %s`, name, name),
	} {
		if _, err := admin.Exec(ctx, stmt); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, name))
		_, _ = admin.Exec(ctx, fmt.Sprintf(`DROP ROLE IF EXISTS %s`, name))
	})

	ac := admin.Config()
	setPGEnv(t, ac.Host, strconv.Itoa(int(ac.Port)), name, reservedPassword, name)

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	cfg, err := db.Config("")
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(ctx, cfg.ConnConfig, logger); err != nil {
		t.Fatalf("migrate with reserved password: %v", err)
	}
	if got := ready(t, ctx, cfg, logger); got != http.StatusOK {
		t.Fatalf("readiness = %d, want 200", got)
	}

	// A wrong password must fail readiness without logging either password.
	const wrong = "wr@ng/#pw"
	t.Setenv("PGPASSWORD", wrong)
	bad, err := db.Config("")
	if err != nil {
		t.Fatal(err)
	}
	if got := ready(t, ctx, bad, logger); got != http.StatusServiceUnavailable {
		t.Fatalf("readiness with wrong password = %d, want 503", got)
	}

	for _, secret := range []string{reservedPassword, "p@ss", wrong} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("logs contain a password (%q):\n%s", secret, logs.String())
		}
	}
}

func ready(t *testing.T, ctx context.Context, cfg *pgxpool.Config, logger *slog.Logger) int {
	t.Helper()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	router := httpapi.NewRouter(logger, "http://localhost:8000", health.Handler{DB: pool, Logger: logger, Timeout: 5 * time.Second})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health/ready", nil))
	return rec.Code
}
