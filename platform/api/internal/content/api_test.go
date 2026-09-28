package content_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/authtest"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/content"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

func start(t *testing.T) (*authtest.Server, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	srv := authtest.Start(t, pool, func(a *auth.Handler, logger *slog.Logger) []httpapi.Module {
		return []httpapi.Module{content.NewHandler(content.NewStore(pool), logger, a.RequireOwner, auth.Actor)}
	})
	return srv, pool
}

type contentResp struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	ProjectID    *string `json:"project_id"`
	Translations []struct {
		Locale        string  `json:"locale"`
		LatestVersion int     `json:"latest_version"`
		Title         *string `json:"title"`
	} `json:"translations"`
}

type revisionResp struct {
	Version             int      `json:"version"`
	Kind                string   `json:"kind"`
	Title               string   `json:"title"`
	Slug                string   `json:"slug"`
	RestoredFromVersion *int     `json:"restored_from_version"`
	CopiedFromLocale    *string  `json:"copied_from_locale"`
	TagIDs              []string `json:"tag_ids"`
	Body                struct {
		Content []map[string]any `json:"content"`
	} `json:"body"`
}

type saveResp struct {
	Created  bool         `json:"created"`
	Revision revisionResp `json:"revision"`
}

type errorResp struct {
	Code           string            `json:"code"`
	Fields         map[string]string `json:"fields"`
	CurrentVersion *int              `json:"current_version"`
}

func create(t *testing.T, s *authtest.Server, body map[string]any) contentResp {
	t.Helper()
	r := s.Do("POST", "/api/v1/admin/contents", body)
	if r.Status != http.StatusCreated {
		t.Fatalf("create %v = %d %s", body, r.Status, r.Body)
	}
	var c contentResp
	r.JSON(t, &c)
	return c
}

func snapshot(title, text string) map[string]any {
	return map[string]any{
		"title": title, "slug": slugOf(title),
		"body": map[string]any{"type": "doc", "content": []any{
			map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text}}},
		}},
	}
}

func slugOf(s string) string { return strings.ReplaceAll(strings.ToLower(s), " ", "-") }

func revPath(id, locale string) string {
	return fmt.Sprintf("/api/v1/admin/contents/%s/translations/%s/revisions", id, locale)
}

func save(t *testing.T, s *authtest.Server, id, locale string, expected int, snap map[string]any, headers ...string) authtest.Response {
	t.Helper()
	return s.Do("POST", revPath(id, locale), map[string]any{"expected_version": expected, "kind": "manual", "snapshot": snap}, headers...)
}

func TestContentsPersistAndLogNeedsProject(t *testing.T) {
	s, _ := start(t)
	project := create(t, s, map[string]any{"kind": "project", "locale": "es"})
	article := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	log := create(t, s, map[string]any{"kind": "log", "locale": "es", "project_id": project.ID})
	if log.ProjectID == nil || *log.ProjectID != project.ID {
		t.Fatalf("log parent = %v", log.ProjectID)
	}

	for name, body := range map[string]map[string]any{
		"log without project":   {"kind": "log", "locale": "es"},
		"log under article":     {"kind": "log", "locale": "es", "project_id": article.ID},
		"log under unknown":     {"kind": "log", "locale": "es", "project_id": "00000000-0000-4000-8000-000000000000"},
		"article with parent":   {"kind": "article", "locale": "es", "project_id": project.ID},
		"unknown kind":          {"kind": "page", "locale": "es"},
		"unknown locale":        {"kind": "article", "locale": "fr"},
		"unknown field":         {"kind": "article", "locale": "es", "published": true},
		"kind change attempted": {"kind": "article", "locale": "es", "id": project.ID},
	} {
		if r := s.Do("POST", "/api/v1/admin/contents", body); r.Status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}

	if r := save(t, s, project.ID, "es", 0, snapshot("Rover", "Primer texto")); r.Status != http.StatusCreated {
		t.Fatalf("save: %d %s", r.Status, r.Body)
	}
	// Reloading returns what was saved.
	var tr struct {
		LatestVersion int          `json:"latest_version"`
		Latest        revisionResp `json:"latest"`
	}
	s.Do("GET", fmt.Sprintf("/api/v1/admin/contents/%s/translations/es", project.ID), nil).JSON(t, &tr)
	if tr.LatestVersion != 1 || tr.Latest.Title != "Rover" || tr.Latest.Body.Content[0]["type"] != "paragraph" {
		t.Fatalf("reloaded translation: %+v", tr)
	}

	var list struct {
		Items []contentResp `json:"items"`
		Total int           `json:"total"`
	}
	s.Do("GET", "/api/v1/admin/contents?kind=log&project_id="+project.ID, nil).JSON(t, &list)
	if list.Total != 1 || list.Items[0].ID != log.ID {
		t.Fatalf("logs of project: %+v", list)
	}
	s.Do("GET", "/api/v1/admin/contents?page_size=2", nil).JSON(t, &list)
	if list.Total != 3 || len(list.Items) != 2 {
		t.Fatalf("pagination: total=%d items=%d", list.Total, len(list.Items))
	}
}

func TestTranslationsAreIndependent(t *testing.T) {
	s, _ := start(t)
	c := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	save(t, s, c.ID, "es", 0, snapshot("Hola", "texto es"))
	if r := s.Do("POST", "/api/v1/admin/contents/"+c.ID+"/translations", map[string]any{"locale": "es"}); r.Status != http.StatusConflict {
		t.Fatalf("duplicate locale: %d", r.Status)
	}
	if r := s.Do("POST", "/api/v1/admin/contents/"+c.ID+"/translations", map[string]any{"locale": "en"}); r.Status != http.StatusCreated {
		t.Fatalf("create en: %d %s", r.Status, r.Body)
	}
	save(t, s, c.ID, "en", 0, snapshot("Hello", "english text"))
	save(t, s, c.ID, "es", 1, snapshot("Hola dos", "texto es 2"))

	var got contentResp
	s.Do("GET", "/api/v1/admin/contents/"+c.ID, nil).JSON(t, &got)
	versions := map[string]int{}
	titles := map[string]string{}
	for _, tr := range got.Translations {
		versions[tr.Locale] = tr.LatestVersion
		titles[tr.Locale] = *tr.Title
	}
	if versions["es"] != 2 || versions["en"] != 1 || titles["es"] != "Hola dos" || titles["en"] != "Hello" {
		t.Fatalf("translations mixed up: %v %v", versions, titles)
	}
}

func TestCopyTranslationIsMarkedAsCopy(t *testing.T) {
	s, _ := start(t)
	c := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	if r := s.Do("POST", "/api/v1/admin/contents/"+c.ID+"/translations", map[string]any{"locale": "en", "copy_from_locale": "es"}); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("copy from empty translation: %d", r.Status)
	}
	save(t, s, c.ID, "es", 0, snapshot("Hola", "texto"))
	s.Do("POST", "/api/v1/admin/contents/"+c.ID+"/translations", map[string]any{"locale": "en", "copy_from_locale": "es"})
	var rev revisionResp
	s.Do("GET", revPath(c.ID, "en")+"/1", nil).JSON(t, &rev)
	if rev.Kind != "copy" || rev.CopiedFromLocale == nil || *rev.CopiedFromLocale != "es" || rev.Title != "Hola" {
		t.Fatalf("copied revision: %+v", rev)
	}
}

func TestVersionConflictKeepsBothSidesHonest(t *testing.T) {
	s, _ := start(t)
	c := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	save(t, s, c.ID, "es", 0, snapshot("Base", "v1"))

	// Two tabs start from version 1 and save different text.
	if r := save(t, s, c.ID, "es", 1, snapshot("Base", "pestaña A")); r.Status != http.StatusCreated {
		t.Fatalf("tab A: %d", r.Status)
	}
	r := save(t, s, c.ID, "es", 1, snapshot("Base", "pestaña B"))
	var e errorResp
	r.JSON(t, &e)
	if r.Status != http.StatusConflict || e.Code != "version_conflict" || e.CurrentVersion == nil || *e.CurrentVersion != 2 {
		t.Fatalf("tab B: %d %s", r.Status, r.Body)
	}
	var rev revisionResp
	s.Do("GET", revPath(c.ID, "es")+"/2", nil).JSON(t, &rev)
	if rev.Body.Content[0]["content"].([]any)[0].(map[string]any)["text"] != "pestaña A" {
		t.Fatalf("tab B overwrote tab A: %+v", rev)
	}
}

func TestConcurrentSavesProduceExactlyOneRevision(t *testing.T) {
	s, _ := start(t)
	c := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	var wg sync.WaitGroup
	statuses := make([]int, 8)
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = save(t, s, c.ID, "es", 0, snapshot("Carrera", fmt.Sprintf("texto %d", i))).Status
		}()
	}
	wg.Wait()
	created, conflicts := 0, 0
	for _, st := range statuses {
		switch st {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		}
	}
	if created != 1 || conflicts != len(statuses)-1 {
		t.Fatalf("statuses %v: want one 201 and the rest 409", statuses)
	}
}

func TestUnchangedSnapshotAndIdempotentRetry(t *testing.T) {
	s, _ := start(t)
	c := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	save(t, s, c.ID, "es", 0, snapshot("Igual", "mismo"))

	var out saveResp
	r := save(t, s, c.ID, "es", 1, snapshot("Igual", "mismo"))
	r.JSON(t, &out)
	if r.Status != http.StatusOK || out.Created || out.Revision.Version != 1 {
		t.Fatalf("unchanged save: %d %s", r.Status, r.Body)
	}

	key := []string{"Idempotency-Key", "save-abcdef-0001"}
	first := save(t, s, c.ID, "es", 1, snapshot("Igual", "nuevo"), key...)
	// The response was lost; the client retries the same operation with its original expected_version.
	retry := save(t, s, c.ID, "es", 1, snapshot("Igual", "nuevo"), key...)
	var a, b saveResp
	first.JSON(t, &a)
	retry.JSON(t, &b)
	if first.Status != http.StatusCreated || retry.Status != http.StatusCreated || a.Revision.Version != 2 || b.Revision.Version != 2 {
		t.Fatalf("idempotent retry: %d/%d versions %d/%d", first.Status, retry.Status, a.Revision.Version, b.Revision.Version)
	}
	if r := save(t, s, c.ID, "es", 2, snapshot("Igual", "otra cosa"), key...); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("key reused for another snapshot: %d %s", r.Status, r.Body)
	}
	var hist struct {
		Total int `json:"total"`
	}
	s.Do("GET", revPath(c.ID, "es"), nil).JSON(t, &hist)
	if hist.Total != 2 {
		t.Fatalf("revisions = %d, want 2 (no duplicates)", hist.Total)
	}
}

func TestRestoreAndSaveNeverTouchSnapshotsOrPublication(t *testing.T) {
	s, pool := start(t)
	ctx := context.Background()
	c := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	save(t, s, c.ID, "es", 0, snapshot("Uno", "primera"))
	save(t, s, c.ID, "es", 1, snapshot("Dos", "segunda"))

	// Fixture: version 1 is "published" (WEB-005 will own this; there is no endpoint for it).
	if _, err := pool.Exec(ctx, `UPDATE translations t SET published_revision_id = r.id FROM revisions r
		WHERE r.translation_id = t.id AND r.version = 1 AND t.content_id = $1`, c.ID); err != nil {
		t.Fatal(err)
	}
	fingerprint := func() string {
		var fp string
		if err := pool.QueryRow(ctx, `SELECT string_agg(r.version || ':' || r.title || ':' || encode(r.snapshot_hash, 'hex') || ':' || r.body_json::text, '|' ORDER BY r.version)
			|| '#' || (SELECT published_revision_id::text FROM translations WHERE content_id = $1)
			FROM revisions r JOIN translations t ON t.id = r.translation_id WHERE t.content_id = $1 AND r.version <= 2`, c.ID).Scan(&fp); err != nil {
			t.Fatal(err)
		}
		return fp
	}
	before := fingerprint()

	var out saveResp
	r := s.Do("POST", revPath(c.ID, "es")+"/1/restore", map[string]any{"expected_version": 2})
	r.JSON(t, &out)
	if r.Status != http.StatusCreated || out.Revision.Version != 3 || out.Revision.Kind != "restore" ||
		out.Revision.RestoredFromVersion == nil || *out.Revision.RestoredFromVersion != 1 || out.Revision.Title != "Uno" {
		t.Fatalf("restore: %d %s", r.Status, r.Body)
	}
	save(t, s, c.ID, "es", 3, snapshot("Cuatro", "cuarta"))
	if after := fingerprint(); after != before {
		t.Fatalf("earlier snapshots or publication changed:\n%s\n%s", before, after)
	}
	if r := s.Do("POST", revPath(c.ID, "es")+"/1/restore", map[string]any{"expected_version": 2}); r.Status != http.StatusConflict {
		t.Fatalf("restore with stale version: %d", r.Status)
	}
	// The database itself refuses to rewrite history.
	if _, err := pool.Exec(ctx, `UPDATE revisions SET title = 'x'`); err == nil {
		t.Fatal("revisions are writable")
	}
}

func TestArchiveRules(t *testing.T) {
	s, pool := start(t)
	c := create(t, s, map[string]any{"kind": "project", "locale": "es"})
	save(t, s, c.ID, "es", 0, snapshot("Proyecto", "x"))
	if r := s.Do("POST", "/api/v1/admin/contents/"+c.ID+"/archive", nil); r.Status != http.StatusOK {
		t.Fatalf("archive: %d", r.Status)
	}
	if r := save(t, s, c.ID, "es", 1, snapshot("Proyecto", "y")); r.Status != http.StatusConflict {
		t.Fatalf("save on archived: %d", r.Status)
	}
	if r := s.Do("POST", "/api/v1/admin/contents", map[string]any{"kind": "log", "locale": "es", "project_id": c.ID}); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("log under archived project: %d", r.Status)
	}
	var hist struct {
		Total int `json:"total"`
	}
	s.Do("GET", revPath(c.ID, "es"), nil).JSON(t, &hist)
	if hist.Total != 1 {
		t.Fatalf("archiving lost history: %d", hist.Total)
	}
	s.Do("POST", "/api/v1/admin/contents/"+c.ID+"/unarchive", nil)

	if _, err := pool.Exec(context.Background(), `UPDATE translations t SET published_revision_id = r.id FROM revisions r
		WHERE r.translation_id = t.id AND t.content_id = $1`, c.ID); err != nil {
		t.Fatal(err)
	}
	var e errorResp
	r := s.Do("POST", "/api/v1/admin/contents/"+c.ID+"/archive", nil)
	r.JSON(t, &e)
	if r.Status != http.StatusConflict || e.Code != "published_content" {
		t.Fatalf("archive published: %d %s", r.Status, r.Body)
	}
}

func TestMaliciousAndOversizedBodies(t *testing.T) {
	s, _ := start(t)
	c := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	withBody := func(body any) map[string]any {
		return map[string]any{"title": "x", "slug": "x", "body": body}
	}
	cases := map[string]map[string]any{
		"javascript link": withBody(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{
			map[string]any{"type": "text", "text": "x", "marks": []any{map[string]any{"type": "link", "attrs": map[string]any{"href": "javascript:alert(1)"}}}}}}}}),
		"unknown node": withBody(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "iframe", "attrs": map[string]any{"src": "https://x"}}}}),
		"bad slug":     {"title": "x", "slug": "../etc", "body": map[string]any{"type": "doc", "content": []any{}}},
		"project fields on article": {"title": "x", "slug": "x", "body": map[string]any{"type": "doc", "content": []any{}},
			"project_fields": map[string]any{"status": "idea"}},
		"unknown snapshot key": {"title": "x", "slug": "x", "body": map[string]any{"type": "doc", "content": []any{}}, "html": "<b>"},
	}
	for name, snap := range cases {
		if r := save(t, s, c.ID, "es", 0, snap); r.Status != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	var e errorResp
	media := withBody(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "image", "attrs": map[string]any{
		"assetId": "0b8f3a3e-5d2c-4c1a-9a57-3f0e2b6d9c11", "alt": "x"}}}})
	r := save(t, s, c.ID, "es", 0, media)
	r.JSON(t, &e)
	// WEB-004: media nodes are accepted only for existing, ready assets (never invented ids).
	if r.Status != http.StatusUnprocessableEntity || e.Code != "validation_failed" || !strings.Contains(e.Fields["body"], "no existe") {
		t.Errorf("media with an unknown asset: %d %s", r.Status, r.Body)
	}
	huge := fmt.Sprintf(`{"expected_version":0,"kind":"manual","snapshot":{"title":"x","slug":"x","body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":%q}]}]}}}`,
		strings.Repeat("a", 1<<20))
	if r := s.Do("POST", revPath(c.ID, "es"), huge); r.Status != http.StatusRequestEntityTooLarge {
		t.Errorf("1 MiB+ body: %d", r.Status)
	}
	if r := s.Do("POST", revPath(c.ID, "es"), map[string]any{"kind": "manual", "snapshot": snapshot("x", "y")}); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("missing expected_version: %d", r.Status)
	}
	var hist struct {
		Total int `json:"total"`
	}
	s.Do("GET", revPath(c.ID, "es"), nil).JSON(t, &hist)
	if hist.Total != 0 {
		t.Fatalf("a rejected body was stored: %d revisions", hist.Total)
	}
}

func TestTaxonomyInSnapshots(t *testing.T) {
	s, _ := start(t)
	var cat, tag struct {
		ID string `json:"id"`
	}
	s.Do("POST", "/api/v1/admin/categories", map[string]any{"slug": "robotica", "labels": map[string]string{"es": "Robótica", "en": "Robotics"}}).JSON(t, &cat)
	s.Do("POST", "/api/v1/admin/tags", map[string]any{"slug": "esp32", "labels": map[string]string{"es": "ESP32", "en": "ESP32"}}).JSON(t, &tag)
	if r := s.Do("POST", "/api/v1/admin/tags", map[string]any{"slug": "esp32", "labels": map[string]string{"es": "a", "en": "b"}}); r.Status != http.StatusConflict {
		t.Fatalf("duplicate tag slug: %d", r.Status)
	}
	c := create(t, s, map[string]any{"kind": "project", "locale": "es"})
	snap := snapshot("Rover", "x")
	snap["category_id"] = cat.ID
	snap["tag_ids"] = []string{tag.ID}
	snap["project_fields"] = map[string]any{"status": "in_development", "technologies": []string{"ESP32"},
		"links": []map[string]string{{"label": "Repo", "url": "https://github.com/Codelab-ai-dev/brambiLab"}}}
	if r := save(t, s, c.ID, "es", 0, snap); r.Status != http.StatusCreated {
		t.Fatalf("save with taxonomy: %d %s", r.Status, r.Body)
	}
	// Renaming a tag keeps its id, so past revisions still point at the same tag.
	if r := s.Do("PATCH", "/api/v1/admin/tags/"+tag.ID, map[string]any{"slug": "esp-32", "labels": map[string]string{"es": "ESP-32", "en": "ESP-32"}}); r.Status != http.StatusOK {
		t.Fatalf("rename tag: %d", r.Status)
	}
	var rev revisionResp
	s.Do("GET", revPath(c.ID, "es")+"/1", nil).JSON(t, &rev)
	if len(rev.TagIDs) != 1 || rev.TagIDs[0] != tag.ID {
		t.Fatalf("revision tags: %v", rev.TagIDs)
	}
	snap["tag_ids"] = []string{"00000000-0000-4000-8000-000000000009"}
	if r := save(t, s, c.ID, "es", 1, snap); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown tag: %d", r.Status)
	}
}

func TestPrivateAndProtected(t *testing.T) {
	s, _ := start(t)
	c := create(t, s, map[string]any{"kind": "article", "locale": "es"})
	save(t, s, c.ID, "es", 0, snapshot("Secreto", "borrador privado"))
	for _, p := range []string{"/api/v1/admin/contents", "/api/v1/admin/contents/" + c.ID,
		"/api/v1/admin/contents/" + c.ID + "/translations/es", revPath(c.ID, "es"), revPath(c.ID, "es") + "/1",
		"/api/v1/admin/categories", "/api/v1/admin/tags"} {
		r := s.Anonymous("GET", p, nil)
		if r.Status != http.StatusUnauthorized || strings.Contains(string(r.Body), "Secreto") || strings.Contains(string(r.Body), "borrador") {
			t.Errorf("anonymous GET %s: %d %s", p, r.Status, r.Body)
		}
		if got := s.Do("GET", p, nil); !strings.Contains(got.Header.Get("Cache-Control"), "no-store") {
			t.Errorf("GET %s Cache-Control = %q", p, got.Header.Get("Cache-Control"))
		}
	}
	if r := s.WithoutCSRF("POST", revPath(c.ID, "es"), map[string]any{"expected_version": 1, "kind": "manual", "snapshot": snapshot("x", "y")}); r.Status != http.StatusForbidden {
		t.Errorf("save without CSRF: %d", r.Status)
	}
	if r := s.Do("POST", "/api/v1/admin/contents", map[string]any{"kind": "article", "locale": "es"}, "Origin", "https://evil.example"); r.Status != http.StatusForbidden {
		t.Errorf("cross-origin create: %d", r.Status)
	}
}
