package publishing

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

// PublicContent is the published snapshot served to anonymous readers (OpenAPI PublicContent).
type PublicContent struct {
	Kind          string          `json:"kind"`
	Locale        string          `json:"locale"`
	Slug          string          `json:"slug"`
	Title         string          `json:"title"`
	Summary       string          `json:"summary"`
	Body          json.RawMessage `json:"body"`
	SEO           json.RawMessage `json:"seo"`
	ProjectFields json.RawMessage `json:"project_fields,omitempty"`
	// CoverAssetID and Cover are set only while the cover image is publicly servable.
	CoverAssetID     *string                `json:"cover_asset_id"`
	Cover            *PublicCover           `json:"cover"`
	PublishedAt      time.Time              `json:"published_at"`
	FirstPublishedAt time.Time              `json:"first_published_at"`
	Project          *PublicParent          `json:"project,omitempty"`
	Category         *PublicTerm            `json:"category"`
	Tags             []PublicTerm           `json:"tags"`
	Alternates       []Alternate            `json:"alternates"`
	Assets           map[string]PublicAsset `json:"assets"`
}

type PublicTerm struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

type PublicCover struct {
	AssetID string `json:"asset_id"`
	Width   *int   `json:"width"`
	Height  *int   `json:"height"`
	Alt     string `json:"alt"`
}

// Alternate is the other visible translation of the same content, by its current slugs.
type Alternate struct {
	Locale      string  `json:"locale"`
	Kind        string  `json:"kind"`
	Slug        string  `json:"slug"`
	ProjectSlug *string `json:"project_slug"`
}

// PublicAsset describes a referenced file that anonymous visitors can fetch now. Files that
// lost their permissions are absent, and the page shows them as unavailable.
type PublicAsset struct {
	Kind         string `json:"kind"`
	MIME         string `json:"mime"`
	Width        *int   `json:"width"`
	Height       *int   `json:"height"`
	Bytes        int64  `json:"bytes"`
	Downloadable bool   `json:"downloadable"`
	Name         string `json:"name"`
}

type PublicParent struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type route struct {
	contentID string
	current   bool
}

// resolveRoute finds the content owning (locale, scope, slug), current route or alias.
func resolveRoute(ctx context.Context, q querier, locale, scope, slug string) (route, bool, error) {
	var rt route
	if !slugPattern.MatchString(slug) || len(slug) > 120 {
		return rt, false, nil
	}
	err := q.QueryRow(ctx, `SELECT content_id, is_current FROM public_routes WHERE locale = $1 AND scope = $2 AND slug = $3`, locale, scope, slug).
		Scan(&rt.contentID, &rt.current)
	if errors.Is(err, pgx.ErrNoRows) {
		return rt, false, nil
	}
	return rt, err == nil, err
}

// publicContent loads a visible translation (nil when not visible: withdrawn, archived, or a log
// whose project is not visible).
func publicContent(ctx context.Context, q querier, contentID, locale string) (*PublicContent, error) {
	var pc PublicContent
	var revisionID string
	var parentID *string
	var catSlug, catLabel *string
	err := q.QueryRow(ctx, `
		SELECT v.kind, v.locale, r.slug, r.title, r.summary, r.body_json, r.seo, r.project_fields, v.published_at, t.first_published_at,
		       v.project_id::text, r.id::text, c.slug, CASE v.locale WHEN 'en' THEN c.label_en ELSE c.label_es END
		FROM visible_translations v
		JOIN translations t ON t.id = v.translation_id
		JOIN revisions r ON r.id = v.revision_id
		LEFT JOIN categories c ON c.id = r.category_id
		WHERE v.content_id = $1 AND v.locale = $2`, contentID, locale).
		Scan(&pc.Kind, &pc.Locale, &pc.Slug, &pc.Title, &pc.Summary, &pc.Body, &pc.SEO, &pc.ProjectFields, &pc.PublishedAt, &pc.FirstPublishedAt,
			&parentID, &revisionID, &catSlug, &catLabel)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pc.PublishedAt, pc.FirstPublishedAt = pc.PublishedAt.UTC(), pc.FirstPublishedAt.UTC()
	if len(pc.ProjectFields) == 0 || string(pc.ProjectFields) == "null" {
		pc.ProjectFields = nil
	}
	if catSlug != nil {
		pc.Category = &PublicTerm{Slug: *catSlug, Label: *catLabel}
	}
	if err := publicExtras(ctx, q, &pc, contentID, revisionID); err != nil {
		return nil, err
	}
	if parentID != nil {
		var parent PublicParent
		if err := q.QueryRow(ctx, `
			SELECT pr.slug, r.title FROM visible_translations v
			JOIN revisions r ON r.id = v.revision_id
			JOIN public_routes pr ON pr.content_id = v.content_id AND pr.locale = v.locale AND pr.is_current
			WHERE v.content_id = $1 AND v.locale = $2`, *parentID, locale).Scan(&parent.Slug, &parent.Title); err != nil {
			return nil, err
		}
		pc.Project = &parent
	}
	return &pc, nil
}

// publicExtras adds tags, alternates, the servable assets and the public cover.
func publicExtras(ctx context.Context, q querier, pc *PublicContent, contentID, revisionID string) error {
	rows, err := q.Query(ctx, `SELECT tg.slug, CASE $2 WHEN 'en' THEN tg.label_en ELSE tg.label_es END
		FROM revision_tags rt JOIN tags tg ON tg.id = rt.tag_id WHERE rt.revision_id = $1 ORDER BY tg.slug`, revisionID, pc.Locale)
	if err != nil {
		return err
	}
	if pc.Tags, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (PublicTerm, error) {
		var t PublicTerm
		return t, r.Scan(&t.Slug, &t.Label)
	}); err != nil {
		return err
	}
	rows, err = q.Query(ctx, `
		SELECT o.locale, o.kind, r.slug, pr.slug
		FROM visible_translations o
		JOIN revisions r ON r.id = o.revision_id
		LEFT JOIN visible_translations pv ON pv.content_id = o.project_id AND pv.locale = o.locale
		LEFT JOIN revisions pr ON pr.id = pv.revision_id
		WHERE o.content_id = $1 AND o.locale <> $2 ORDER BY o.locale`, contentID, pc.Locale)
	if err != nil {
		return err
	}
	if pc.Alternates, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Alternate, error) {
		var a Alternate
		return a, r.Scan(&a.Locale, &a.Kind, &a.Slug, &a.ProjectSlug)
	}); err != nil {
		return err
	}
	// Same rule as anonymous delivery (internal/media): ready and public; downloads also downloadable.
	rows, err = q.Query(ctx, `
		SELECT a.id::text, a.kind, a.mime, a.width, a.height, a.bytes, a.downloadable, a.original_name,
		       bool_or(ra.usage = 'cover'), COALESCE(max(at.alt), '')
		FROM revision_assets ra
		JOIN assets a ON a.id = ra.asset_id
		LEFT JOIN asset_translations at ON at.asset_id = a.id AND at.locale = $2
		WHERE ra.revision_id = $1 AND a.status = 'ready' AND a.public_enabled
		GROUP BY a.id ORDER BY a.id`, revisionID, pc.Locale)
	if err != nil {
		return err
	}
	defer rows.Close()
	pc.Assets = map[string]PublicAsset{}
	for rows.Next() {
		var id, alt string
		var a PublicAsset
		var cover bool
		if err := rows.Scan(&id, &a.Kind, &a.MIME, &a.Width, &a.Height, &a.Bytes, &a.Downloadable, &a.Name, &cover, &alt); err != nil {
			return err
		}
		pc.Assets[id] = a
		if cover && a.Kind == "image" {
			pc.CoverAssetID = &id
			pc.Cover = &PublicCover{AssetID: id, Width: a.Width, Height: a.Height, Alt: alt}
		}
	}
	return rows.Err()
}

func (h *Handler) publicProject(w http.ResponseWriter, r *http.Request) {
	h.servePublic(w, r, func(ctx context.Context, locale string) (route, bool, error) {
		return resolveRoute(ctx, h.svc.pool, locale, "project", r.PathValue("slug"))
	})
}

func (h *Handler) publicArticle(w http.ResponseWriter, r *http.Request) {
	h.servePublic(w, r, func(ctx context.Context, locale string) (route, bool, error) {
		return resolveRoute(ctx, h.svc.pool, locale, "article", r.PathValue("slug"))
	})
}

// publicLog resolves the project by its slug (current or alias), then the log inside the
// project's identity scope. Either being an alias makes the whole path an alias.
func (h *Handler) publicLog(w http.ResponseWriter, r *http.Request) {
	h.servePublic(w, r, func(ctx context.Context, locale string) (route, bool, error) {
		project, ok, err := resolveRoute(ctx, h.svc.pool, locale, "project", r.PathValue("projectSlug"))
		if err != nil || !ok {
			return route{}, false, err
		}
		log, ok, err := resolveRoute(ctx, h.svc.pool, locale, "log:"+project.contentID, r.PathValue("slug"))
		if err != nil || !ok {
			return route{}, false, err
		}
		log.current = log.current && project.current
		return log, true, nil
	})
}

// servePublic answers 200 for a current route, 301 (one hop) for an alias, and the same 404 for
// everything not visible. Never cached: publication state changes at any moment.
func (h *Handler) servePublic(w http.ResponseWriter, r *http.Request, resolve func(context.Context, string) (route, bool, error)) {
	w.Header().Set("Cache-Control", "no-store")
	locale := r.PathValue("locale")
	notFound := func() { httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found") }
	if !validLocale(locale) {
		notFound()
		return
	}
	ctx := r.Context()
	rt, ok, err := resolve(ctx, locale)
	if err != nil {
		h.serviceError(w, r, "resolve public route", err)
		return
	}
	if !ok {
		notFound()
		return
	}
	pc, err := publicContent(ctx, h.svc.pool, rt.contentID, locale)
	if err != nil {
		h.serviceError(w, r, "load public content", err)
		return
	}
	if pc == nil {
		notFound()
		return
	}
	if !rt.current {
		target, err := currentRoute(ctx, h.svc.pool, rt.contentID, locale)
		if err != nil {
			h.serviceError(w, r, "resolve current route", err)
			return
		}
		if target == nil {
			notFound()
			return
		}
		w.Header().Set("Location", *target)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, pc)
}
