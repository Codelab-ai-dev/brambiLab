package media_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/authtest"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/content"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/media"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

// startFull mounts auth, content and media, like the real server.
func startFull(t *testing.T) *env {
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
	e := &env{pool: pool, root: root, storage: st}
	e.Server = authtest.Start(t, pool, func(a *auth.Handler, logger *slog.Logger) []httpapi.Module {
		e.handler = media.NewHandler(media.NewStore(pool), st, lim, logger, a.RequireOwner, auth.Actor, a.IsOwner)
		return []httpapi.Module{e.handler, content.NewHandler(content.NewStore(pool), logger, a.RequireOwner, auth.Actor)}
	})
	return e
}

func (e *env) uploadID(t *testing.T, file, name string) string {
	t.Helper()
	r := e.upload(name, testdata(t, file))
	if r.Status != http.StatusCreated {
		t.Fatalf("upload %s: %d %s", file, r.Status, r.Body)
	}
	var a asset
	r.JSON(t, &a)
	return a.ID
}

func (e *env) setAsset(t *testing.T, id string, public, downloadable bool) {
	t.Helper()
	if r := e.Do("PATCH", "/api/v1/admin/assets/"+id, map[string]any{"public_enabled": public, "downloadable": downloadable}); r.Status != http.StatusOK {
		t.Fatalf("patch asset: %d %s", r.Status, r.Body)
	}
}

type contentRef struct{ id string }

// newContent creates a content (log needs projectID) and saves one revision using body and cover.
func (e *env) newContent(t *testing.T, kind, projectID string, body []any, cover string) contentRef {
	t.Helper()
	req := map[string]any{"kind": kind, "locale": "es"}
	if projectID != "" {
		req["project_id"] = projectID
	}
	r := e.Do("POST", "/api/v1/admin/contents", req)
	if r.Status != http.StatusCreated {
		t.Fatalf("create content: %d %s", r.Status, r.Body)
	}
	var c struct {
		ID string `json:"id"`
	}
	r.JSON(t, &c)
	snap := map[string]any{"title": "Con medios", "slug": "con-medios", "body": map[string]any{"type": "doc", "content": body}}
	if cover != "" {
		snap["cover_asset_id"] = cover
	}
	if r := e.save(t, c.ID, 0, snap); r.Status != http.StatusCreated {
		t.Fatalf("save revision: %d %s", r.Status, r.Body)
	}
	return contentRef{c.ID}
}

func (e *env) save(t *testing.T, contentID string, expected int, snap map[string]any) authtest.Response {
	t.Helper()
	return e.Do("POST", "/api/v1/admin/contents/"+contentID+"/translations/es/revisions",
		map[string]any{"expected_version": expected, "kind": "manual", "snapshot": snap})
}

func (e *env) publish(t *testing.T, c contentRef, locale string, on bool) {
	t.Helper()
	sql := `UPDATE translations t SET published_revision_id = NULL, published_at = NULL WHERE t.content_id = $1 AND t.locale = $2`
	if on {
		sql = `UPDATE translations t SET published_revision_id = r.id, published_at = now() FROM revisions r
			WHERE r.translation_id = t.id AND r.version = t.latest_version AND t.content_id = $1 AND t.locale = $2`
	}
	if _, err := e.pool.Exec(context.Background(), sql, c.id, locale); err != nil {
		t.Fatal(err)
	}
}

func imageNode(id, alt string) map[string]any {
	return map[string]any{"type": "image", "attrs": map[string]any{"assetId": id, "alt": alt}}
}

func get(t *testing.T, c *http.Client, url string, headers ...string) (*http.Response, []byte) {
	t.Helper()
	return do(t, c, http.MethodGet, url, headers...)
}

func do(t *testing.T, c *http.Client, method, url string, headers ...string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

// assertHidden checks that anonymous access reveals nothing, for GET, HEAD, Range and download.
func (e *env) assertHidden(t *testing.T, id, why string) {
	t.Helper()
	for _, path := range []string{"/media/" + id, "/media/" + id + "/download"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			resp, body := do(t, http.DefaultClient, method, e.Origin+path, "Range", "bytes=0-10")
			if resp.StatusCode != http.StatusNotFound || resp.Header.Get("ETag") != "" ||
				strings.HasPrefix(resp.Header.Get("Content-Type"), "image/") || resp.Header.Get("Content-Range") != "" ||
				!strings.Contains(resp.Header.Get("Cache-Control"), "no-store") {
				t.Fatalf("%s: anonymous %s %s = %d %v", why, method, path, resp.StatusCode, resp.Header)
			}
			if method == http.MethodGet && (bytes.Contains(body, []byte("placa")) || bytes.Contains(body, []byte("jpeg"))) {
				t.Fatalf("%s: 404 body leaks metadata: %s", why, body)
			}
		}
	}
}

func TestPublicRuleForImages(t *testing.T) {
	e := startFull(t)
	img := e.uploadID(t, "rotated.jpg", "placa.jpg")
	a := e.newContent(t, "article", "", []any{imageNode(img, "Placa")}, "")

	// Owner previews private files; responses are private and never cached.
	resp, body := get(t, e.Owner, e.Origin+"/media/"+img)
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/jpeg" || resp.Header.Get("Cache-Control") != "private, no-store" || len(body) == 0 {
		t.Fatalf("owner preview: %d %v", resp.StatusCode, resp.Header)
	}

	e.assertHidden(t, img, "draft only")
	e.setAsset(t, img, true, false)
	e.assertHidden(t, img, "public_enabled but only in a draft")
	e.publish(t, a, "es", true)
	e.setAsset(t, img, false, false)
	e.assertHidden(t, img, "published but public_enabled off")

	e.setAsset(t, img, true, false)
	resp, body = get(t, http.DefaultClient, e.Origin+"/media/"+img)
	if resp.StatusCode != 200 || resp.Header.Get("Cache-Control") != "no-cache" || resp.Header.Get("ETag") == "" {
		t.Fatalf("public image: %d %v", resp.StatusCode, resp.Header)
	}
	for k, v := range map[string]string{"X-Content-Type-Options": "nosniff", "Content-Security-Policy": "default-src 'none'; sandbox", "Content-Type": "image/jpeg"} {
		if resp.Header.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, resp.Header.Get(k), v)
		}
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "inline;") {
		t.Errorf("disposition %q", resp.Header.Get("Content-Disposition"))
	}
	// Revalidation: unchanged file → 304 without body.
	resp, body = get(t, http.DefaultClient, e.Origin+"/media/"+img, "If-None-Match", resp.Header.Get("ETag"))
	if resp.StatusCode != http.StatusNotModified || len(body) != 0 {
		t.Fatalf("revalidation: %d", resp.StatusCode)
	}
	// Inline public does not imply downloadable.
	if resp, _ := get(t, http.DefaultClient, e.Origin+"/media/"+img+"/download"); resp.StatusCode != 404 {
		t.Fatalf("download without downloadable: %d", resp.StatusCode)
	}

	// Shared asset: withdrawing one content keeps it public through the other.
	b := e.newContent(t, "article", "", []any{imageNode(img, "Otra")}, "")
	e.publish(t, b, "es", true)
	e.publish(t, a, "es", false)
	if resp, _ := get(t, http.DefaultClient, e.Origin+"/media/"+img); resp.StatusCode != 200 {
		t.Fatalf("still referenced by a published content: %d", resp.StatusCode)
	}
	e.publish(t, b, "es", false)
	e.assertHidden(t, img, "withdrawn everywhere")

	// Archived content does not count even with a published pointer.
	e.publish(t, b, "es", true)
	if _, err := e.pool.Exec(context.Background(), `UPDATE contents SET archived_at = now() WHERE id = $1`, b.id); err != nil {
		t.Fatal(err)
	}
	e.assertHidden(t, img, "archived content")
}

func TestLogMediaNeedsItsProjectPublished(t *testing.T) {
	e := startFull(t)
	img := e.uploadID(t, "rotated.jpg", "dia1.jpg")
	e.setAsset(t, img, true, false)
	project := e.newContent(t, "project", "", []any{}, "")
	log := e.newContent(t, "log", project.id, []any{imageNode(img, "Día 1")}, "")
	e.publish(t, log, "es", true)
	e.assertHidden(t, img, "log published, project not")
	// Project published only in English: still hidden for the Spanish log.
	if r := e.Do("POST", "/api/v1/admin/contents/"+project.id+"/translations", map[string]any{"locale": "en"}); r.Status != 201 {
		t.Fatal(r.Status)
	}
	if r := e.Do("POST", "/api/v1/admin/contents/"+project.id+"/translations/en/revisions", map[string]any{"expected_version": 0, "kind": "manual",
		"snapshot": map[string]any{"title": "P", "slug": "p", "body": map[string]any{"type": "doc", "content": []any{}}}}); r.Status != 201 {
		t.Fatal(r.Status)
	}
	e.publish(t, project, "en", true)
	e.assertHidden(t, img, "project published in another locale")
	e.publish(t, project, "es", true)
	if resp, _ := get(t, http.DefaultClient, e.Origin+"/media/"+img); resp.StatusCode != 200 {
		t.Fatalf("log and project published: %d", resp.StatusCode)
	}
}

func TestResourcesOnlyAsAttachments(t *testing.T) {
	e := startFull(t)
	pdf := e.uploadID(t, "doc.pdf", "ficha técnica «v1».pdf")
	c := e.newContent(t, "article", "", []any{map[string]any{"type": "download", "attrs": map[string]any{"assetId": pdf, "label": "Ficha"}}}, "")
	e.publish(t, c, "es", true)
	e.setAsset(t, pdf, true, false)
	// Never inline, not even for the owner.
	if resp, _ := get(t, e.Owner, e.Origin+"/media/"+pdf); resp.StatusCode != 404 {
		t.Fatalf("owner inline PDF: %d", resp.StatusCode)
	}
	e.assertHidden(t, pdf, "not downloadable")
	e.setAsset(t, pdf, true, true)
	resp, body := get(t, http.DefaultClient, e.Origin+"/media/"+pdf+"/download")
	cd := resp.Header.Get("Content-Disposition")
	if resp.StatusCode != 200 || !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, "filename*=UTF-8''ficha%20t%C3%A9cnica") ||
		resp.Header.Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("public download: %d %q %q", resp.StatusCode, cd, resp.Header.Get("Content-Type"))
	}
	if strings.ContainsAny(strings.SplitN(cd, "filename*", 2)[0], "«»") {
		t.Fatalf("ASCII fallback contains non-ASCII: %q", cd)
	}
	if resp, _ := get(t, http.DefaultClient, e.Origin+"/media/"+pdf); resp.StatusCode != 404 {
		t.Fatalf("anonymous inline PDF: %d", resp.StatusCode)
	}
}

func TestVideoRanges(t *testing.T) {
	e := startFull(t)
	vid := e.uploadID(t, "h264.mp4", "prueba.mp4")
	c := e.newContent(t, "article", "", []any{map[string]any{"type": "video", "attrs": map[string]any{"assetId": vid}}}, "")
	e.publish(t, c, "es", true)
	e.setAsset(t, vid, true, false)
	full := testdata(t, "h264.mp4")
	size := len(full)
	url := e.Origin + "/media/" + vid

	resp, body := get(t, http.DefaultClient, url, "Range", "bytes=0-99")
	if resp.StatusCode != 206 || resp.Header.Get("Content-Range") != fmt.Sprintf("bytes 0-99/%d", size) || len(body) != 100 || !bytes.Equal(body, full[:100]) {
		t.Fatalf("first range: %d %q %d", resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	}
	resp, body = get(t, http.DefaultClient, url, "Range", fmt.Sprintf("bytes=%d-", size-10))
	if resp.StatusCode != 206 || !bytes.Equal(body, full[size-10:]) {
		t.Fatalf("tail range: %d", resp.StatusCode)
	}
	if resp, _ := get(t, http.DefaultClient, url, "Range", fmt.Sprintf("bytes=%d-", size+10)); resp.StatusCode != 416 || resp.Header.Get("Content-Range") != fmt.Sprintf("bytes */%d", size) {
		t.Fatalf("unsatisfiable: %d %q", resp.StatusCode, resp.Header.Get("Content-Range"))
	}
	if resp, _ := get(t, http.DefaultClient, url, "Range", "bytes=abc"); resp.StatusCode != 416 {
		t.Fatalf("malformed range: %d", resp.StatusCode)
	}
	resp, body = get(t, http.DefaultClient, url, "Range", "bytes=0-9,20-29")
	if resp.StatusCode != 206 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "multipart/byteranges") || !bytes.Contains(body, full[20:30]) {
		t.Fatalf("multiple ranges: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp, body = do(t, http.DefaultClient, http.MethodHead, url)
	if resp.StatusCode != 200 || resp.Header.Get("Accept-Ranges") != "bytes" || resp.ContentLength != int64(size) || len(body) != 0 {
		t.Fatalf("HEAD: %d %v", resp.StatusCode, resp.Header)
	}
}

func TestDeliveryStreamsWithBoundedMemory(t *testing.T) {
	e := startFull(t)
	ctx := context.Background()
	const size = 64 << 20
	key := media.NewKey()
	if err := e.storage.Local.Put(ctx, key, io.LimitReader(zeroReader{}, size), size); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := e.pool.QueryRow(ctx, `INSERT INTO assets (kind, object_key, original_name, mime, bytes, sha256, status)
		VALUES ('video', $1, 'grande.mp4', 'video/mp4', $2, '\x00', 'ready') RETURNING id`, key, size).Scan(&id); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	req, _ := http.NewRequest("GET", e.Origin+"/media/"+id, nil)
	resp, err := e.Owner.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	runtime.ReadMemStats(&after)
	if n != size {
		t.Fatalf("read %d bytes", n)
	}
	// Client and server share this process; streaming 64 MiB must allocate far less than 64 MiB.
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 16<<20 {
		t.Fatalf("serving 64 MiB allocated %d MiB: not streaming", alloc>>20)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestRevisionReferencesAreRecordedAndValidated(t *testing.T) {
	e := startFull(t)
	img := e.uploadID(t, "rotated.jpg", "img.jpg")
	poster := e.uploadID(t, "text.png", "poster.png")
	vid := e.uploadID(t, "vp9.mp4", "v.mp4")
	pdf := e.uploadID(t, "doc.pdf", "d.pdf")
	body := []any{
		imageNode(img, "Placa"),
		map[string]any{"type": "video", "attrs": map[string]any{"assetId": vid, "posterAssetId": poster, "caption": "Prueba"}},
		map[string]any{"type": "download", "attrs": map[string]any{"assetId": pdf, "label": "Ficha"}},
	}
	c := e.newContent(t, "article", "", body, img)
	usages := func() string {
		var s string
		_ = e.pool.QueryRow(context.Background(), `SELECT string_agg(ra.usage, ',' ORDER BY ra.usage) FROM revision_assets ra
			JOIN revisions r ON r.id = ra.revision_id JOIN translations t ON t.id = r.translation_id WHERE t.content_id = $1 AND r.version = t.latest_version`, c.id).Scan(&s)
		return s
	}
	if got := usages(); got != "cover,download,image,poster,video" {
		t.Fatalf("recorded usages: %q", got)
	}
	var rev struct {
		CoverAssetID *string `json:"cover_asset_id"`
	}
	e.Do("GET", "/api/v1/admin/contents/"+c.id+"/translations/es/revisions/1", nil).JSON(t, &rev)
	if rev.CoverAssetID == nil || *rev.CoverAssetID != img {
		t.Fatalf("cover not stored: %v", rev.CoverAssetID)
	}
	for _, id := range []string{img, poster, vid, pdf} {
		if r := e.Do("DELETE", "/api/v1/admin/assets/"+id, nil); r.Status != http.StatusConflict {
			t.Errorf("delete %s in use: %d", id, r.Status)
		}
	}
	// Editing library texts never changes the revision's own alt.
	e.Do("PATCH", "/api/v1/admin/assets/"+img, map[string]any{"texts": map[string]any{"es": map[string]string{"alt": "Nuevo alt"}}})
	r := e.Do("GET", "/api/v1/admin/contents/"+c.id+"/translations/es/revisions/1", nil)
	if !bytes.Contains(r.Body, []byte(`"alt":"Placa"`)) || bytes.Contains(r.Body, []byte("Nuevo alt")) {
		t.Fatalf("revision alt changed with the library: %s", r.Body)
	}

	snap := func(b []any, cover string) map[string]any {
		s := map[string]any{"title": "x", "slug": "x", "body": map[string]any{"type": "doc", "content": b}}
		if cover != "" {
			s["cover_asset_id"] = cover
		}
		return s
	}
	for name, s := range map[string]map[string]any{
		"video as image":    snap([]any{imageNode(vid, "x")}, ""),
		"video as poster":   snap([]any{map[string]any{"type": "video", "attrs": map[string]any{"assetId": vid, "posterAssetId": vid}}}, ""),
		"image as video":    snap([]any{map[string]any{"type": "video", "attrs": map[string]any{"assetId": img}}}, ""),
		"pdf as cover":      snap([]any{}, pdf),
		"unknown asset":     snap([]any{imageNode("00000000-0000-4000-8000-000000000001", "x")}, ""),
		"non-uuid asset id": snap([]any{imageNode("fake", "x")}, ""),
	} {
		if r := e.save(t, c.id, 1, s); r.Status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}

	// Restore and copy keep references for the new revisions.
	e.save(t, c.id, 1, snap([]any{}, ""))
	if r := e.Do("POST", "/api/v1/admin/contents/"+c.id+"/translations/es/revisions/1/restore", map[string]any{"expected_version": 2}); r.Status != 201 {
		t.Fatalf("restore: %d %s", r.Status, r.Body)
	}
	if got := usages(); got != "cover,download,image,poster,video" {
		t.Fatalf("restored revision usages: %q", got)
	}
	if r := e.Do("POST", "/api/v1/admin/contents/"+c.id+"/translations", map[string]any{"locale": "en", "copy_from_locale": "es"}); r.Status != 201 {
		t.Fatalf("copy: %d %s", r.Status, r.Body)
	}
	var copied int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM revision_assets ra JOIN revisions r ON r.id = ra.revision_id
		JOIN translations t ON t.id = r.translation_id WHERE t.content_id = $1 AND t.locale = 'en'`, c.id).Scan(&copied)
	if copied != 5 {
		t.Fatalf("copied translation references: %d", copied)
	}
}

func TestDeleteRacesWithSaveNeverLeaveDanglingReferences(t *testing.T) {
	e := startFull(t)
	c := e.newContent(t, "article", "", []any{}, "")
	version := 1
	for i := 0; i < 15; i++ {
		img := e.uploadID(t, "text.png", fmt.Sprintf("r%d.png", i))
		var wg sync.WaitGroup
		var saveStatus, deleteStatus int
		wg.Add(2)
		go func() {
			defer wg.Done()
			saveStatus = e.save(t, c.id, version, map[string]any{"title": fmt.Sprintf("v%d", i), "slug": "x",
				"body": map[string]any{"type": "doc", "content": []any{imageNode(img, "x")}}}).Status
		}()
		go func() {
			defer wg.Done()
			// Staggered start so both interleavings happen: delete first, or save first.
			time.Sleep(time.Duration(i%5) * 3 * time.Millisecond)
			deleteStatus = e.Do("DELETE", "/api/v1/admin/assets/"+img, nil).Status
		}()
		wg.Wait()
		switch {
		case saveStatus == 201 && deleteStatus == 409:
			version++
		case saveStatus == 422 && deleteStatus == 204:
		default:
			t.Fatalf("round %d: save %d, delete %d", i, saveStatus, deleteStatus)
		}
	}
	saves, deletes := version-1, 15-(version-1)
	t.Logf("race outcomes: %d saves won, %d deletes won", saves, deletes)
	if saves == 0 || deletes == 0 {
		t.Fatalf("only one interleaving exercised (saves %d, deletes %d)", saves, deletes)
	}
	var dangling int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM revision_assets ra LEFT JOIN assets a ON a.id = ra.asset_id WHERE a.id IS NULL`).Scan(&dangling)
	if dangling != 0 {
		t.Fatalf("dangling references: %d", dangling)
	}
}
