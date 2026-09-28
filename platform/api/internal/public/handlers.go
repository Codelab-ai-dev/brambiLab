package public

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/site"
)

const (
	defaultPageSize = 12
	maxPageSize     = 50
	// SitemapPageSize keeps each sitemap file far below the 50 000-URL limit.
	SitemapPageSize    = 5000
	homeLatestLogs     = 5
	homeLatestArticles = 3
	homeLatestProjects = 3
)

var (
	slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	kinds       = map[string]bool{"project": true, "article": true, "log": true}
)

// Handler serves GET /api/v1/public/... Responses are never cached (withdrawals are immediate).
type Handler struct {
	pool   *pgxpool.Pool
	site   *site.Store
	logger *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, st *site.Store, logger *slog.Logger) *Handler {
	return &Handler{pool: pool, site: st, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/public/{locale}/contents", h.listContents)
	mux.HandleFunc("GET /api/v1/public/{locale}/search", h.search)
	mux.HandleFunc("GET /api/v1/public/{locale}/taxonomy", h.taxonomy)
	mux.HandleFunc("GET /api/v1/public/{locale}/site", h.siteInfo)
	mux.HandleFunc("GET /api/v1/public/{locale}/home", h.home)
	mux.HandleFunc("GET /api/v1/public/sitemap", h.sitemap)
}

type page struct {
	Items    []Card `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Total    int    `json:"total"`
}

// params validates the shared query parameters. Unknown terms or projects match nothing (a
// stale shared URL shows an empty list), but malformed values are 422.
type params struct {
	locale  string
	filter  Filter
	page    int
	size    int
	nothing bool // a filter names something that does not exist publicly
}

func (h *Handler) params(w http.ResponseWriter, r *http.Request, allowProject bool) (params, bool) {
	w.Header().Set("Cache-Control", "no-store")
	p := params{locale: r.PathValue("locale"), page: 1, size: defaultPageSize}
	if p.locale != "es" && p.locale != "en" {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return p, false
	}
	q := r.URL.Query()
	fields := map[string]string{}
	for key := range q {
		switch key {
		case "kind", "category", "tag", "page", "page_size", "q":
		case "project":
			if !allowProject {
				fields[key] = "is not supported here"
			}
		default:
			fields[key] = "is not a known parameter"
		}
		if len(q[key]) > 1 {
			fields[key] = "must appear once"
		}
	}
	p.filter.Kind = q.Get("kind")
	if p.filter.Kind != "" && !kinds[p.filter.Kind] {
		fields["kind"] = "must be project, article or log"
	}
	for _, key := range []string{"category", "tag", "project"} {
		if v := q.Get(key); v != "" && (!slugPattern.MatchString(v) || len(v) > 120) {
			fields[key] = "must be a slug"
		}
	}
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 10000 {
			fields["page"] = "must be an integer between 1 and 10000"
		} else {
			p.page = n
		}
	}
	if v := q.Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > maxPageSize {
			fields["page_size"] = "must be between 1 and 50"
		} else {
			p.size = n
		}
	}
	if len(fields) > 0 {
		httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "validation_failed", Message: "Request is invalid", Fields: fields, RequestID: httpapi.RequestID(r.Context())})
		return p, false
	}
	p.filter.Locale, p.filter.Limit, p.filter.Offset = p.locale, p.size, (p.page-1)*p.size
	ctx := r.Context()
	resolve := func(sql, value string) (*string, error) {
		if value == "" {
			return nil, nil
		}
		var id string
		err := h.pool.QueryRow(ctx, sql, value, p.locale).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			p.nothing = true
			return nil, nil
		}
		return &id, err
	}
	var err error
	if p.filter.CategoryID, err = resolve(`SELECT id::text FROM categories WHERE slug = $1 AND $2 <> ''`, q.Get("category")); err == nil {
		if p.filter.TagID, err = resolve(`SELECT id::text FROM tags WHERE slug = $1 AND $2 <> ''`, q.Get("tag")); err == nil {
			// A project filter names the project by its current public slug in this locale.
			p.filter.ProjectID, err = resolve(`SELECT pr.content_id::text FROM public_routes pr
				JOIN visible_translations v ON v.content_id = pr.content_id AND v.locale = pr.locale
				WHERE pr.slug = $1 AND pr.locale = $2 AND pr.scope = 'project' AND pr.is_current`, q.Get("project"))
		}
	}
	if err != nil {
		h.fail(w, r, "resolve filters", err)
		return p, false
	}
	return p, true
}

func (h *Handler) listContents(w http.ResponseWriter, r *http.Request) {
	p, ok := h.params(w, r, true)
	if !ok {
		return
	}
	if r.URL.Query().Has("q") {
		httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "validation_failed", Message: "Request is invalid",
			Fields: map[string]string{"q": "use /search for text queries"}, RequestID: httpapi.RequestID(r.Context())})
		return
	}
	out := page{Items: []Card{}, Page: p.page, PageSize: p.size}
	if !p.nothing {
		items, total, err := cards(r.Context(), h.pool, p.filter)
		if err != nil {
			h.fail(w, r, "list public contents", err)
			return
		}
		out.Items, out.Total = items, total
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

type searchPage struct {
	page
	Query    string `json:"query"`
	HasTerms bool   `json:"has_terms"`
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	p, ok := h.params(w, r, false)
	if !ok {
		return
	}
	text, fits := normalizeQuery(r.URL.Query().Get("q"))
	if !fits {
		httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "validation_failed", Message: "Request is invalid",
			Fields: map[string]string{"q": "at most 200 characters"}, RequestID: httpapi.RequestID(r.Context())})
		return
	}
	out := searchPage{page: page{Items: []Card{}, Page: p.page, PageSize: p.size}, Query: text}
	items, total, hasTerms, err := search(r.Context(), h.pool, text, p.filter)
	if err != nil {
		h.fail(w, r, "search", err)
		return
	}
	out.HasTerms = hasTerms
	if !p.nothing {
		out.Items, out.Total = items, total
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) taxonomy(w http.ResponseWriter, r *http.Request) {
	p, ok := h.params(w, r, false)
	if !ok {
		return
	}
	out := map[string][]Term{}
	for key, sql := range map[string]string{
		"categories": `SELECT c.slug, CASE $1 WHEN 'en' THEN c.label_en ELSE c.label_es END, count(*)
			FROM visible_translations v JOIN revisions r ON r.id = v.revision_id JOIN categories c ON c.id = r.category_id
			WHERE v.locale = $1 AND ($2 = '' OR v.kind = $2) GROUP BY c.id ORDER BY 2, 1`,
		"tags": `SELECT tg.slug, CASE $1 WHEN 'en' THEN tg.label_en ELSE tg.label_es END, count(*)
			FROM visible_translations v JOIN revision_tags rt ON rt.revision_id = v.revision_id JOIN tags tg ON tg.id = rt.tag_id
			WHERE v.locale = $1 AND ($2 = '' OR v.kind = $2) GROUP BY tg.id ORDER BY 2, 1`,
	} {
		rows, err := h.pool.Query(r.Context(), sql, p.locale, p.filter.Kind)
		if err != nil {
			h.fail(w, r, "taxonomy", err)
			return
		}
		terms, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Term, error) {
			var t Term
			return t, row.Scan(&t.Slug, &t.Label, &t.Count)
		})
		if err != nil {
			h.fail(w, r, "taxonomy", err)
			return
		}
		out[key] = terms
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) locale(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	l := r.PathValue("locale")
	if l != "es" && l != "en" {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return "", false
	}
	return l, true
}

func (h *Handler) siteInfo(w http.ResponseWriter, r *http.Request) {
	l, ok := h.locale(w, r)
	if !ok {
		return
	}
	info, err := h.site.Public(r.Context(), l)
	if err != nil {
		h.fail(w, r, "site info", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, info)
}

type home struct {
	Site           site.Public `json:"site"`
	Featured       []Card      `json:"featured"`
	LatestProjects []Card      `json:"latest_projects"`
	LatestLogs     []Card      `json:"latest_logs"`
	LatestArticles []Card      `json:"latest_articles"`
}

// home gathers the start page. Featured projects appear only while visible in this locale;
// latest_projects lets the page show real projects when nothing is featured.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	l, ok := h.locale(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var out home
	var err error
	if out.Site, err = h.site.Public(ctx, l); err != nil {
		h.fail(w, r, "home", err)
		return
	}
	ids, err := h.site.FeaturedIDs(ctx)
	if err != nil {
		h.fail(w, r, "home", err)
		return
	}
	out.Featured = []Card{}
	if len(ids) > 0 {
		if out.Featured, _, err = cards(ctx, h.pool, Filter{Locale: l, Kind: "project", ContentIDs: ids, Limit: len(ids)}); err != nil {
			h.fail(w, r, "home", err)
			return
		}
	}
	for _, part := range []struct {
		dst   *[]Card
		kind  string
		limit int
	}{{&out.LatestProjects, "project", homeLatestProjects}, {&out.LatestLogs, "log", homeLatestLogs}, {&out.LatestArticles, "article", homeLatestArticles}} {
		if *part.dst, _, err = cards(ctx, h.pool, Filter{Locale: l, Kind: part.kind, Limit: part.limit}); err != nil {
			h.fail(w, r, "home", err)
			return
		}
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// SitemapEntry is one canonical public page; alternates lists the locales of the same content.
type SitemapEntry struct {
	Kind        string             `json:"kind"`
	Locale      string             `json:"locale"`
	Slug        string             `json:"slug"`
	ProjectSlug *string            `json:"project_slug"`
	LastMod     time.Time          `json:"lastmod"`
	Alternates  []SitemapAlternate `json:"alternates"`
}

// SitemapAlternate is the other visible translation of the same content, by its current slugs.
type SitemapAlternate struct {
	Locale      string  `json:"locale"`
	Slug        string  `json:"slug"`
	ProjectSlug *string `json:"project_slug"`
}

func (h *Handler) sitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	pg := 1
	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 10000 {
			httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "validation_failed", Message: "Request is invalid",
				Fields: map[string]string{"page": "must be an integer between 1 and 10000"}, RequestID: httpapi.RequestID(r.Context())})
			return
		}
		pg = n
	}
	entries, total, err := sitemapEntries(r.Context(), h.pool, SitemapPageSize, (pg-1)*SitemapPageSize)
	if err != nil {
		h.fail(w, r, "sitemap", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": entries, "page": pg, "page_size": SitemapPageSize, "total": total})
}

func sitemapEntries(ctx context.Context, q querier, limit, offset int) ([]SitemapEntry, int, error) {
	rows, err := q.Query(ctx, `
		SELECT v.kind, v.locale, r.slug, pr.slug, v.published_at,
		       COALESCE((SELECT json_agg(json_build_object('locale', o.locale, 'slug', orr.slug, 'project_slug', opr.slug) ORDER BY o.locale)
		                 FROM visible_translations o
		                 JOIN revisions orr ON orr.id = o.revision_id
		                 LEFT JOIN visible_translations opv ON opv.content_id = o.project_id AND opv.locale = o.locale
		                 LEFT JOIN revisions opr ON opr.id = opv.revision_id
		                 WHERE o.content_id = v.content_id AND o.locale <> v.locale), '[]'::json),
		       count(*) OVER ()
		FROM visible_translations v
		JOIN revisions r ON r.id = v.revision_id
		LEFT JOIN visible_translations pv ON pv.content_id = v.project_id AND pv.locale = v.locale
		LEFT JOIN revisions pr ON pr.id = pv.revision_id
		ORDER BY v.kind, v.content_id, v.locale LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []SitemapEntry{}
	total := 0
	for rows.Next() {
		var e SitemapEntry
		var alts []byte
		if err := rows.Scan(&e.Kind, &e.Locale, &e.Slug, &e.ProjectSlug, &e.LastMod, &alts, &total); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(alts, &e.Alternates); err != nil {
			return nil, 0, err
		}
		e.LastMod = e.LastMod.UTC()
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.Error(op, "request_id", httpapi.RequestID(r.Context()), "error", err)
	httpapi.WriteError(w, r, http.StatusInternalServerError, "internal", "Internal server error")
}
