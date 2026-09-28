package public

import (
	"context"
	"strings"
	"unicode/utf8"
)

// MaxQuery bounds the search text (characters).
const MaxQuery = 200

// Snippet markers: control characters stripped from the indexed text first, so they can only come
// from ts_headline and user text can never forge a highlight.
const (
	hitStart = "\x01"
	hitStop  = "\x02"
)

func config(locale string) string {
	if locale == "en" {
		return "bl_en"
	}
	return "bl_es"
}

// normalizeQuery trims and collapses whitespace; ok is false when the text is too long.
func normalizeQuery(q string) (string, bool) {
	q = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, q)), " ")
	return q, utf8.RuneCountInString(q) <= MaxQuery
}

// search returns ranked cards with snippets. hasTerms is false when the text has no searchable
// lexemes (empty, only punctuation or stop words); callers then show a hint, not "no results".
func search(ctx context.Context, q querier, text string, f Filter) (items []Card, total int, hasTerms bool, err error) {
	cfg := config(f.Locale)
	if text == "" {
		return []Card{}, 0, false, nil
	}
	var nodes int
	if err := q.QueryRow(ctx, `SELECT numnode(websearch_to_tsquery($1::regconfig, $2))`, cfg, text).Scan(&nodes); err != nil {
		return nil, 0, false, err
	}
	if nodes == 0 {
		return []Card{}, 0, false, nil
	}
	rows, err := q.Query(ctx, `
		WITH matching AS (
			SELECT v.translation_id, v.content_id, v.kind, v.locale, v.project_id, v.revision_id, v.published_at, t.first_published_at,
			       ts_rank_cd(s.search, websearch_to_tsquery($1::regconfig, $2)) AS rank
			FROM search_documents s
			-- Current visibility, and the projection must match the published revision.
			JOIN visible_translations v ON v.translation_id = s.translation_id AND v.revision_id = s.revision_id
			JOIN translations t ON t.id = v.translation_id
			JOIN revisions r ON r.id = v.revision_id
			-- Inline tsquery (not a CTE) so the planner sees it and uses the GIN index.
			WHERE s.search @@ websearch_to_tsquery($1::regconfig, $2) AND v.locale = $3
			  AND ($4 = '' OR v.kind = $4)
			  AND ($5::uuid IS NULL OR r.category_id = $5)
			  AND ($6::uuid IS NULL OR EXISTS (SELECT 1 FROM revision_tags rt WHERE rt.revision_id = r.id AND rt.tag_id = $6))
		), base AS (
			SELECT *, count(*) OVER () AS total FROM matching
			ORDER BY rank DESC, first_published_at DESC, content_id DESC LIMIT $7 OFFSET $8
		)`+strings.Replace(cardSelect, "SELECT b.content_id::text,", `SELECT b.total,
		       ts_headline($1::regconfig, regexp_replace(r.plain_text, '[\x01\x02]', '', 'g'), websearch_to_tsquery($1::regconfig, $2),
		                   'StartSel=' || chr(1) || ', StopSel=' || chr(2) || ', MaxWords=30, MinWords=12, MaxFragments=2, FragmentDelimiter=" … "'),
		       b.content_id::text,`, 1)+`
		ORDER BY b.rank DESC, b.first_published_at DESC, b.content_id DESC`,
		cfg, text, f.Locale, f.Kind, f.CategoryID, f.TagID, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, true, err
	}
	defer rows.Close()
	items = []Card{}
	for rows.Next() {
		var headline string
		c, err := scanCard(prefixed2{rows, &total, &headline})
		if err != nil {
			return nil, 0, true, err
		}
		c.Snippet = segments(headline)
		items = append(items, c)
	}
	return items, total, true, rows.Err()
}

// segments splits a ts_headline result into plain-text pieces.
func segments(headline string) []Segment {
	var out []Segment
	hit := false
	for len(headline) > 0 {
		marker := hitStart
		if hit {
			marker = hitStop
		}
		i := strings.Index(headline, marker)
		if i < 0 {
			i = len(headline)
		}
		if text := strings.NewReplacer(hitStart, "", hitStop, "").Replace(headline[:i]); text != "" {
			out = append(out, Segment{Text: text, Hit: hit})
		}
		if i == len(headline) {
			break
		}
		headline = headline[i+1:]
		hit = !hit
	}
	return out
}

type prefixed2 struct {
	row    interface{ Scan(...any) error }
	first  *int
	second *string
}

func (p prefixed2) Scan(dest ...any) error {
	return p.row.Scan(append([]any{p.first, p.second}, dest...)...)
}
