// Package public serves the anonymous read side of the site: listings, search, taxonomy, home,
// site settings and sitemap data (web-v1.md §9.1, WEB-006). Every query joins
// visible_translations and reads the published revision only; nothing here takes a session.
package public

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Card is a listing entry: no body, only what a card shows (OpenAPI PublicCard).
type Card struct {
	Kind             string        `json:"kind"`
	Slug             string        `json:"slug"`
	Title            string        `json:"title"`
	Summary          string        `json:"summary"`
	PublishedAt      time.Time     `json:"published_at"`
	FirstPublishedAt time.Time     `json:"first_published_at"`
	Category         *Term         `json:"category"`
	Tags             []Term        `json:"tags"`
	Project          *Parent       `json:"project"`
	ProjectFields    *ProjectFacts `json:"project_fields"`
	Cover            *Cover        `json:"cover"`
	Snippet          []Segment     `json:"snippet,omitempty"`
	contentID        string
}

type Term struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
	Count int    `json:"count,omitempty"`
}

type Parent struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// ProjectFacts is the technical status and technologies of a project card.
type ProjectFacts struct {
	Status       string   `json:"status,omitempty"`
	Technologies []string `json:"technologies,omitempty"`
}

// Cover is present only while the image is publicly servable.
type Cover struct {
	AssetID string `json:"asset_id"`
	Width   *int   `json:"width"`
	Height  *int   `json:"height"`
	Alt     string `json:"alt"`
}

// Segment is a piece of a search snippet; Hit marks matched terms. Plain text, never HTML.
type Segment struct {
	Text string `json:"text"`
	Hit  bool   `json:"hit,omitempty"`
}

type Filter struct {
	Locale     string
	Kind       string
	CategoryID *string
	TagID      *string
	ProjectID  *string
	ContentIDs []string // explicit set (featured), kept in the given order
	Limit      int
	Offset     int
}

// cardSelect returns the columns of a card for rows of "base" (a CTE with translation_id,
// content_id, kind, locale, project_id, revision_id, published_at, first_published_at).
// One query per page: taxonomy, parent and cover are joined, never fetched per row.
const cardSelect = `
	SELECT b.content_id::text, b.kind, r.slug, r.title, r.summary, b.published_at, b.first_published_at,
	       c.slug, CASE b.locale WHEN 'en' THEN c.label_en ELSE c.label_es END,
	       COALESCE((SELECT json_agg(json_build_object('slug', tg.slug, 'label', CASE b.locale WHEN 'en' THEN tg.label_en ELSE tg.label_es END) ORDER BY tg.slug)
	                 FROM revision_tags rt JOIN tags tg ON tg.id = rt.tag_id WHERE rt.revision_id = r.id), '[]'::json),
	       pr.slug, pr.title,
	       r.project_fields->>'status', r.project_fields->'technologies',
	       a.id::text, a.width, a.height, COALESCE(at.alt, '')
	FROM base b
	JOIN revisions r ON r.id = b.revision_id
	LEFT JOIN categories c ON c.id = r.category_id
	LEFT JOIN visible_translations pv ON pv.content_id = b.project_id AND pv.locale = b.locale
	LEFT JOIN revisions pr ON pr.id = pv.revision_id
	LEFT JOIN assets a ON a.id = r.cover_asset_id AND a.status = 'ready' AND a.public_enabled AND a.kind = 'image'
	LEFT JOIN asset_translations at ON at.asset_id = a.id AND at.locale = b.locale`

func scanCard(row pgx.Row) (Card, error) {
	var c Card
	var catSlug, catLabel, parentSlug, parentTitle, status, coverID *string
	var tags, techs []byte
	var width, height *int
	var alt string
	if err := row.Scan(&c.contentID, &c.Kind, &c.Slug, &c.Title, &c.Summary, &c.PublishedAt, &c.FirstPublishedAt,
		&catSlug, &catLabel, &tags, &parentSlug, &parentTitle, &status, &techs, &coverID, &width, &height, &alt); err != nil {
		return c, err
	}
	c.PublishedAt, c.FirstPublishedAt = c.PublishedAt.UTC(), c.FirstPublishedAt.UTC()
	if catSlug != nil {
		c.Category = &Term{Slug: *catSlug, Label: *catLabel}
	}
	if err := json.Unmarshal(tags, &c.Tags); err != nil {
		return c, err
	}
	if parentSlug != nil {
		c.Project = &Parent{Slug: *parentSlug, Title: *parentTitle}
	}
	if c.Kind == "project" {
		facts := ProjectFacts{}
		if status != nil {
			facts.Status = *status
		}
		if len(techs) > 0 {
			_ = json.Unmarshal(techs, &facts.Technologies) // validated on save; ignore odd legacy shapes
		}
		if facts.Status != "" || len(facts.Technologies) > 0 {
			c.ProjectFields = &facts
		}
	}
	if coverID != nil {
		c.Cover = &Cover{AssetID: *coverID, Width: width, Height: height, Alt: alt}
	}
	return c, nil
}

// cards returns one page of visible translations matching f, and the total count.
func cards(ctx context.Context, q querier, f Filter) ([]Card, int, error) {
	args := []any{f.Locale, f.Kind, f.CategoryID, f.TagID, f.ProjectID, f.ContentIDs, f.Limit, f.Offset}
	order := `b.first_published_at DESC, b.content_id DESC`
	if f.ContentIDs != nil {
		order = `array_position($6::uuid[], b.content_id)`
	}
	rows, err := q.Query(ctx, `
		WITH matching AS (
			SELECT v.translation_id, v.content_id, v.kind, v.locale, v.project_id, v.revision_id, v.published_at, t.first_published_at
			FROM visible_translations v
			JOIN translations t ON t.id = v.translation_id
			JOIN revisions r ON r.id = v.revision_id
			WHERE v.locale = $1
			  AND ($2 = '' OR v.kind = $2)
			  AND ($3::uuid IS NULL OR r.category_id = $3)
			  AND ($4::uuid IS NULL OR EXISTS (SELECT 1 FROM revision_tags rt WHERE rt.revision_id = r.id AND rt.tag_id = $4))
			  AND ($5::uuid IS NULL OR v.project_id = $5)
			  AND ($6::uuid[] IS NULL OR v.content_id = ANY ($6))
		), base AS (
			SELECT *, count(*) OVER () AS total FROM matching b ORDER BY `+order+` LIMIT $7 OFFSET $8
		)`+strings.Replace(cardSelect, "SELECT b.content_id::text,", "SELECT b.total, b.content_id::text,", 1)+`
		ORDER BY `+order, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Card{}
	total := 0
	for rows.Next() {
		var c Card
		c, total, err = scanCardWithTotal(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && f.Offset > 0 {
		// Past the last page: report the real total so the client can offer a way back.
		if err := q.QueryRow(ctx, `SELECT count(*) FROM visible_translations v JOIN revisions r ON r.id = v.revision_id
			WHERE v.locale = $1 AND ($2 = '' OR v.kind = $2) AND ($3::uuid IS NULL OR r.category_id = $3)
			  AND ($4::uuid IS NULL OR EXISTS (SELECT 1 FROM revision_tags rt WHERE rt.revision_id = r.id AND rt.tag_id = $4))
			  AND ($5::uuid IS NULL OR v.project_id = $5) AND ($6::uuid[] IS NULL OR v.content_id = ANY ($6))`,
			args[:6]...).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// scanCardWithTotal reads a card row prefixed by its window total.
func scanCardWithTotal(rows pgx.Rows) (Card, int, error) {
	var total int
	c, err := scanCard(prefixed{rows, &total})
	return c, total, err
}

// prefixed scans a leading column into first, then the card columns.
type prefixed struct {
	row   pgx.Row
	first *int
}

func (p prefixed) Scan(dest ...any) error { return p.row.Scan(append([]any{p.first}, dest...)...) }

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}
