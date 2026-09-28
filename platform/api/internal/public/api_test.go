package public_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/authtest"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/content"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/media"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/public"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/publishing"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/site"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

type env struct {
	*authtest.Server
	pool *pgxpool.Pool
	svc  *publishing.Service
	now  atomic.Pointer[time.Time]
}

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
	e.setNow(time.Date(2026, 9, 28, 16, 0, 0, 0, time.UTC))
	e.svc.Now = func() time.Time { return *e.now.Load() }
	st := site.NewStore(pool)
	e.Server = authtest.Start(t, pool, func(a *auth.Handler, logger *slog.Logger) []httpapi.Module {
		return []httpapi.Module{
			content.NewHandler(content.NewStore(pool), logger, a.RequireOwner, auth.Actor),
			media.NewHandler(media.NewStore(pool), local, lim, logger, a.RequireOwner, auth.Actor, a.IsOwner),
			publishing.NewHandler(e.svc, logger, a.RequireOwner, auth.Actor),
			site.NewHandler(st, logger, a.RequireOwner, auth.Actor),
			public.NewHandler(pool, st, logger),
		}
	})
	return e
}

func (e *env) setNow(t time.Time) { e.now.Store(&t) }
func (e *env) tick()              { e.setNow(e.now.Load().Add(time.Minute)) }

func decode[T any](t *testing.T, r authtest.Response) T {
	t.Helper()
	var v T
	r.JSON(t, &v)
	return v
}

func must(t *testing.T, what string, r authtest.Response, status int) authtest.Response {
	t.Helper()
	if r.Status != status {
		t.Fatalf("%s: %d %s, want %d", what, r.Status, r.Body, status)
	}
	return r
}

func (e *env) term(t *testing.T, kind, slug, es, en string) string {
	t.Helper()
	r := must(t, "term", e.Do("POST", "/api/v1/admin/"+kind, map[string]any{"slug": slug, "labels": map[string]string{"es": es, "en": en}}), http.StatusCreated)
	return decode[struct {
		ID string `json:"id"`
	}](t, r).ID
}

func (e *env) create(t *testing.T, kind, locale, projectID string) string {
	t.Helper()
	body := map[string]any{"kind": kind, "locale": locale}
	if projectID != "" {
		body["project_id"] = projectID
	}
	return decode[struct {
		ID string `json:"id"`
	}](t, must(t, "create", e.Do("POST", "/api/v1/admin/contents", body), http.StatusCreated)).ID
}

func (e *env) translate(t *testing.T, id, locale string) {
	t.Helper()
	must(t, "translate", e.Do("POST", "/api/v1/admin/contents/"+id+"/translations", map[string]any{"locale": locale}), http.StatusCreated)
}

type snap map[string]any

func doc(text string, extra ...any) map[string]any {
	nodes := []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text}}}}
	return map[string]any{"type": "doc", "content": append(nodes, extra...)}
}

func article(title, slug, body string) snap {
	return snap{"title": title, "slug": slug, "summary": "Resumen de " + title, "body": doc(body)}
}

func (e *env) save(t *testing.T, id, locale string, s snap) int {
	t.Helper()
	path := fmt.Sprintf("/api/v1/admin/contents/%s/translations/%s", id, locale)
	latest := decode[struct {
		LatestVersion int `json:"latest_version"`
	}](t, e.Do("GET", path, nil)).LatestVersion
	must(t, "save", e.Do("POST", path+"/revisions", map[string]any{"expected_version": latest, "kind": "manual", "snapshot": s}), http.StatusCreated)
	return latest + 1
}

func (e *env) editorial(t *testing.T, id, locale string) int {
	t.Helper()
	return decode[struct {
		V int `json:"editorial_version"`
	}](t, e.Do("GET", fmt.Sprintf("/api/v1/admin/contents/%s/translations/%s/publication", id, locale), nil)).V
}

func (e *env) publish(t *testing.T, id, locale string, version int) {
	t.Helper()
	e.tick()
	must(t, "publish", e.Do("POST", fmt.Sprintf("/api/v1/admin/contents/%s/translations/%s/publication/publish", id, locale),
		map[string]any{"revision_version": version, "expected_editorial_version": e.editorial(t, id, locale)}), http.StatusOK)
}

func (e *env) withdraw(t *testing.T, id, locale string) {
	t.Helper()
	must(t, "withdraw", e.Do("POST", fmt.Sprintf("/api/v1/admin/contents/%s/translations/%s/publication/withdraw", id, locale),
		map[string]any{"expected_editorial_version": e.editorial(t, id, locale)}), http.StatusOK)
}

// get fetches a public URL anonymously, without following redirects, and checks no-store.
func (e *env) get(t *testing.T, path string) authtest.Response {
	t.Helper()
	c := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(e.Origin + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("GET %s: Cache-Control %q", path, cc)
	}
	return authtest.Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
}

type card struct {
	Kind     string `json:"kind"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Category *struct {
		Slug  string `json:"slug"`
		Label string `json:"label"`
	} `json:"category"`
	Tags []struct {
		Slug  string `json:"slug"`
		Label string `json:"label"`
	} `json:"tags"`
	Project *struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
	} `json:"project"`
	ProjectFields *struct {
		Status       string   `json:"status"`
		Technologies []string `json:"technologies"`
	} `json:"project_fields"`
	Cover *struct {
		AssetID string `json:"asset_id"`
		Width   *int   `json:"width"`
	} `json:"cover"`
	Snippet []struct {
		Text string `json:"text"`
		Hit  bool   `json:"hit"`
	} `json:"snippet"`
	Body json.RawMessage `json:"body"`
}

type listing struct {
	Items    []card `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Total    int    `json:"total"`
	HasTerms bool   `json:"has_terms"`
}

func (e *env) list(t *testing.T, path string) listing {
	t.Helper()
	return decode[listing](t, must(t, path, e.get(t, path), http.StatusOK))
}

func slugs(l listing) string {
	s := make([]string, len(l.Items))
	for i, c := range l.Items {
		s[i] = c.Slug
	}
	return strings.Join(s, ",")
}

func (e *env) upload(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "media", "testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	r := must(t, "upload", e.Do("POST", "/api/v1/admin/assets?filename="+file, data), http.StatusCreated)
	return decode[struct {
		ID string `json:"id"`
	}](t, r).ID
}

func (e *env) setAsset(t *testing.T, id string, public, downloadable bool) {
	t.Helper()
	must(t, "patch asset", e.Do("PATCH", "/api/v1/admin/assets/"+id, map[string]any{"public_enabled": public, "downloadable": downloadable}), http.StatusOK)
}

// --- tests ----------------------------------------------------------------------------------------

func TestListingsFiltersOrderAndPagination(t *testing.T) {
	e := start(t)
	hw := e.term(t, "categories", "hardware", "Hardware", "Hardware")
	sw := e.term(t, "categories", "software", "Software", "Software")
	motor := e.term(t, "tags", "motores", "Motores", "Motors")
	esp := e.term(t, "tags", "esp32", "ESP32", "ESP32")

	// a1..a5 published in order; filters: hardware = a1,a3,a5; motores = a1,a2,a3; both = a1,a3.
	ids := map[string]string{}
	for i, spec := range []struct {
		cat  string
		tags []string
	}{{hw, []string{motor}}, {sw, []string{motor, esp}}, {hw, []string{motor}}, {sw, nil}, {hw, []string{esp}}} {
		slug := fmt.Sprintf("a%d", i+1)
		id := e.create(t, "article", "es", "")
		s := article("Artículo "+slug, slug, "Texto")
		s["category_id"] = spec.cat
		s["tag_ids"] = append([]string{}, spec.tags...)
		e.publish(t, id, "es", e.save(t, id, "es", s))
		ids[slug] = id
	}
	if got := slugs(e.list(t, "/api/v1/public/es/contents?kind=article")); got != "a5,a4,a3,a2,a1" {
		t.Fatalf("order %s", got)
	}
	if got := slugs(e.list(t, "/api/v1/public/es/contents?kind=article&category=hardware")); got != "a5,a3,a1" {
		t.Fatalf("category %s", got)
	}
	if got := slugs(e.list(t, "/api/v1/public/es/contents?category=hardware&tag=motores")); got != "a3,a1" {
		t.Fatalf("category AND tag %s", got)
	}
	l := e.list(t, "/api/v1/public/es/contents?tag=motores")
	if slugs(l) != "a3,a2,a1" || l.Items[0].Category.Label != "Hardware" || len(l.Items[1].Tags) != 2 || l.Items[0].Body != nil {
		t.Fatalf("tag listing %+v", l)
	}
	// Stable pagination: pages partition the list without repeats.
	p1, p2, p3 := e.list(t, "/api/v1/public/es/contents?page_size=2"), e.list(t, "/api/v1/public/es/contents?page_size=2&page=2"), e.list(t, "/api/v1/public/es/contents?page_size=2&page=3")
	if slugs(p1)+","+slugs(p2)+","+slugs(p3) != "a5,a4,a3,a2,a1" || p1.Total != 5 || p3.Total != 5 {
		t.Fatalf("pages %s | %s | %s", slugs(p1), slugs(p2), slugs(p3))
	}
	if past := e.list(t, "/api/v1/public/es/contents?page_size=2&page=9"); len(past.Items) != 0 || past.Total != 5 {
		t.Fatalf("past the end %+v", past)
	}
	// Unknown (but well-formed) filters match nothing; malformed ones are rejected.
	if l := e.list(t, "/api/v1/public/es/contents?category=nada"); len(l.Items) != 0 || l.Total != 0 {
		t.Fatalf("unknown category %+v", l)
	}
	for _, q := range []string{"kind=page", "category=No%20Slug", "page=0", "page_size=51", "page=abc", "sort=title", "kind=article&kind=log", "q=rover"} {
		must(t, q, e.get(t, "/api/v1/public/es/contents?"+q), http.StatusUnprocessableEntity)
	}
	must(t, "bad locale", e.get(t, "/api/v1/public/fr/contents"), http.StatusNotFound)
	if l := e.list(t, "/api/v1/public/en/contents"); len(l.Items) != 0 {
		t.Fatalf("es content listed in en: %s", slugs(l))
	}

	// A newer draft (other title, category and tag) changes nothing public.
	d := article("Borrador secreto", "a1", "Texto")
	d["category_id"] = sw
	d["tag_ids"] = []string{}
	e.save(t, ids["a1"], "es", d)
	if got := slugs(e.list(t, "/api/v1/public/es/contents?category=hardware&tag=motores")); got != "a3,a1" {
		t.Fatalf("draft changed filters: %s", got)
	}
	if body := string(e.get(t, "/api/v1/public/es/contents").Body); strings.Contains(body, "secreto") {
		t.Fatalf("draft title leaked: %s", body)
	}
	// Withdrawal removes the card; republishing does not move it to the top (first_published_at).
	e.withdraw(t, ids["a1"], "es")
	if got := slugs(e.list(t, "/api/v1/public/es/contents")); got != "a5,a4,a3,a2" {
		t.Fatalf("after withdraw %s", got)
	}
	e.publish(t, ids["a2"], "es", 1)
	e.publish(t, ids["a2"], "es", e.save(t, ids["a2"], "es", article("Artículo a2 v2", "a2", "Texto")))
	if got := slugs(e.list(t, "/api/v1/public/es/contents")); got != "a5,a4,a3,a2" {
		t.Fatalf("republish reordered %s", got)
	}
}

func TestProjectCardsLogsAndCovers(t *testing.T) {
	e := start(t)
	img := e.upload(t, "text.png")
	p := e.create(t, "project", "es", "")
	s := snap{"title": "Rover", "slug": "rover", "summary": "Robot", "body": doc("Texto"), "cover_asset_id": img,
		"project_fields": map[string]any{"status": "in_development", "technologies": []string{"ESP32", "Go"}}}
	e.setAsset(t, img, true, false)
	e.publish(t, p, "es", e.save(t, p, "es", s))
	for i := 1; i <= 3; i++ {
		l := e.create(t, "log", "es", p)
		e.publish(t, l, "es", e.save(t, l, "es", article(fmt.Sprintf("Día %d", i), fmt.Sprintf("dia-%d", i), "Avance")))
	}
	pc := e.list(t, "/api/v1/public/es/contents?kind=project").Items[0]
	if pc.ProjectFields == nil || pc.ProjectFields.Status != "in_development" || strings.Join(pc.ProjectFields.Technologies, ",") != "ESP32,Go" ||
		pc.Cover == nil || pc.Cover.AssetID != img || pc.Cover.Width == nil {
		t.Fatalf("project card %+v", pc)
	}
	logs := e.list(t, "/api/v1/public/es/contents?kind=log&project=rover&page_size=2")
	if slugs(logs) != "dia-3,dia-2" || logs.Total != 3 || logs.Items[0].Project.Slug != "rover" {
		t.Fatalf("logs %+v", logs)
	}
	if l := e.list(t, "/api/v1/public/es/contents?project=otro"); len(l.Items) != 0 {
		t.Fatalf("unknown project %+v", l)
	}
	must(t, "project filter on search", e.get(t, "/api/v1/public/es/search?q=rover&project=rover"), http.StatusUnprocessableEntity)

	// Revoking the cover's public permission removes it from cards and detail at once.
	e.setAsset(t, img, false, false)
	if c := e.list(t, "/api/v1/public/es/contents?kind=project").Items[0]; c.Cover != nil {
		t.Fatalf("private cover still listed %+v", c.Cover)
	}
	detail := decode[struct {
		CoverAssetID *string        `json:"cover_asset_id"`
		Cover        any            `json:"cover"`
		Assets       map[string]any `json:"assets"`
	}](t, e.get(t, "/api/v1/public/es/projects/rover"))
	if detail.CoverAssetID != nil || detail.Cover != nil || len(detail.Assets) != 0 {
		t.Fatalf("private cover in detail %+v", detail)
	}
}

func TestDetailTaxonomyAlternatesAndAssets(t *testing.T) {
	e := start(t)
	cat := e.term(t, "categories", "robotica", "Robótica", "Robotics")
	tag := e.term(t, "tags", "motores", "Motores", "Motors")
	img := e.upload(t, "text.png")
	pdf := e.upload(t, "doc.pdf")
	e.setAsset(t, img, true, false)
	e.setAsset(t, pdf, true, true)
	p := e.create(t, "project", "es", "")
	e.translate(t, p, "en")
	body := doc("Texto", map[string]any{"type": "image", "attrs": map[string]any{"assetId": img, "alt": "Placa"}},
		map[string]any{"type": "download", "attrs": map[string]any{"assetId": pdf, "label": "Ficha"}})
	es := snap{"title": "Rover", "slug": "rover-solar", "body": body, "category_id": cat, "tag_ids": []string{tag}}
	e.publish(t, p, "es", e.save(t, p, "es", es))

	type detail struct {
		Category   *struct{ Label string }  `json:"category"`
		Tags       []struct{ Label string } `json:"tags"`
		Alternates []struct {
			Locale, Kind, Slug string
			ProjectSlug        *string `json:"project_slug"`
		} `json:"alternates"`
		Assets map[string]struct {
			Kind         string
			Downloadable bool
			Name         string
			Bytes        int64
		} `json:"assets"`
		FirstPublishedAt string `json:"first_published_at"`
	}
	d := decode[detail](t, e.get(t, "/api/v1/public/es/projects/rover-solar"))
	if d.Category.Label != "Robótica" || d.Tags[0].Label != "Motores" || len(d.Alternates) != 0 || len(d.Assets) != 2 ||
		!d.Assets[pdf].Downloadable || d.Assets[img].Kind != "image" || d.Assets[pdf].Name != "doc.pdf" || d.FirstPublishedAt == "" {
		t.Fatalf("es detail %+v", d)
	}
	// English published with its own slug: each side points at the other's current slug.
	en := snap{"title": "Rover", "slug": "solar-rover", "body": doc("Text"), "category_id": cat, "tag_ids": []string{tag}}
	e.publish(t, p, "en", e.save(t, p, "en", en))
	d = decode[detail](t, e.get(t, "/api/v1/public/es/projects/rover-solar"))
	if len(d.Alternates) != 1 || d.Alternates[0].Locale != "en" || d.Alternates[0].Slug != "solar-rover" || d.Alternates[0].Kind != "project" {
		t.Fatalf("alternates %+v", d.Alternates)
	}
	if den := decode[detail](t, e.get(t, "/api/v1/public/en/projects/solar-rover")); den.Category.Label != "Robotics" || den.Alternates[0].Slug != "rover-solar" {
		t.Fatalf("en detail %+v", den)
	}
	// A log's alternate carries its project slug in the other locale.
	l := e.create(t, "log", "es", p)
	e.translate(t, l, "en")
	e.publish(t, l, "es", e.save(t, l, "es", article("Día 1", "dia-1", "Avance")))
	e.publish(t, l, "en", e.save(t, l, "en", article("Day 1", "day-1", "Progress")))
	dl := decode[detail](t, e.get(t, "/api/v1/public/es/projects/rover-solar/logs/dia-1"))
	if len(dl.Alternates) != 1 || dl.Alternates[0].Slug != "day-1" || dl.Alternates[0].ProjectSlug == nil || *dl.Alternates[0].ProjectSlug != "solar-rover" {
		t.Fatalf("log alternates %+v", dl.Alternates)
	}
	// Withdrawing English removes the alternate; revoking a download removes it from assets.
	e.withdraw(t, l, "en")
	e.setAsset(t, pdf, false, true)
	d = decode[detail](t, e.get(t, "/api/v1/public/es/projects/rover-solar"))
	if _, ok := d.Assets[pdf]; ok || len(d.Assets) != 1 {
		t.Fatalf("revoked asset listed %+v", d.Assets)
	}
	if dl := decode[detail](t, e.get(t, "/api/v1/public/es/projects/rover-solar/logs/dia-1")); len(dl.Alternates) != 0 {
		t.Fatalf("withdrawn alternate %+v", dl.Alternates)
	}
}

func TestTaxonomyCountsOnlyPublishedRevisions(t *testing.T) {
	e := start(t)
	hw := e.term(t, "categories", "hardware", "Hardware", "Hardware")
	e.term(t, "categories", "vacia", "Vacía", "Empty")
	secret := e.term(t, "tags", "secreto", "Secreto", "Secret")
	motor := e.term(t, "tags", "motores", "Motores", "Motors")
	a := e.create(t, "article", "es", "")
	s := article("Uno", "uno", "Texto")
	s["category_id"], s["tag_ids"] = hw, []string{motor}
	e.publish(t, a, "es", e.save(t, a, "es", s))
	// A draft uses the "secreto" tag: it must not appear anywhere public.
	d := article("Uno", "uno", "Texto")
	d["tag_ids"] = []string{secret, motor}
	e.save(t, a, "es", d)

	type tax struct {
		Categories []struct {
			Slug, Label string
			Count       int
		}
		Tags []struct {
			Slug  string
			Count int
		}
	}
	tx := decode[tax](t, e.get(t, "/api/v1/public/es/taxonomy"))
	if len(tx.Categories) != 1 || tx.Categories[0].Slug != "hardware" || tx.Categories[0].Count != 1 || len(tx.Tags) != 1 || tx.Tags[0].Slug != "motores" {
		t.Fatalf("taxonomy %+v", tx)
	}
	if tx := decode[tax](t, e.get(t, "/api/v1/public/es/taxonomy?kind=project")); len(tx.Categories)+len(tx.Tags) != 0 {
		t.Fatalf("project taxonomy %+v", tx)
	}
	if tx := decode[tax](t, e.get(t, "/api/v1/public/en/taxonomy")); len(tx.Categories)+len(tx.Tags) != 0 {
		t.Fatalf("en taxonomy %+v", tx)
	}
	e.withdraw(t, a, "es")
	if tx := decode[tax](t, e.get(t, "/api/v1/public/es/taxonomy")); len(tx.Categories)+len(tx.Tags) != 0 {
		t.Fatalf("taxonomy after withdraw %+v", tx)
	}
}

func (e *env) search(t *testing.T, locale, q string, extra ...string) listing {
	t.Helper()
	path := "/api/v1/public/" + locale + "/search?q=" + url.QueryEscape(q)
	for _, x := range extra {
		path += "&" + x
	}
	return e.list(t, path)
}

func TestSearchLanguagesAccentsAndVisibility(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	e.translate(t, a, "en")
	e.publish(t, a, "es", e.save(t, a, "es", article("Energía solar del rover", "energia", "Las baterías se cargan con paneles.")))
	e.publish(t, a, "en", e.save(t, a, "en", article("Rover solar power", "power", "Batteries charge from panels.")))
	b := e.create(t, "article", "es", "")
	e.publish(t, b, "es", e.save(t, b, "es", article("Motores", "motores", "El rover usa energía de dos baterías.")))

	// Accents, case and punctuation are tolerated; stemming matches plural/singular.
	for _, q := range []string{"energia", "ENERGÍA!!!", "  energía ", "batería", "\"energía solar\""} {
		if l := e.search(t, "es", q); !l.HasTerms || l.Total == 0 {
			t.Fatalf("q=%q: %+v", q, l)
		}
	}
	// Accents are ignored also in hyphenated tokens with digits.
	m := e.create(t, "article", "es", "")
	e.publish(t, m, "es", e.save(t, m, "es", article("Módulo", "modulo", "Prueba de telemetría-x1 con el ESP32-C3.")))
	for _, q := range []string{"telemetria-x1", "telemetría-x1", "esp32-c3", "telemetria"} {
		if l := e.search(t, "es", q); slugs(l) != "modulo" {
			t.Fatalf("q=%q: %s", q, slugs(l))
		}
	}
	e.withdraw(t, m, "es")

	// Title matches rank above body matches, also when paging (the newer body match is not first).
	if l := e.search(t, "es", "energía"); slugs(l) != "energia,motores" {
		t.Fatalf("ranking %s", slugs(l))
	}
	if l := e.search(t, "es", "energía", "page_size=1"); slugs(l) != "energia" || l.Total != 2 {
		t.Fatalf("ranked first page %s", slugs(l))
	}
	// Each locale searches its own text only.
	if l := e.search(t, "en", "energía"); l.Total != 0 {
		t.Fatalf("es text found in en: %s", slugs(l))
	}
	if l := e.search(t, "en", "batteries"); slugs(l) != "power" {
		t.Fatalf("en search %s", slugs(l))
	}
	// No searchable terms: an explicit state, not "no results".
	for _, q := range []string{"", "   ", "?!¡.", "el de la", "-"} {
		if l := e.search(t, "es", q); l.HasTerms || len(l.Items) != 0 {
			t.Fatalf("q=%q should have no terms: %+v", q, l)
		}
	}
	must(t, "too long", e.get(t, "/api/v1/public/es/search?q="+strings.Repeat("a", 201)), http.StatusUnprocessableEntity)
	if l := e.search(t, "es", strings.Repeat("á", 200)); l.HasTerms != true {
		t.Fatalf("200 characters are allowed: %+v", l)
	}

	// Combined with a kind filter.
	if l := e.search(t, "es", "rover", "kind=project"); l.Total != 0 {
		t.Fatalf("kind filter %s", slugs(l))
	}

	// Drafts never reach the index; publishing replaces the indexed text; withdrawing removes it.
	e.save(t, b, "es", article("Motores", "motores", "Texto confidencial del borrador."))
	if l := e.search(t, "es", "confidencial"); l.Total != 0 {
		t.Fatalf("draft searchable %+v", l)
	}
	e.publish(t, b, "es", 2)
	if l := e.search(t, "es", "confidencial"); slugs(l) != "motores" {
		t.Fatalf("republished text %+v", l)
	}
	if l := e.search(t, "es", "energía"); slugs(l) != "energia" {
		t.Fatalf("old text still indexed: %s", slugs(l))
	}
	e.withdraw(t, b, "es")
	if l := e.search(t, "es", "confidencial"); l.Total != 0 {
		t.Fatalf("withdrawn searchable %+v", l)
	}
	if n := count(t, e.pool, `SELECT count(*) FROM search_documents`); n != 2 {
		t.Fatalf("projection rows = %d, want 2", n)
	}
	// The projection is only an index: current visibility decides. Archived out of band (the API
	// refuses it while published), the article disappears although its row is still indexed.
	if _, err := e.pool.Exec(context.Background(), `UPDATE contents SET archived_at = now() WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}
	if l := e.search(t, "es", "energía solar"); l.Total != 0 {
		t.Fatalf("archived content searchable: %s", slugs(l))
	}
	if _, err := e.pool.Exec(context.Background(), `UPDATE contents SET archived_at = NULL WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}

	// Scheduled publication updates the index when it runs.
	c := e.create(t, "article", "es", "")
	v := e.save(t, c, "es", article("Telemetría", "telemetria", "Datos por radio."))
	must(t, "schedule", e.Do("POST", fmt.Sprintf("/api/v1/admin/contents/%s/translations/es/publication/schedule", c),
		map[string]any{"revision_version": v, "run_at_local": "2026-10-01T09:30", "time_zone": "America/Mexico_City", "expected_editorial_version": 0}), http.StatusOK)
	if l := e.search(t, "es", "telemetría"); l.Total != 0 {
		t.Fatalf("scheduled but not run: %+v", l)
	}
	e.setNow(time.Date(2026, 10, 1, 15, 30, 0, 0, time.UTC))
	if _, err := e.svc.RunDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if l := e.search(t, "es", "telemetría"); slugs(l) != "telemetria" {
		t.Fatalf("scheduled publication not indexed %+v", l)
	}
}

func TestSearchSnippetsArePlainText(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	source := `<script>alert("x")</script> <b>negrita</b> el sensor mide energía & voltaje <img src=x onerror=alert(1)>`
	e.publish(t, a, "es", e.save(t, a, "es", article("Notas", "notas", source)))
	l := e.search(t, "es", "energía")
	if len(l.Items) != 1 || len(l.Items[0].Snippet) == 0 {
		t.Fatalf("results %+v", l)
	}
	hits := []string{}
	for _, seg := range l.Items[0].Snippet {
		// Every word comes from the source (ts_headline may drop tag-like tokens, never add any)
		// and no marker survives. The web renders segments as text.
		if strings.ContainsAny(seg.Text, "\x01\x02") {
			t.Fatalf("segment %q keeps a marker", seg.Text)
		}
		for _, w := range strings.Fields(seg.Text) {
			if !strings.Contains(source, w) {
				t.Fatalf("segment word %q is not in the source", w)
			}
		}
		if seg.Hit {
			hits = append(hits, seg.Text)
		}
	}
	if strings.Join(hits, "|") != "energía" {
		t.Fatalf("hits %v", hits)
	}
	// A marker character typed by the author cannot forge a highlight.
	b := e.create(t, "article", "es", "")
	e.publish(t, b, "es", e.save(t, b, "es", article("Marcas", "marcas", "falso\x01resaltado\x02 y energía real")))
	for _, seg := range e.search(t, "es", "energía real").Items[0].Snippet {
		if seg.Hit && seg.Text != "energía" && seg.Text != "real" {
			t.Fatalf("forged hit %q", seg.Text)
		}
	}
}

func count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type settings struct {
	Version          int `json:"version"`
	FeaturedProjects []struct {
		ContentID string          `json:"content_id"`
		Visible   map[string]bool `json:"visible"`
	} `json:"featured_projects"`
}

func validSite(version int, featured ...string) map[string]any {
	return map[string]any{
		"expected_version":     version,
		"intro":                map[string]string{"es": "Laboratorio personal.", "en": "Personal lab."},
		"bio":                  map[string]string{"es": "Biografía.\nSegundo párrafo.", "en": "Bio."},
		"contact_email":        "hola@example.com",
		"links":                []map[string]string{{"kind": "github", "label": "GitHub", "url": "https://github.com/example"}},
		"featured_project_ids": append([]string{}, featured...),
	}
}

func TestSiteSettingsValidationVersionAndAccess(t *testing.T) {
	e := start(t)
	st := decode[settings](t, must(t, "get", e.Do("GET", "/api/v1/admin/site", nil), http.StatusOK))
	if st.Version != 0 || len(st.FeaturedProjects) != 0 {
		t.Fatalf("initial %+v", st)
	}
	// Honest defaults: nothing invented.
	pub := decode[map[string]any](t, e.get(t, "/api/v1/public/es/site"))
	if pub["intro"] != "" || pub["bio"] != "" || pub["contact_email"] != "" || len(pub["links"].([]any)) != 0 {
		t.Fatalf("default public site %+v", pub)
	}

	article := e.create(t, "article", "es", "")
	archived := e.create(t, "project", "es", "")
	must(t, "archive", e.Do("POST", "/api/v1/admin/contents/"+archived+"/archive", nil), http.StatusOK)
	bad := func(name string, mutate func(map[string]any)) {
		t.Helper()
		b := validSite(0)
		mutate(b)
		must(t, name, e.Do("PUT", "/api/v1/admin/site", b), http.StatusUnprocessableEntity)
	}
	bad("http link", func(b map[string]any) {
		b["links"] = []map[string]string{{"kind": "github", "label": "G", "url": "http://github.com/x"}}
	})
	bad("javascript link", func(b map[string]any) {
		b["links"] = []map[string]string{{"kind": "other", "label": "X", "url": "javascript:alert(1)"}}
	})
	bad("credentials", func(b map[string]any) {
		b["links"] = []map[string]string{{"kind": "other", "label": "X", "url": "https://u:p@example.com"}}
	})
	bad("relative", func(b map[string]any) {
		b["links"] = []map[string]string{{"kind": "other", "label": "X", "url": "/admin"}}
	})
	bad("unknown kind", func(b map[string]any) {
		b["links"] = []map[string]string{{"kind": "mastodon", "label": "X", "url": "https://example.com"}}
	})
	bad("empty label", func(b map[string]any) {
		b["links"] = []map[string]string{{"kind": "other", "label": " ", "url": "https://example.com"}}
	})
	bad("too many links", func(b map[string]any) {
		links := []map[string]string{}
		for range site.MaxLinks + 1 {
			links = append(links, map[string]string{"kind": "other", "label": "X", "url": "https://example.com"})
		}
		b["links"] = links
	})
	bad("email with name", func(b map[string]any) { b["contact_email"] = "Gus <g@example.com>" })
	bad("not an email", func(b map[string]any) { b["contact_email"] = "nope" })
	bad("long intro", func(b map[string]any) { b["intro"] = map[string]string{"es": strings.Repeat("a", site.MaxIntro+1)} })
	bad("featured article", func(b map[string]any) { b["featured_project_ids"] = []string{article} })
	bad("featured archived", func(b map[string]any) { b["featured_project_ids"] = []string{archived} })
	bad("featured repeated", func(b map[string]any) {
		p := e.create(t, "project", "es", "")
		b["featured_project_ids"] = []string{p, p}
	})
	bad("unknown field", func(b map[string]any) { b["resend_api_key"] = "x" })
	bad("missing version", func(b map[string]any) { delete(b, "expected_version") })
	if st := decode[settings](t, e.Do("GET", "/api/v1/admin/site", nil)); st.Version != 0 {
		t.Fatalf("rejected saves changed the version: %+v", st)
	}

	must(t, "anonymous", e.Anonymous("GET", "/api/v1/admin/site", nil), http.StatusUnauthorized)
	must(t, "anonymous put", e.Anonymous("PUT", "/api/v1/admin/site", validSite(0)), http.StatusUnauthorized)
	must(t, "no csrf", e.WithoutCSRF("PUT", "/api/v1/admin/site", validSite(0)), http.StatusForbidden)

	r := must(t, "save", e.Do("PUT", "/api/v1/admin/site", validSite(0)), http.StatusOK)
	if decode[settings](t, r).Version != 1 {
		t.Fatalf("saved %s", r.Body)
	}
	conflict := decode[struct {
		Code           string `json:"code"`
		CurrentVersion int    `json:"current_version"`
	}](t, must(t, "stale", e.Do("PUT", "/api/v1/admin/site", validSite(0)), http.StatusConflict))
	if conflict.Code != "version_conflict" || conflict.CurrentVersion != 1 {
		t.Fatalf("conflict %+v", conflict)
	}
	pub = decode[map[string]any](t, e.get(t, "/api/v1/public/en/site"))
	if pub["intro"] != "Personal lab." || pub["bio"] != "Bio." || pub["contact_email"] != "hola@example.com" || len(pub) != 4 {
		t.Fatalf("public en site %+v", pub)
	}
	if n := count(t, e.pool, `SELECT count(*) FROM audit_events WHERE action = 'site.update'`); n != 1 {
		t.Fatalf("audit events %d", n)
	}
}

func TestHomeFeaturedOnlyWhileVisible(t *testing.T) {
	e := start(t)
	p1 := e.create(t, "project", "es", "")
	p2 := e.create(t, "project", "es", "")
	p3 := e.create(t, "project", "es", "") // featured but never published
	p4 := e.create(t, "project", "es", "") // published but not featured
	e.publish(t, p1, "es", e.save(t, p1, "es", article("Uno", "uno", "x")))
	e.publish(t, p2, "es", e.save(t, p2, "es", article("Dos", "dos", "x")))
	e.save(t, p3, "es", article("Tres secreto", "tres", "x"))
	e.publish(t, p4, "es", e.save(t, p4, "es", article("Cuatro", "cuatro", "x")))
	for i := range 6 {
		a := e.create(t, "article", "es", "")
		e.publish(t, a, "es", e.save(t, a, "es", article(fmt.Sprintf("Art %d", i), fmt.Sprintf("art-%d", i), "x")))
	}
	l := e.create(t, "log", "es", p1)
	e.publish(t, l, "es", e.save(t, l, "es", article("Día 1", "dia-1", "x")))

	must(t, "save", e.Do("PUT", "/api/v1/admin/site", validSite(0, p2, p3, p1)), http.StatusOK)
	type homeResp struct {
		Site struct {
			Intro string `json:"intro"`
		} `json:"site"`
		Featured       []card `json:"featured"`
		LatestProjects []card `json:"latest_projects"`
		LatestLogs     []card `json:"latest_logs"`
		LatestArticles []card `json:"latest_articles"`
	}
	h := decode[homeResp](t, e.get(t, "/api/v1/public/es/home"))
	names := func(cs []card) string { return slugs(listing{Items: cs}) }
	if h.Site.Intro != "Laboratorio personal." || names(h.Featured) != "dos,uno" || names(h.LatestArticles) != "art-5,art-4,art-3" ||
		names(h.LatestLogs) != "dia-1" || names(h.LatestProjects) != "cuatro,dos,uno" {
		t.Fatalf("home %+v", h)
	}
	if strings.Contains(string(e.get(t, "/api/v1/public/es/home").Body), "secreto") {
		t.Fatal("unpublished featured project leaked")
	}
	// The panel shows which selections are visible.
	st := decode[settings](t, e.Do("GET", "/api/v1/admin/site", nil))
	if len(st.FeaturedProjects) != 3 || st.FeaturedProjects[1].ContentID != p3 || st.FeaturedProjects[1].Visible["es"] || !st.FeaturedProjects[0].Visible["es"] {
		t.Fatalf("admin featured %+v", st.FeaturedProjects)
	}
	// Withdrawing a featured project removes it from the home at once; the selection stays.
	e.withdraw(t, p2, "es")
	if h := decode[homeResp](t, e.get(t, "/api/v1/public/es/home")); names(h.Featured) != "uno" {
		t.Fatalf("after withdraw %s", names(h.Featured))
	}
	if st := decode[settings](t, e.Do("GET", "/api/v1/admin/site", nil)); len(st.FeaturedProjects) != 3 {
		t.Fatalf("selection changed %+v", st)
	}
	// Another locale shows only its own visible projects.
	if h := decode[homeResp](t, e.get(t, "/api/v1/public/en/home")); len(h.Featured)+len(h.LatestProjects)+len(h.LatestArticles) != 0 {
		t.Fatalf("en home %+v", h)
	}
}

func TestSitemapListsCanonicalVisiblePagesOnly(t *testing.T) {
	e := start(t)
	p := e.create(t, "project", "es", "")
	e.translate(t, p, "en")
	e.publish(t, p, "es", e.save(t, p, "es", article("Rover", "rover", "x")))
	e.publish(t, p, "en", e.save(t, p, "en", article("Rover", "rover-en", "x")))
	e.publish(t, p, "es", e.save(t, p, "es", article("Rover", "rover-marte", "x"))) // "rover" becomes an alias
	l := e.create(t, "log", "es", p)
	e.publish(t, l, "es", e.save(t, l, "es", article("Día", "dia", "x")))
	gone := e.create(t, "article", "es", "")
	e.publish(t, gone, "es", e.save(t, gone, "es", article("Retirado", "retirado", "x")))
	e.withdraw(t, gone, "es")
	draft := e.create(t, "article", "es", "")
	e.save(t, draft, "es", article("Borrador", "borrador", "x"))

	type entry struct {
		Kind, Locale, Slug string
		ProjectSlug        *string `json:"project_slug"`
		LastMod            string  `json:"lastmod"`
		Alternates         []struct {
			Locale, Slug string
			ProjectSlug  *string `json:"project_slug"`
		} `json:"alternates"`
	}
	sm := decode[struct {
		Items []entry `json:"items"`
		Total int     `json:"total"`
	}](t, e.get(t, "/api/v1/public/sitemap"))
	got := []string{}
	for _, it := range sm.Items {
		s := it.Locale + ":" + it.Kind + ":" + it.Slug
		if it.ProjectSlug != nil {
			s += "@" + *it.ProjectSlug
		}
		alts := []string{}
		for _, a := range it.Alternates {
			alts = append(alts, a.Locale+"="+a.Slug)
		}
		got = append(got, s+":"+strings.Join(alts, "+"))
		if it.LastMod == "" {
			t.Fatalf("no lastmod %+v", it)
		}
	}
	want := map[string]bool{"es:project:rover-marte:en=rover-en": true, "en:project:rover-en:es=rover-marte": true, "es:log:dia@rover-marte:": true}
	if len(got) != 3 || sm.Total != 3 {
		t.Fatalf("sitemap %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("unexpected entry %q in %v", g, got)
		}
	}
	must(t, "bad page", e.get(t, "/api/v1/public/sitemap?page=0"), http.StatusUnprocessableEntity)
}
