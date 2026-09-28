package migrations_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/migrations"
)

// Integration test: requires a disposable PostgreSQL in TEST_DATABASE_URL.
func TestUpIsIdempotentAndSafeConcurrently(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	conn, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = migrations.Up(ctx, conn, logger)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent run %d: %v", i, err)
		}
	}
	if err := migrations.Up(ctx, conn, logger); err != nil {
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

// Upgrading a WEB-002 database (sessions in use) to the content schema keeps existing sessions.
func TestUpgradeFromAuthSchemaKeepsSessions(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	name := "bl_upgrade_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
	})
	conn, _ := pgx.ParseConfig(url)
	conn.Database = name
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := migrations.UpTo(ctx, conn, 2); err != nil {
		t.Fatalf("migrate to WEB-002 schema: %v", err)
	}
	db, err := pgx.ConnectConfig(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if _, err := db.Exec(ctx, `INSERT INTO sessions (token_hash, csrf_token, github_user_id, github_login, expires_at)
		VALUES ('\x01', 'csrf', 201345228, 'owner', now() + interval '1 hour')`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(ctx, conn, logger); err != nil {
		t.Fatalf("upgrade to content schema: %v", err)
	}
	var sessions, contentTables int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE revoked_at IS NULL`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_name IN ('contents','translations','revisions','revision_tags','categories','tags')`).Scan(&contentTables); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 || contentTables != 6 {
		t.Fatalf("after upgrade: sessions=%d content tables=%d", sessions, contentTables)
	}
}

// Upgrading a WEB-003 database (content, revisions, sessions) to the media schema keeps everything.
func TestUpgradeFromContentSchemaKeepsData(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	name := "bl_upgrade_media_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
	})
	conn, _ := pgx.ParseConfig(url)
	conn.Database = name
	if err := migrations.UpTo(ctx, conn, 3); err != nil {
		t.Fatalf("migrate to WEB-003 schema: %v", err)
	}
	db, err := pgx.ConnectConfig(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if _, err := db.Exec(ctx, `
		INSERT INTO sessions (token_hash, csrf_token, github_user_id, github_login, expires_at) VALUES ('\x02', 'c', 1, 'o', now() + interval '1 hour');
		WITH c AS (INSERT INTO contents (kind) VALUES ('project') RETURNING id),
		     t AS (INSERT INTO translations (content_id, locale, latest_version) SELECT id, 'es', 1 FROM c RETURNING id)
		INSERT INTO revisions (translation_id, version, kind, title, slug, body_json, body_schema_version, plain_text, snapshot_hash)
		SELECT id, 1, 'manual', 'Rover', 'rover', '{"type":"doc","content":[]}', 1, '', '\x01' FROM t`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(ctx, conn, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("upgrade to media schema: %v", err)
	}
	var sessions, revisions int
	var title string
	var cover *string
	if err := db.QueryRow(ctx, `SELECT (SELECT count(*) FROM sessions), (SELECT count(*) FROM revisions), (SELECT title FROM revisions), (SELECT cover_asset_id::text FROM revisions)`).
		Scan(&sessions, &revisions, &title, &cover); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 || revisions != 1 || title != "Rover" || cover != nil {
		t.Fatalf("after upgrade: sessions=%d revisions=%d title=%q cover=%v", sessions, revisions, title, cover)
	}
}

// Upgrading a WEB-004 database with a pointer set before WEB-005 gives it a date and a route.
func TestUpgradeFromMediaSchemaBackfillsPublications(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	name := "bl_upgrade_publishing_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
	})
	conn, _ := pgx.ParseConfig(url)
	conn.Database = name
	if err := migrations.UpTo(ctx, conn, 4); err != nil {
		t.Fatalf("migrate to WEB-004 schema: %v", err)
	}
	db, err := pgx.ConnectConfig(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if _, err := db.Exec(ctx, `
		WITH c AS (INSERT INTO contents (kind) VALUES ('article') RETURNING id),
		     t AS (INSERT INTO translations (content_id, locale, latest_version) SELECT id, 'es', 1 FROM c RETURNING id),
		     r AS (INSERT INTO revisions (translation_id, version, kind, title, slug, body_json, body_schema_version, plain_text, snapshot_hash)
		           SELECT id, 1, 'manual', 'Nota', 'nota', '{"type":"doc","content":[]}', 1, '', '\x01' FROM t RETURNING id)
		SELECT 1 FROM r;
		UPDATE translations t SET published_revision_id = r.id FROM revisions r WHERE r.translation_id = t.id`); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(ctx, conn, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("upgrade to publishing schema: %v", err)
	}
	var dated, routes, visible, editorial int
	if err := db.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM translations WHERE published_at IS NOT NULL AND first_published_at IS NOT NULL),
		(SELECT count(*) FROM public_routes WHERE locale = 'es' AND scope = 'article' AND slug = 'nota' AND is_current),
		(SELECT count(*) FROM visible_translations),
		(SELECT max(editorial_version) FROM translations)`).Scan(&dated, &routes, &visible, &editorial); err != nil {
		t.Fatal(err)
	}
	if dated != 1 || routes != 1 || visible != 1 || editorial != 0 {
		t.Fatalf("after upgrade: dated=%d routes=%d visible=%d editorial=%d", dated, routes, visible, editorial)
	}
}
