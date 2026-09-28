package media_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/authtest"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/media"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

// faultyStorage wraps the local backend to simulate low disk, disk full and lost objects.
type faultyStorage struct {
	*media.Local
	mu       sync.Mutex
	free     uint64
	putErr   error
	statSize int64
}

func (f *faultyStorage) FreeBytes() (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.free > 0 {
		return f.free, nil
	}
	return f.Local.FreeBytes()
}

func (f *faultyStorage) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	f.mu.Lock()
	err := f.putErr
	f.mu.Unlock()
	if err != nil {
		return err
	}
	return f.Local.Put(ctx, key, r, size)
}

func (f *faultyStorage) Stat(ctx context.Context, key string) (media.Info, error) {
	info, err := f.Local.Stat(ctx, key)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statSize != 0 {
		info.Size = f.statSize
	}
	return info, err
}

type env struct {
	*authtest.Server
	pool    *pgxpool.Pool
	root    string
	storage *faultyStorage
	handler *media.Handler
}

func start(t *testing.T, tweak func(*media.Limits)) *env {
	t.Helper()
	pool := testdb.New(t)
	root := t.TempDir()
	local, err := media.NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	st := &faultyStorage{Local: local}
	lim := media.DefaultLimits()
	lim.FreeReserve = 0
	if tweak != nil {
		tweak(&lim)
	}
	e := &env{pool: pool, root: root, storage: st}
	e.Server = authtest.Start(t, pool, func(a *auth.Handler, logger *slog.Logger) []httpapi.Module {
		e.handler = media.NewHandler(media.NewStore(pool), st, lim, logger, a.RequireOwner, auth.Actor, a.IsOwner)
		return []httpapi.Module{e.handler}
	})
	return e
}

func testdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type asset struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Status        string `json:"status"`
	OriginalName  string `json:"original_name"`
	MIME          string `json:"mime"`
	Bytes         int64  `json:"bytes"`
	SHA256        string `json:"sha256"`
	Width         *int   `json:"width"`
	Height        *int   `json:"height"`
	PublicEnabled bool   `json:"public_enabled"`
	Downloadable  bool   `json:"downloadable"`
	Texts         map[string]struct {
		Alt     string `json:"alt"`
		Caption string `json:"caption"`
	} `json:"texts"`
}

type apiError struct {
	Code string `json:"code"`
}

func (e *env) upload(name string, data []byte, headers ...string) authtest.Response {
	return e.Do("POST", "/api/v1/admin/assets?filename="+urlEscape(name), data, headers...)
}

func urlEscape(s string) string { return url.QueryEscape(s) }

func errCode(t *testing.T, r authtest.Response) string {
	t.Helper()
	var e apiError
	_ = json.Unmarshal(r.Body, &e)
	return e.Code
}

// storedObjects lists files under objects/ (the only place bytes may end up).
func (e *env) storedObjects(t *testing.T) []string {
	t.Helper()
	var files []string
	_ = filepath.Walk(filepath.Join(e.root, "objects"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func (e *env) count(t *testing.T, sql string) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestUploadAllowlistAndSanitizedBytes(t *testing.T) {
	e := start(t, nil)
	cases := []struct{ file, name, kind, mime string }{
		{"rotated.jpg", "Placa frontal.jpg", "image", "image/jpeg"},
		{"text.png", "diagrama.png", "image", "image/png"},
		{"exif.webp", "foto.webp", "image", "image/webp"},
		{"h264.mp4", "prueba.mp4", "video", "video/mp4"},
		{"doc.pdf", "ficha.pdf", "resource", "application/pdf"},
		{"ascii.stl", "soporte.stl", "resource", "model/stl"},
	}
	for _, c := range cases {
		// The client's Content-Type is irrelevant: lying about it changes nothing.
		r := e.upload(c.name, testdata(t, c.file), "Content-Type", "text/html")
		if r.Status != http.StatusCreated {
			t.Fatalf("%s: %d %s", c.file, r.Status, r.Body)
		}
		var a asset
		r.JSON(t, &a)
		if a.Kind != c.kind || a.MIME != c.mime || a.Status != "ready" || a.OriginalName != c.name || a.PublicEnabled || a.Downloadable {
			t.Errorf("%s: %+v", c.file, a)
		}
	}

	// The stored JPEG is the sanitized, rotated one; hash and size describe those bytes.
	var list struct {
		Items []asset `json:"items"`
		Total int     `json:"total"`
	}
	e.Do("GET", "/api/v1/admin/assets?kind=image", nil).JSON(t, &list)
	if list.Total != 3 {
		t.Fatalf("images listed: %d", list.Total)
	}
	var jpg asset
	for _, a := range list.Items {
		if a.MIME == "image/jpeg" {
			jpg = a
		}
	}
	if jpg.Width == nil || *jpg.Width != 32 || *jpg.Height != 64 {
		t.Fatalf("stored dimensions after rotation: %v×%v", jpg.Width, jpg.Height)
	}
	var stored []byte
	for _, f := range e.storedObjects(t) {
		b, _ := os.ReadFile(f)
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) == jpg.SHA256 {
			stored = b
		}
	}
	if stored == nil || int64(len(stored)) != jpg.Bytes {
		t.Fatal("no stored object matches the reported hash and size")
	}
	if bytes.Contains(stored, []byte("TestCam")) || bytes.Contains(stored, []byte("Exif")) {
		t.Fatal("stored JPEG still has EXIF")
	}
	if _, err := exec.LookPath("exiftool"); err == nil {
		f := filepath.Join(t.TempDir(), "x.jpg")
		_ = os.WriteFile(f, stored, 0o600)
		out, _ := exec.Command("exiftool", "-s", "-GPS:all", "-EXIF:all", f).CombinedOutput()
		if len(bytes.TrimSpace(out)) > 0 {
			t.Fatalf("exiftool finds metadata in the stored file:\n%s", out)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM assets WHERE status <> 'ready'`); n != 0 {
		t.Fatalf("non-ready assets after good uploads: %d", n)
	}
}

func TestUploadRejections(t *testing.T) {
	e := start(t, func(l *media.Limits) { l.Image = 2048; l.MaxSide = 50 })
	big := bytes.Repeat([]byte{0xFF}, 4096)
	var tall bytes.Buffer
	_ = png.Encode(&tall, image.NewGray(image.Rect(0, 0, 10, 60)))
	cases := []struct {
		name   string
		data   []byte
		status int
		code   string
	}{
		{"too-big.jpg", big, http.StatusRequestEntityTooLarge, "too_large"},
		{"disguised.jpg", testdata(t, "doc.pdf"), http.StatusUnsupportedMediaType, "unsupported_type"},
		{"page.png", []byte("<html><script>alert(1)</script>"), http.StatusUnsupportedMediaType, "unsupported_type"},
		{"logo.svg", []byte("<svg/>"), http.StatusUnsupportedMediaType, "unsupported_type"},
		{"foto.heic", []byte("x"), http.StatusUnsupportedMediaType, "unsupported_type"},
		{"tall.png", tall.Bytes(), http.StatusUnprocessableEntity, "invalid_media"},
		{"empty.pdf", []byte{}, http.StatusUnprocessableEntity, "invalid_media"},
		{"rotated.webp", testdata(t, "rotated.webp"), http.StatusUnprocessableEntity, "invalid_media"},
	}
	for _, c := range cases {
		r := e.upload(c.name, c.data)
		if r.Status != c.status || errCode(t, r) != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.name, r.Status, r.Body, c.status, c.code)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM assets WHERE status = 'ready'`); n != 0 {
		t.Fatalf("rejected uploads produced %d ready assets", n)
	}
	if n := e.count(t, `SELECT count(*) FROM assets WHERE status = 'pending'`); n != 0 {
		t.Fatalf("rejected uploads left %d pending rows", n)
	}
	if objs := e.storedObjects(t); len(objs) != 0 {
		t.Fatalf("rejected uploads left bytes: %v", objs)
	}
}

func TestContentLengthIsRequired(t *testing.T) {
	e := start(t, nil)
	pr, pw := io.Pipe()
	go func() { pw.Write(testdata(t, "doc.pdf")); pw.Close() }()
	req, _ := http.NewRequest("POST", e.Origin+"/api/v1/admin/assets?filename=x.pdf", pr) // unknown length → chunked
	req.Header.Set("Origin", e.Origin)
	req.Header.Set("X-CSRF-Token", e.CSRF)
	resp, err := e.Owner.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusLengthRequired {
		t.Fatalf("chunked upload = %d, want 411", resp.StatusCode)
	}
}

func TestMaliciousNamesNeverBecomePaths(t *testing.T) {
	e := start(t, nil)
	for _, name := range []string{"../../../../etc/passwd.pdf", "..\\..\\win.pdf", "a\x00b.pdf", "  .hidden.pdf", "<script>.pdf", strings.Repeat("ñ", 300) + ".pdf"} {
		r := e.upload(name, testdata(t, "doc.pdf"))
		if r.Status != http.StatusCreated {
			t.Fatalf("%q: %d %s", name, r.Status, r.Body)
		}
		var a asset
		r.JSON(t, &a)
		if strings.ContainsAny(a.OriginalName, "/\\\x00<>") || strings.HasPrefix(a.OriginalName, ".") || len(a.OriginalName) > 255 {
			t.Errorf("%q stored as %q", name, a.OriginalName)
		}
	}
	for _, f := range e.storedObjects(t) {
		rel, _ := filepath.Rel(filepath.Join(e.root, "objects"), f)
		if strings.Contains(rel, "..") || !regexpKey(filepath.ToSlash(rel)) {
			t.Errorf("object outside the opaque key scheme: %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(e.root, "etc")); err == nil {
		t.Fatal("a path from the file name was created")
	}
}

func regexpKey(s string) bool {
	parts := strings.Split(s, "/")
	return len(parts) == 3 && len(parts[0]) == 2 && len(parts[1]) == 2 && len(parts[2]) == 32
}

func TestDiskProblemsNeverLeaveReadyAssets(t *testing.T) {
	e := start(t, nil)
	// Not enough free space: refused before reading the body.
	e.storage.free = 10
	if r := e.upload("a.pdf", testdata(t, "doc.pdf")); r.Status != http.StatusInsufficientStorage {
		t.Fatalf("low disk: %d", r.Status)
	}
	e.storage.free = 0
	// Disk fills up while storing.
	e.storage.putErr = fmt.Errorf("write: %w", syscall.ENOSPC)
	if r := e.upload("b.pdf", testdata(t, "doc.pdf")); r.Status != http.StatusInsufficientStorage || errCode(t, r) != "insufficient_storage" {
		t.Fatalf("disk full: %d %s", r.Status, r.Body)
	}
	e.storage.putErr = nil
	// The object vanishes or is truncated after writing: never reported ready.
	e.storage.statSize = 1
	if r := e.upload("c.pdf", testdata(t, "doc.pdf")); r.Status != http.StatusInternalServerError {
		t.Fatalf("size mismatch: %d", r.Status)
	}
	e.storage.statSize = 0
	if n := e.count(t, `SELECT count(*) FROM assets WHERE status = 'ready'`); n != 0 {
		t.Fatalf("ready assets after disk problems: %d", n)
	}
	if n := e.count(t, `SELECT count(*) FROM assets WHERE status = 'failed'`); n != 2 {
		t.Fatalf("failed rows = %d, want 2 (the pre-check creates none)", n)
	}
	if objs := e.storedObjects(t); len(objs) != 0 {
		t.Fatalf("bytes left behind: %v", objs)
	}
	if r := e.upload("d.pdf", testdata(t, "doc.pdf")); r.Status != http.StatusCreated {
		t.Fatalf("recovered upload: %d", r.Status)
	}
}

func TestOneLargeUploadAtATime(t *testing.T) {
	e := start(t, func(l *media.Limits) { l.LargeThreshold = 100 })
	data := testdata(t, "h264.mp4")
	pr, pw := io.Pipe()
	firstDone := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest("POST", e.Origin+"/api/v1/admin/assets?filename=slow.mp4", pr)
		req.ContentLength = int64(len(data))
		req.Header.Set("Origin", e.Origin)
		req.Header.Set("X-CSRF-Token", e.CSRF)
		resp, err := e.Owner.Do(req)
		if err != nil {
			firstDone <- 0
			return
		}
		resp.Body.Close()
		firstDone <- resp.StatusCode
	}()
	pw.Write(data[:100]) // the first upload is now in progress, holding the slot
	time.Sleep(200 * time.Millisecond)
	r := e.upload("second.mp4", data)
	if r.Status != http.StatusTooManyRequests || r.Header.Get("Retry-After") == "" {
		t.Fatalf("second large upload: %d %s", r.Status, r.Body)
	}
	pw.Write(data[100:])
	pw.Close()
	if st := <-firstDone; st != http.StatusCreated {
		t.Fatalf("first upload finished with %d", st)
	}
	if r := e.upload("third.mp4", data); r.Status != http.StatusCreated {
		t.Fatalf("after the slot is free: %d", r.Status)
	}
}

func TestLibraryRequiresOwnerAndCSRF(t *testing.T) {
	e := start(t, nil)
	if r := e.Anonymous("POST", "/api/v1/admin/assets?filename=a.pdf", testdata(t, "doc.pdf")); r.Status != http.StatusUnauthorized {
		t.Errorf("anonymous upload: %d", r.Status)
	}
	if r := e.WithoutCSRF("POST", "/api/v1/admin/assets?filename=a.pdf", testdata(t, "doc.pdf")); r.Status != http.StatusForbidden {
		t.Errorf("upload without CSRF: %d", r.Status)
	}
	if r := e.upload("a.pdf", testdata(t, "doc.pdf"), "Origin", "https://evil.example"); r.Status != http.StatusForbidden {
		t.Errorf("cross-origin upload: %d", r.Status)
	}
	for _, p := range []string{"/api/v1/admin/assets", "/api/v1/admin/assets/00000000-0000-4000-8000-000000000000"} {
		if r := e.Anonymous("GET", p, nil); r.Status != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s: %d", p, r.Status)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM assets`); n != 0 {
		t.Fatalf("rejected requests created %d assets", n)
	}
}

func TestMetadataPermissionsAndDeletion(t *testing.T) {
	e := start(t, nil)
	var a asset
	e.upload("placa.jpg", testdata(t, "rotated.jpg")).JSON(t, &a)
	path := "/api/v1/admin/assets/" + a.ID
	r := e.Do("PATCH", path, map[string]any{"public_enabled": true, "downloadable": true,
		"texts": map[string]any{"es": map[string]string{"alt": "Placa de control", "caption": "Cara frontal"}, "en": map[string]string{"alt": "Control board"}}})
	var updated asset
	r.JSON(t, &updated)
	if r.Status != http.StatusOK || !updated.PublicEnabled || !updated.Downloadable || updated.Texts["es"].Alt != "Placa de control" || updated.Texts["en"].Alt != "Control board" {
		t.Fatalf("patch: %d %s", r.Status, r.Body)
	}
	if r := e.Do("PATCH", path, map[string]any{"texts": map[string]any{"fr": map[string]string{"alt": "x"}}}); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("unknown locale: %d", r.Status)
	}
	if r := e.Do("PATCH", path, map[string]any{"sha256": "x"}); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("bytes are not editable: %d", r.Status)
	}

	// Referenced by a revision (fixture): deletion is refused and the bytes stay.
	ctx := context.Background()
	var revID string
	if err := e.pool.QueryRow(ctx, `WITH c AS (INSERT INTO contents (kind) VALUES ('article') RETURNING id),
		t AS (INSERT INTO translations (content_id, locale, latest_version) SELECT id, 'es', 1 FROM c RETURNING id)
		INSERT INTO revisions (translation_id, version, kind, title, slug, body_json, body_schema_version, plain_text, snapshot_hash)
		SELECT id, 1, 'manual', 'x', 'x', '{}', 1, '', '\x00' FROM t RETURNING id`).Scan(&revID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `INSERT INTO revision_assets (revision_id, asset_id, usage) VALUES ($1, $2, 'image')`, revID, a.ID); err != nil {
		t.Fatal(err)
	}
	if r := e.Do("DELETE", path, nil); r.Status != http.StatusConflict || errCode(t, r) != "asset_in_use" {
		t.Fatalf("delete referenced: %d %s", r.Status, r.Body)
	}
	var detail struct {
		References []struct {
			Version int    `json:"version"`
			Usage   string `json:"usage"`
		} `json:"references"`
	}
	e.Do("GET", path, nil).JSON(t, &detail)
	if len(detail.References) != 1 || detail.References[0].Usage != "image" {
		t.Fatalf("references: %+v", detail.References)
	}
	if len(e.storedObjects(t)) != 1 {
		t.Fatal("bytes of a referenced asset disappeared")
	}

	// An unreferenced asset is deleted with its bytes.
	var b asset
	e.upload("otro.pdf", testdata(t, "doc.pdf")).JSON(t, &b)
	if r := e.Do("DELETE", "/api/v1/admin/assets/"+b.ID, nil); r.Status != http.StatusNoContent {
		t.Fatalf("delete unreferenced: %d", r.Status)
	}
	if len(e.storedObjects(t)) != 1 {
		t.Fatalf("objects after delete: %v", e.storedObjects(t))
	}
	if r := e.WithoutCSRF("DELETE", path, nil); r.Status != http.StatusForbidden {
		t.Errorf("delete without CSRF: %d", r.Status)
	}
}

func TestCleanupHandlesCrashesButNotActiveUploads(t *testing.T) {
	e := start(t, nil)
	ctx := context.Background()
	// A crash left a pending row whose bytes were already written, plus an old staging file.
	key := media.NewKey()
	if err := e.storage.Local.Put(ctx, key, bytes.NewReader([]byte("partial")), 7); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `INSERT INTO assets (kind, object_key, original_name, created_at) VALUES ('resource', $1, 'x.pdf', now() - interval '3 hours')`, key); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(e.storage.TempDir(), "upload-old")
	fresh := filepath.Join(e.storage.TempDir(), "upload-fresh")
	_ = os.WriteFile(old, []byte("x"), 0o600)
	_ = os.WriteFile(fresh, []byte("x"), 0o600)
	past := time.Now().Add(-3 * time.Hour)
	_ = os.Chtimes(old, past, past)

	if err := e.handler.Cleanup(ctx, 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("old staging file not removed")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("recent staging file removed")
	}
	if n := e.count(t, `SELECT count(*) FROM assets WHERE status = 'failed' AND failure = 'abandoned'`); n != 1 {
		t.Errorf("abandoned pending rows marked failed: %d", n)
	}
	if len(e.storedObjects(t)) != 0 {
		t.Error("bytes of the abandoned upload remain")
	}
}
