package publishing_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/authtest"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/content"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/media"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/publishing"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

type env struct {
	*authtest.Server
	pool *pgxpool.Pool
	svc  *publishing.Service
	now  atomic.Pointer[time.Time]
}

// start mounts auth, content, media and publishing like the real server, with a fixed clock.
func start(t *testing.T) *env {
	t.Helper()
	pool := testdb.New(t)
	local, err := media.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lim := media.DefaultLimits()
	lim.FreeReserve = 0
	e := &env{pool: pool, svc: publishing.NewService(pool)}
	e.setNow(time.Date(2026, 9, 28, 16, 0, 0, 0, time.UTC)) // 10:00 in Mexico City
	e.svc.Now = func() time.Time { return *e.now.Load() }
	e.Server = authtest.Start(t, pool, func(a *auth.Handler, logger *slog.Logger) []httpapi.Module {
		return []httpapi.Module{
			content.NewHandler(content.NewStore(pool), logger, a.RequireOwner, auth.Actor),
			media.NewHandler(media.NewStore(pool), local, lim, logger, a.RequireOwner, auth.Actor, a.IsOwner),
			publishing.NewHandler(e.svc, logger, a.RequireOwner, auth.Actor),
		}
	})
	return e
}

func (e *env) setNow(t time.Time) { e.now.Store(&t) }

// --- helpers --------------------------------------------------------------------------------------

type state struct {
	Status           string  `json:"status"`
	EditorialVersion int     `json:"editorial_version"`
	LatestVersion    int     `json:"latest_version"`
	PublishedVersion *int    `json:"published_version"`
	PublishedAt      *string `json:"published_at"`
	FirstPublishedAt *string `json:"first_published_at"`
	WithdrawnAt      *string `json:"withdrawn_at"`
	Route            *string `json:"route"`
	TimeZone         string  `json:"time_zone"`
	ActiveJob        *struct {
		ID              string `json:"id"`
		RevisionVersion int    `json:"revision_version"`
		RunAt           string `json:"run_at"`
		RunAtLocal      string `json:"run_at_local"`
		Status          string `json:"status"`
	} `json:"active_job"`
}

type apiError struct {
	Code           string            `json:"code"`
	Fields         map[string]string `json:"fields"`
	CurrentVersion *int              `json:"current_version"`
	State          *state            `json:"state"`
}

func decode[T any](t *testing.T, r authtest.Response) T {
	t.Helper()
	var v T
	r.JSON(t, &v)
	return v
}

func (e *env) create(t *testing.T, kind, locale, projectID string) string {
	t.Helper()
	body := map[string]any{"kind": kind, "locale": locale}
	if projectID != "" {
		body["project_id"] = projectID
	}
	r := e.Do("POST", "/api/v1/admin/contents", body)
	if r.Status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.Status, r.Body)
	}
	return decode[struct {
		ID string `json:"id"`
	}](t, r).ID
}

func doc(nodes ...any) map[string]any {
	if len(nodes) == 0 {
		nodes = []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Texto"}}}}
	}
	return map[string]any{"type": "doc", "content": nodes}
}

func snap(title, slug string, nodes ...any) map[string]any {
	return map[string]any{"title": title, "slug": slug, "body": doc(nodes...)}
}

// save stores a new revision and returns its version.
func (e *env) save(t *testing.T, id, locale string, s map[string]any) int {
	t.Helper()
	path := fmt.Sprintf("/api/v1/admin/contents/%s/translations/%s", id, locale)
	latest := decode[struct {
		LatestVersion int `json:"latest_version"`
	}](t, e.Do("GET", path, nil)).LatestVersion
	r := e.Do("POST", path+"/revisions", map[string]any{"expected_version": latest, "kind": "manual", "snapshot": s})
	if r.Status != http.StatusCreated {
		t.Fatalf("save: %d %s", r.Status, r.Body)
	}
	return latest + 1
}

func pubPath(id, locale, action string) string {
	p := fmt.Sprintf("/api/v1/admin/contents/%s/translations/%s/publication", id, locale)
	if action != "" {
		p += "/" + action
	}
	return p
}

func (e *env) state(t *testing.T, id, locale string) state {
	t.Helper()
	r := e.Do("GET", pubPath(id, locale, ""), nil)
	if r.Status != http.StatusOK {
		t.Fatalf("state: %d %s", r.Status, r.Body)
	}
	return decode[state](t, r)
}

func (e *env) act(id, locale, action string, body map[string]any, headers ...string) authtest.Response {
	return e.Do("POST", pubPath(id, locale, action), body, headers...)
}

// publish publishes version with the current editorial version and requires success.
func (e *env) publish(t *testing.T, id, locale string, version int) state {
	t.Helper()
	ed := e.state(t, id, locale).EditorialVersion
	r := e.act(id, locale, "publish", map[string]any{"revision_version": version, "expected_editorial_version": ed})
	if r.Status != http.StatusOK {
		t.Fatalf("publish %s v%d: %d %s", locale, version, r.Status, r.Body)
	}
	return decode[state](t, r)
}

func (e *env) simple(t *testing.T, id, locale, action string) authtest.Response {
	t.Helper()
	return e.act(id, locale, action, map[string]any{"expected_editorial_version": e.state(t, id, locale).EditorialVersion})
}

func (e *env) schedule(t *testing.T, id, locale string, version int, local string, replace bool) authtest.Response {
	t.Helper()
	return e.act(id, locale, "schedule", map[string]any{"revision_version": version, "run_at_local": local,
		"time_zone": "America/Mexico_City", "replace": replace, "expected_editorial_version": e.state(t, id, locale).EditorialVersion})
}

func requireStatus(t *testing.T, what string, r authtest.Response, status int, code string) apiError {
	t.Helper()
	if r.Status != status {
		t.Fatalf("%s: %d %s, want %d %s", what, r.Status, r.Body, status, code)
	}
	var e apiError
	if code != "" {
		r.JSON(t, &e)
		if e.Code != code {
			t.Fatalf("%s: code %q, want %q (%s)", what, e.Code, code, r.Body)
		}
	}
	return e
}

func (e *env) public(t *testing.T, path string) authtest.Response {
	t.Helper()
	req, err := http.NewRequest("GET", e.Origin+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("GET %s: Cache-Control %q, want no-store", path, cc)
	}
	return authtest.Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
}

func (e *env) publicTitle(t *testing.T, path string) string {
	t.Helper()
	r := e.public(t, path)
	if r.Status != http.StatusOK {
		t.Fatalf("public %s: %d %s", path, r.Status, r.Body)
	}
	return decode[struct {
		Title string `json:"title"`
	}](t, r).Title
}

func (e *env) requireGone(t *testing.T, path string) {
	t.Helper()
	if r := e.public(t, path); r.Status != http.StatusNotFound {
		t.Fatalf("public %s: %d %s, want 404", path, r.Status, r.Body)
	}
}

func (e *env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// --- tests ----------------------------------------------------------------------------------------

func TestPublishWithdrawAndAliases(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	v1 := e.save(t, a, "es", snap("Rover solar", "rover-solar"))

	st := e.state(t, a, "es")
	if st.Status != "unpublished" || st.EditorialVersion != 0 || st.LatestVersion != 1 || st.Route != nil || st.TimeZone != "America/Mexico_City" {
		t.Fatalf("initial state %+v", st)
	}
	e.requireGone(t, "/api/v1/public/es/articles/rover-solar") // a draft is never public

	st = e.publish(t, a, "es", v1)
	if st.Status != "published" || st.EditorialVersion != 1 || *st.PublishedVersion != 1 || st.Route == nil ||
		*st.Route != "/api/v1/public/es/articles/rover-solar" || st.FirstPublishedAt == nil {
		t.Fatalf("after publish %+v", st)
	}
	if got := e.publicTitle(t, "/api/v1/public/es/articles/rover-solar"); got != "Rover solar" {
		t.Fatalf("public title %q", got)
	}
	e.requireGone(t, "/api/v1/public/en/articles/rover-solar") // the other locale is independent
	e.requireGone(t, "/api/v1/public/es/projects/rover-solar") // the scope is part of the route

	// A newer draft changes nothing publicly and does not touch editorial_version.
	v2 := e.save(t, a, "es", snap("Rover solar v2", "rover-autonomo"))
	if got := e.publicTitle(t, "/api/v1/public/es/articles/rover-solar"); got != "Rover solar" {
		t.Fatalf("draft leaked: %q", got)
	}
	if st := e.state(t, a, "es"); st.EditorialVersion != 1 || st.LatestVersion != 2 || *st.PublishedVersion != 1 {
		t.Fatalf("after draft %+v", st)
	}

	// Publishing the same revision again is a no-op.
	if st := e.publish(t, a, "es", v1); st.EditorialVersion != 1 {
		t.Fatalf("identical publish bumped the version: %+v", st)
	}

	// New slug: the old one becomes an alias answering 301 to the current route.
	st = e.publish(t, a, "es", v2)
	if *st.Route != "/api/v1/public/es/articles/rover-autonomo" || st.EditorialVersion != 2 {
		t.Fatalf("after slug change %+v", st)
	}
	r := e.public(t, "/api/v1/public/es/articles/rover-solar")
	if r.Status != http.StatusMovedPermanently || r.Header.Get("Location") != "/api/v1/public/es/articles/rover-autonomo" {
		t.Fatalf("alias: %d %v", r.Status, r.Header)
	}
	// Back to the first slug: the alias becomes current again and the other one redirects (no loop).
	st = e.publish(t, a, "es", v1)
	if *st.Route != "/api/v1/public/es/articles/rover-solar" {
		t.Fatalf("slug back %+v", st)
	}
	if r := e.public(t, "/api/v1/public/es/articles/rover-autonomo"); r.Status != http.StatusMovedPermanently || r.Header.Get("Location") != "/api/v1/public/es/articles/rover-solar" {
		t.Fatalf("reverse alias: %d %v", r.Status, r.Header)
	}
	if n := e.count(t, `SELECT count(*) FROM public_routes WHERE content_id = $1 AND is_current`, a); n != 1 {
		t.Fatalf("current routes = %d", n)
	}

	// Withdraw: both paths disappear (no redirect either), revisions are kept.
	r = e.simple(t, a, "es", "withdraw")
	requireStatus(t, "withdraw", r, http.StatusOK, "")
	st = decode[state](t, r)
	if st.Status != "withdrawn" || st.PublishedVersion != nil || st.Route != nil || st.WithdrawnAt == nil || st.FirstPublishedAt == nil {
		t.Fatalf("after withdraw %+v", st)
	}
	e.requireGone(t, "/api/v1/public/es/articles/rover-solar")
	e.requireGone(t, "/api/v1/public/es/articles/rover-autonomo")
	if n := e.count(t, `SELECT count(*) FROM revisions r JOIN translations t ON t.id = r.translation_id WHERE t.content_id = $1`, a); n != 2 {
		t.Fatalf("revisions after withdraw = %d", n)
	}
	requireStatus(t, "withdraw again", e.simple(t, a, "es", "withdraw"), http.StatusConflict, "not_published")

	// Republish restores the public page; first_published_at is kept.
	first := *st.FirstPublishedAt
	e.setNow(e.now.Load().Add(time.Hour))
	st = e.publish(t, a, "es", v2)
	if st.Status != "published" || st.WithdrawnAt != nil || *st.FirstPublishedAt != first || *st.PublishedAt == first {
		t.Fatalf("republish %+v", st)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_events WHERE entity_id = $1 AND action LIKE 'publication.%'`, a); n != 5 {
		t.Fatalf("publication audit events = %d, want 5", n)
	}
}

func TestEditorialConflictsAndIdempotency(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	v := e.save(t, a, "es", snap("Brazo", "brazo"))

	body := map[string]any{"revision_version": v, "expected_editorial_version": 0}
	key := []string{"Idempotency-Key", "publish-brazo-1"}
	first := e.act(a, "es", "publish", body, key...)
	requireStatus(t, "publish", first, http.StatusOK, "")

	// Same key and body: the original answer, even though the version moved on.
	again := e.act(a, "es", "publish", body, key...)
	if again.Status != http.StatusOK || string(again.Body) == "" || again.Header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %d %v %s", again.Status, again.Header, again.Body)
	}
	if decode[state](t, again).EditorialVersion != decode[state](t, first).EditorialVersion {
		t.Fatalf("replay differs: %s vs %s", again.Body, first.Body)
	}
	// Same key, other body.
	requireStatus(t, "key reuse", e.act(a, "es", "withdraw", map[string]any{"expected_editorial_version": 1}, key...),
		http.StatusUnprocessableEntity, "idempotency_key_reused")
	requireStatus(t, "key reuse same action", e.act(a, "es", "publish", map[string]any{"revision_version": v, "expected_editorial_version": 1}, key...),
		http.StatusUnprocessableEntity, "idempotency_key_reused")
	requireStatus(t, "bad key", e.act(a, "es", "publish", body, "Idempotency-Key", "x"), http.StatusUnprocessableEntity, "validation_failed")

	// Stale editorial version: 409 with the current state; nothing changes.
	errBody := requireStatus(t, "stale", e.act(a, "es", "withdraw", map[string]any{"expected_editorial_version": 0}), http.StatusConflict, "editorial_conflict")
	if errBody.State == nil || errBody.State.EditorialVersion != 1 || errBody.State.Status != "published" || *errBody.CurrentVersion != 1 {
		t.Fatalf("conflict body %+v", errBody)
	}
	if st := e.state(t, a, "es"); st.Status != "published" {
		t.Fatalf("stale withdraw changed the state: %+v", st)
	}

	// Strict bodies.
	for name, b := range map[string]any{
		"missing expected": map[string]any{"revision_version": v},
		"missing version":  map[string]any{"expected_editorial_version": 1},
		"unknown field":    map[string]any{"revision_version": v, "expected_editorial_version": 1, "force": true},
		"negative":         map[string]any{"revision_version": v, "expected_editorial_version": -1},
		"not json":         "{",
		"two objects":      `{"revision_version":1,"expected_editorial_version":1}{}`,
	} {
		requireStatus(t, name, e.Do("POST", pubPath(a, "es", "publish"), b), http.StatusUnprocessableEntity, "validation_failed")
	}
	requireStatus(t, "unknown revision", e.act(a, "es", "publish", map[string]any{"revision_version": 9, "expected_editorial_version": 1}),
		http.StatusUnprocessableEntity, "validation_failed")
	requireStatus(t, "unknown translation", e.act(a, "en", "publish", map[string]any{"revision_version": 1, "expected_editorial_version": 0}),
		http.StatusNotFound, "not_found")
	requireStatus(t, "bad locale", e.act(a, "fr", "publish", map[string]any{"revision_version": 1, "expected_editorial_version": 0}),
		http.StatusNotFound, "not_found")
}

func (e *env) upload(t *testing.T, file, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "media", "testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	r := e.Do("POST", "/api/v1/admin/assets?filename="+name, data)
	if r.Status != http.StatusCreated {
		t.Fatalf("upload %s: %d %s", file, r.Status, r.Body)
	}
	return decode[struct {
		ID string `json:"id"`
	}](t, r).ID
}

func (e *env) setAsset(t *testing.T, id string, public, downloadable bool) {
	t.Helper()
	if r := e.Do("PATCH", "/api/v1/admin/assets/"+id, map[string]any{"public_enabled": public, "downloadable": downloadable}); r.Status != http.StatusOK {
		t.Fatalf("patch asset: %d %s", r.Status, r.Body)
	}
}

func TestPublishValidatesWithoutFixingAnything(t *testing.T) {
	e := start(t)
	img := e.upload(t, "text.png", "placa.png")
	pdf := e.upload(t, "doc.pdf", "ficha.pdf")
	a := e.create(t, "article", "es", "")
	s := snap("Placa", "placa",
		map[string]any{"type": "image", "attrs": map[string]any{"assetId": img, "alt": "Placa"}},
		map[string]any{"type": "download", "attrs": map[string]any{"assetId": pdf, "label": "Ficha"}})
	s["cover_asset_id"] = img
	v := e.save(t, a, "es", s)

	pub := func() authtest.Response {
		return e.act(a, "es", "publish", map[string]any{"revision_version": v, "expected_editorial_version": e.state(t, a, "es").EditorialVersion})
	}
	blocked := requireStatus(t, "private media", pub(), http.StatusUnprocessableEntity, "publish_blocked")
	if len(blocked.Fields) != 2 || blocked.Fields["assets."+img] == "" || blocked.Fields["assets."+pdf] == "" {
		t.Fatalf("problems %+v", blocked.Fields)
	}
	e.setAsset(t, img, true, false)
	e.setAsset(t, pdf, true, false)
	blocked = requireStatus(t, "download not downloadable", pub(), http.StatusUnprocessableEntity, "publish_blocked")
	if len(blocked.Fields) != 1 || !strings.Contains(blocked.Fields["assets."+pdf], "descarga") {
		t.Fatalf("problems %+v", blocked.Fields)
	}
	// Nothing was enabled behind the owner's back and nothing was published.
	if n := e.count(t, `SELECT count(*) FROM assets WHERE downloadable`); n != 0 {
		t.Fatalf("assets auto-enabled: %d", n)
	}
	if st := e.state(t, a, "es"); st.Status != "unpublished" || st.EditorialVersion != 0 {
		t.Fatalf("blocked publish changed state %+v", st)
	}
	e.setAsset(t, pdf, true, true)
	requireStatus(t, "all ready", pub(), http.StatusOK, "")

	// Archived content cannot be published (it must be withdrawn first to archive at all).
	b := e.create(t, "article", "es", "")
	vb := e.save(t, b, "es", snap("Otro", "otro"))
	if r := e.Do("POST", "/api/v1/admin/contents/"+b+"/archive", nil); r.Status != http.StatusOK {
		t.Fatalf("archive: %d %s", r.Status, r.Body)
	}
	blocked = requireStatus(t, "archived", e.act(b, "es", "publish", map[string]any{"revision_version": vb, "expected_editorial_version": 0}),
		http.StatusUnprocessableEntity, "publish_blocked")
	if blocked.Fields["content"] == "" {
		t.Fatalf("problems %+v", blocked.Fields)
	}
	requireStatus(t, "archive published", e.Do("POST", "/api/v1/admin/contents/"+a+"/archive", nil), http.StatusConflict, "published_content")
}

func TestLogsNeedTheirProjectAndBlockItsWithdrawal(t *testing.T) {
	e := start(t)
	p := e.create(t, "project", "es", "")
	pv := e.save(t, p, "es", snap("Rover", "rover"))
	l := e.create(t, "log", "es", p)
	lv := e.save(t, l, "es", snap("Día 1", "dia-1"))

	blocked := requireStatus(t, "log before project", e.act(l, "es", "publish", map[string]any{"revision_version": lv, "expected_editorial_version": 0}),
		http.StatusUnprocessableEntity, "publish_blocked")
	if blocked.Fields["project"] == "" {
		t.Fatalf("problems %+v", blocked.Fields)
	}
	e.publish(t, p, "es", pv)
	st := e.publish(t, l, "es", lv)
	if *st.Route != "/api/v1/public/es/projects/rover/logs/dia-1" {
		t.Fatalf("log route %v", *st.Route)
	}
	r := e.public(t, "/api/v1/public/es/projects/rover/logs/dia-1")
	pc := decode[struct {
		Title   string `json:"title"`
		Project struct {
			Slug  string `json:"slug"`
			Title string `json:"title"`
		} `json:"project"`
	}](t, r)
	if r.Status != 200 || pc.Title != "Día 1" || pc.Project.Slug != "rover" || pc.Project.Title != "Rover" {
		t.Fatalf("public log %d %s", r.Status, r.Body)
	}

	// The project slug changes: the log keeps its identity scope, the old project path redirects.
	pv2 := e.save(t, p, "es", snap("Rover", "rover-marte"))
	e.publish(t, p, "es", pv2)
	r = e.public(t, "/api/v1/public/es/projects/rover/logs/dia-1")
	if r.Status != http.StatusMovedPermanently || r.Header.Get("Location") != "/api/v1/public/es/projects/rover-marte/logs/dia-1" {
		t.Fatalf("log via project alias: %d %v", r.Status, r.Header)
	}
	if got := e.publicTitle(t, "/api/v1/public/es/projects/rover-marte/logs/dia-1"); got != "Día 1" {
		t.Fatalf("log at new project path: %q", got)
	}
	e.requireGone(t, "/api/v1/public/es/projects/otro/logs/dia-1")

	// Another locale of the project is independent.
	pen := e.Do("POST", "/api/v1/admin/contents/"+p+"/translations", map[string]any{"locale": "en"})
	if pen.Status != http.StatusCreated {
		t.Fatalf("create en: %d %s", pen.Status, pen.Body)
	}
	env := e.save(t, p, "en", snap("Rover", "rover"))
	e.publish(t, p, "en", env)

	deps := requireStatus(t, "withdraw project with logs", e.simple(t, p, "es", "withdraw"), http.StatusConflict, "has_published_logs")
	if deps.Fields["logs."+l] != "Día 1" {
		t.Fatalf("dependencies %+v", deps.Fields)
	}
	requireStatus(t, "withdraw log", e.simple(t, l, "es", "withdraw"), http.StatusOK, "")
	requireStatus(t, "withdraw project", e.simple(t, p, "es", "withdraw"), http.StatusOK, "")
	e.requireGone(t, "/api/v1/public/es/projects/rover-marte")
	if got := e.publicTitle(t, "/api/v1/public/en/projects/rover"); got != "Rover" {
		t.Fatalf("en project affected: %q", got)
	}

	// Visibility is the shared rule: a published log whose project stops being visible
	// (here archived out of band, which the API itself refuses) is not served.
	len2 := e.create(t, "log", "en", p)
	e.publish(t, len2, "en", e.save(t, len2, "en", snap("Day 1", "day-1")))
	if got := e.publicTitle(t, "/api/v1/public/en/projects/rover/logs/day-1"); got != "Day 1" {
		t.Fatalf("en log: %q", got)
	}
	if _, err := e.pool.Exec(context.Background(), `UPDATE contents SET archived_at = now() WHERE id = $1`, p); err != nil {
		t.Fatal(err)
	}
	e.requireGone(t, "/api/v1/public/en/projects/rover")
	e.requireGone(t, "/api/v1/public/en/projects/rover/logs/day-1")
}

func TestRoutesAreUniqueAndReservedPerScope(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	b := e.create(t, "article", "es", "")
	va := e.save(t, a, "es", snap("Sensor", "sensor"))
	vb := e.save(t, b, "es", snap("Sensor B", "sensor")) // drafts do not reserve routes

	e.publish(t, a, "es", va)
	requireStatus(t, "same slug", e.act(b, "es", "publish", map[string]any{"revision_version": vb, "expected_editorial_version": 0}), http.StatusConflict, "slug_taken")

	// Aliases stay reserved for their content.
	va2 := e.save(t, a, "es", snap("Sensor", "sensor-v2"))
	e.publish(t, a, "es", va2)
	requireStatus(t, "alias reserved", e.act(b, "es", "publish", map[string]any{"revision_version": vb, "expected_editorial_version": 0}), http.StatusConflict, "slug_taken")
	// Withdrawn content keeps its routes reserved too.
	requireStatus(t, "schedule on alias", e.schedule(t, b, "es", vb, "2026-10-01T09:30", false), http.StatusConflict, "slug_taken")
	requireStatus(t, "withdraw", e.simple(t, a, "es", "withdraw"), http.StatusOK, "")
	requireStatus(t, "reserved after withdraw", e.act(b, "es", "publish", map[string]any{"revision_version": vb, "expected_editorial_version": 0}), http.StatusConflict, "slug_taken")

	// The same slug is fine in another scope or another project's logs.
	p := e.create(t, "project", "es", "")
	e.publish(t, p, "es", e.save(t, p, "es", snap("Sensor", "sensor")))
	q := e.create(t, "project", "es", "")
	e.publish(t, q, "es", e.save(t, q, "es", snap("Otro", "otro")))
	for _, parent := range []string{p, q} {
		l := e.create(t, "log", "es", parent)
		e.publish(t, l, "es", e.save(t, l, "es", snap("Día 1", "dia-1")))
	}
	l := e.create(t, "log", "es", p)
	requireStatus(t, "log slug in the same project", e.act(l, "es", "publish",
		map[string]any{"revision_version": e.save(t, l, "es", snap("Día 1 bis", "dia-1")), "expected_editorial_version": 0}), http.StatusConflict, "slug_taken")
}

// Two contents race for the same free slug: exactly one wins, the other gets slug_taken.
func TestConcurrentPublicationsOfOneSlug(t *testing.T) {
	e := start(t)
	for round := range 5 {
		slug := fmt.Sprintf("carrera-%d", round)
		ids := make([]string, 4)
		for i := range ids {
			ids[i] = e.create(t, "article", "es", "")
			e.save(t, ids[i], "es", snap("Carrera", slug))
		}
		codes := make([]int, len(ids))
		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Go(func() {
				codes[i] = e.act(id, "es", "publish", map[string]any{"revision_version": 1, "expected_editorial_version": 0}).Status
			})
		}
		wg.Wait()
		won := 0
		for _, c := range codes {
			switch c {
			case http.StatusOK:
				won++
			case http.StatusConflict:
			default:
				t.Fatalf("round %d: unexpected status %d", round, c)
			}
		}
		if won != 1 || e.count(t, `SELECT count(*) FROM public_routes WHERE slug = $1`, slug) != 1 ||
			e.count(t, `SELECT count(*) FROM translations t JOIN revisions r ON r.id = t.published_revision_id WHERE r.slug = $1`, slug) != 1 {
			t.Fatalf("round %d: codes %v", round, codes)
		}
	}
}

func TestScheduleCancelAndReplace(t *testing.T) {
	e := start(t) // now = 2026-09-28 10:00 America/Mexico_City
	a := e.create(t, "article", "es", "")
	v1 := e.save(t, a, "es", snap("Plan", "plan"))
	v2 := e.save(t, a, "es", snap("Plan 2", "plan"))

	for name, c := range map[string]struct{ local, zone, code string }{
		"past":       {"2026-09-28T09:59", "America/Mexico_City", "validation_failed"},
		"now":        {"2026-09-28T10:00", "America/Mexico_City", "validation_failed"},
		"format":     {"2026-09-28 11:00", "America/Mexico_City", "validation_failed"},
		"seconds":    {"2026-09-28T11:00:00", "America/Mexico_City", "validation_failed"},
		"other zone": {"2026-09-28T11:00", "UTC", "validation_failed"},
		"impossible": {"2026-02-30T11:00", "America/Mexico_City", "validation_failed"},
	} {
		r := e.act(a, "es", "schedule", map[string]any{"revision_version": v1, "run_at_local": c.local, "time_zone": c.zone, "expected_editorial_version": 0})
		requireStatus(t, name, r, http.StatusUnprocessableEntity, c.code)
	}

	r := e.schedule(t, a, "es", v1, "2026-10-01T09:30", false)
	requireStatus(t, "schedule", r, http.StatusOK, "")
	st := decode[state](t, r)
	// Mexico City has no DST since 2022: UTC-6 all year.
	if st.Status != "unpublished" || st.EditorialVersion != 1 || st.ActiveJob == nil || st.ActiveJob.RunAt != "2026-10-01T15:30:00Z" ||
		st.ActiveJob.RunAtLocal != "2026-10-01T09:30" || st.ActiveJob.RevisionVersion != v1 || st.ActiveJob.Status != "scheduled" {
		t.Fatalf("scheduled state %+v %+v", st, st.ActiveJob)
	}
	e.requireGone(t, "/api/v1/public/es/articles/plan") // scheduling does not publish

	requireStatus(t, "second schedule", e.schedule(t, a, "es", v2, "2026-10-02T09:30", false), http.StatusConflict, "schedule_exists")
	r = e.schedule(t, a, "es", v2, "2026-10-02T09:30", true)
	requireStatus(t, "replace", r, http.StatusOK, "")
	if st := decode[state](t, r); st.ActiveJob.RevisionVersion != v2 || st.EditorialVersion != 2 {
		t.Fatalf("replaced %+v", st.ActiveJob)
	}
	if n := e.count(t, `SELECT count(*) FROM publication_jobs WHERE status = 'cancelled' AND cancel_reason = 'replaced'`); n != 1 {
		t.Fatalf("replaced jobs = %d", n)
	}

	// Archiving is rejected while scheduled.
	requireStatus(t, "archive scheduled", e.Do("POST", "/api/v1/admin/contents/"+a+"/archive", nil), http.StatusConflict, "scheduled_content")

	// Cancel does not publish or withdraw anything.
	requireStatus(t, "cancel", e.simple(t, a, "es", "cancel"), http.StatusOK, "")
	if st := e.state(t, a, "es"); st.ActiveJob != nil || st.Status != "unpublished" || st.EditorialVersion != 3 {
		t.Fatalf("after cancel %+v", st)
	}
	requireStatus(t, "cancel nothing", e.simple(t, a, "es", "cancel"), http.StatusConflict, "no_active_schedule")

	// A manual publication cancels the pending schedule in the same transaction.
	requireStatus(t, "schedule again", e.schedule(t, a, "es", v2, "2026-10-02T09:30", false), http.StatusOK, "")
	st = e.publish(t, a, "es", v1)
	if st.ActiveJob != nil || *st.PublishedVersion != v1 {
		t.Fatalf("manual publish kept the job %+v", st)
	}
	// Withdraw cancels a pending schedule as well (no resurrection later).
	requireStatus(t, "schedule v2", e.schedule(t, a, "es", v2, "2026-10-02T09:30", false), http.StatusOK, "")
	requireStatus(t, "withdraw", e.simple(t, a, "es", "withdraw"), http.StatusOK, "")
	if n := e.count(t, `SELECT count(*) FROM publication_jobs WHERE status = 'scheduled'`); n != 0 {
		t.Fatalf("active jobs after withdraw = %d", n)
	}
	reasons := e.count(t, `SELECT count(DISTINCT cancel_reason) FROM publication_jobs WHERE cancel_reason IN ('replaced','cancelled','manual_publish','withdrawn')`)
	if reasons != 4 {
		t.Fatalf("cancel reasons = %d, want 4", reasons)
	}

	// Job history, newest first.
	jobs := decode[struct {
		Items []struct {
			Status        string  `json:"status"`
			NextAttemptAt *string `json:"next_attempt_at"`
		} `json:"items"`
		Total int `json:"total"`
	}](t, e.Do("GET", pubPath(a, "es", "jobs")+"?page_size=2", nil))
	if jobs.Total != 4 || len(jobs.Items) != 2 || jobs.Items[0].Status != "cancelled" || jobs.Items[0].NextAttemptAt != nil {
		t.Fatalf("jobs %+v", jobs)
	}
	requireStatus(t, "bad page", e.Do("GET", pubPath(a, "es", "jobs")+"?page=0", nil), http.StatusUnprocessableEntity, "validation_failed")
}

func TestLogScheduleFollowsItsProjectSchedule(t *testing.T) {
	e := start(t)
	p := e.create(t, "project", "es", "")
	pv := e.save(t, p, "es", snap("Rover", "rover"))
	l := e.create(t, "log", "es", p)
	lv := e.save(t, l, "es", snap("Día 1", "dia-1"))

	blocked := requireStatus(t, "no parent schedule", e.schedule(t, l, "es", lv, "2026-10-01T10:00", false), http.StatusUnprocessableEntity, "publish_blocked")
	if blocked.Fields["project"] == "" {
		t.Fatalf("problems %+v", blocked.Fields)
	}
	requireStatus(t, "parent", e.schedule(t, p, "es", pv, "2026-10-01T10:00", false), http.StatusOK, "")
	requireStatus(t, "log before parent", e.schedule(t, l, "es", lv, "2026-10-01T09:59", false), http.StatusUnprocessableEntity, "publish_blocked")
	requireStatus(t, "log at parent time", e.schedule(t, l, "es", lv, "2026-10-01T10:00", false), http.StatusOK, "")
}

func TestWithdrawRevokesAnonymousMedia(t *testing.T) {
	e := start(t)
	img := e.upload(t, "text.png", "placa.png")
	e.setAsset(t, img, true, false)
	a := e.create(t, "article", "es", "")
	v := e.save(t, a, "es", snap("Placa", "placa", map[string]any{"type": "image", "attrs": map[string]any{"assetId": img, "alt": "Placa"}}))
	e.publish(t, a, "es", v)

	get := func(method string, headers ...string) *http.Response {
		req, _ := http.NewRequest(method, e.Origin+"/media/"+img, nil)
		for i := 0; i+1 < len(headers); i += 2 {
			req.Header.Set(headers[i], headers[i+1])
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp
	}
	resp := get("GET")
	etag := resp.Header.Get("ETag")
	if resp.StatusCode != 200 || etag == "" {
		t.Fatalf("published media: %d", resp.StatusCode)
	}
	if resp := get("GET", "If-None-Match", etag); resp.StatusCode != http.StatusNotModified {
		t.Fatalf("revalidation while published: %d", resp.StatusCode)
	}
	requireStatus(t, "withdraw", e.simple(t, a, "es", "withdraw"), http.StatusOK, "")
	for name, resp := range map[string]*http.Response{
		"GET":           get("GET"),
		"HEAD":          get("HEAD"),
		"Range":         get("GET", "Range", "bytes=0-10"),
		"If-None-Match": get("GET", "If-None-Match", etag),
	} {
		if resp.StatusCode != http.StatusNotFound || resp.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s after withdraw: %d %q", name, resp.StatusCode, resp.Header.Get("Cache-Control"))
		}
	}
}

func TestPublicationRequiresOwnerAndCSRF(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	v := e.save(t, a, "es", snap("Nota", "nota"))
	body := map[string]any{"revision_version": v, "expected_editorial_version": 0}
	for _, action := range []string{"publish", "schedule", "cancel", "withdraw"} {
		requireStatus(t, "anonymous "+action, e.Anonymous("POST", pubPath(a, "es", action), body), http.StatusUnauthorized, "")
		requireStatus(t, "no csrf "+action, e.WithoutCSRF("POST", pubPath(a, "es", action), body), http.StatusForbidden, "")
	}
	for _, path := range []string{pubPath(a, "es", ""), pubPath(a, "es", "jobs")} {
		requireStatus(t, "anonymous GET", e.Anonymous("GET", path, nil), http.StatusUnauthorized, "")
	}
	if st := e.state(t, a, "es"); st.EditorialVersion != 0 || st.Status != "unpublished" {
		t.Fatalf("rejected requests changed state %+v", st)
	}
}
