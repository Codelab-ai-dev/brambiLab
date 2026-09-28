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
	CoverAssetID  *string         `json:"cover_asset_id"`
	PublishedAt   time.Time       `json:"published_at"`
	Project       *PublicParent   `json:"project,omitempty"`
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
	var projectID *string
	err := q.QueryRow(ctx, `
		SELECT v.kind, v.locale, r.slug, r.title, r.summary, r.body_json, r.seo, r.project_fields, r.cover_asset_id::text, v.published_at, v.project_id::text
		FROM visible_translations v JOIN revisions r ON r.id = v.revision_id
		WHERE v.content_id = $1 AND v.locale = $2`, contentID, locale).
		Scan(&pc.Kind, &pc.Locale, &pc.Slug, &pc.Title, &pc.Summary, &pc.Body, &pc.SEO, &pc.ProjectFields, &pc.CoverAssetID, &pc.PublishedAt, &projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	pc.PublishedAt = pc.PublishedAt.UTC()
	if len(pc.ProjectFields) == 0 || string(pc.ProjectFields) == "null" {
		pc.ProjectFields = nil
	}
	if projectID != nil {
		var parent PublicParent
		if err := q.QueryRow(ctx, `
			SELECT pr.slug, r.title FROM visible_translations v
			JOIN revisions r ON r.id = v.revision_id
			JOIN public_routes pr ON pr.content_id = v.content_id AND pr.locale = v.locale AND pr.is_current
			WHERE v.content_id = $1 AND v.locale = $2`, *projectID, locale).Scan(&parent.Slug, &parent.Title); err != nil {
			return nil, err
		}
		pc.Project = &parent
	}
	return &pc, nil
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
