// Package site stores the editorial site settings: presentation, bio, contact links and featured
// projects (web-v1.md §9.1, WEB-006). Saving is explicit and immediate, guarded by a version.
package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

const (
	MaxIntro    = 500
	MaxBio      = 5000
	MaxLinks    = 8
	MaxFeatured = 6
	maxLabel    = 60
	maxURL      = 500
	maxBody     = 64 << 10
)

var (
	linkKinds = map[string]bool{"github": true, "linkedin": true, "website": true, "other": true}
	idPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type Link struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Localized struct {
	ES string `json:"es"`
	EN string `json:"en"`
}

func (l Localized) In(locale string) string {
	if locale == "en" {
		return l.EN
	}
	return l.ES
}

// Settings is the owner's view (OpenAPI SiteSettings).
type Settings struct {
	Version          int              `json:"version"`
	Intro            Localized        `json:"intro"`
	Bio              Localized        `json:"bio"`
	ContactEmail     string           `json:"contact_email"`
	Links            []Link           `json:"links"`
	FeaturedProjects []FeaturedChoice `json:"featured_projects"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// FeaturedChoice describes a selected project for the panel, including whether it is visible
// (selected but unpublished projects stay selected and simply do not show).
type FeaturedChoice struct {
	ContentID string             `json:"content_id"`
	Titles    map[string]*string `json:"titles"`
	Visible   map[string]bool    `json:"visible"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Public holds only the fields any visitor may see, for one locale.
type Public struct {
	Intro        string `json:"intro"`
	Bio          string `json:"bio"`
	ContactEmail string `json:"contact_email"`
	Links        []Link `json:"links"`
}

func (s *Store) Public(ctx context.Context, locale string) (Public, error) {
	var p Public
	var intro, bio Localized
	var links []byte
	err := s.pool.QueryRow(ctx, `SELECT intro_es, intro_en, bio_es, bio_en, contact_email, links FROM site_settings`).
		Scan(&intro.ES, &intro.EN, &bio.ES, &bio.EN, &p.ContactEmail, &links)
	if err != nil {
		return p, err
	}
	p.Intro, p.Bio = intro.In(locale), bio.In(locale)
	if err := json.Unmarshal(links, &p.Links); err != nil {
		return p, err
	}
	return p, nil
}

// FeaturedIDs returns the selected projects in order (visibility is checked by the caller).
func (s *Store) FeaturedIDs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT content_id::text FROM site_featured_projects ORDER BY position`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *Store) Get(ctx context.Context) (Settings, error) {
	return get(ctx, s.pool)
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func get(ctx context.Context, q querier) (Settings, error) {
	var st Settings
	var links []byte
	err := q.QueryRow(ctx, `SELECT version, intro_es, intro_en, bio_es, bio_en, contact_email, links, updated_at FROM site_settings`).
		Scan(&st.Version, &st.Intro.ES, &st.Intro.EN, &st.Bio.ES, &st.Bio.EN, &st.ContactEmail, &links, &st.UpdatedAt)
	if err != nil {
		return st, err
	}
	st.UpdatedAt = st.UpdatedAt.UTC()
	if err := json.Unmarshal(links, &st.Links); err != nil {
		return st, err
	}
	rows, err := q.Query(ctx, `
		SELECT f.content_id::text, l.locale, r.title, v.translation_id IS NOT NULL
		FROM site_featured_projects f
		CROSS JOIN (VALUES ('es'), ('en')) AS l (locale)
		LEFT JOIN translations t ON t.content_id = f.content_id AND t.locale = l.locale
		LEFT JOIN revisions r ON r.translation_id = t.id AND r.version = t.latest_version
		LEFT JOIN visible_translations v ON v.translation_id = t.id
		ORDER BY f.position, l.locale`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	st.FeaturedProjects = []FeaturedChoice{}
	for rows.Next() {
		var id, locale string
		var title *string
		var visible bool
		if err := rows.Scan(&id, &locale, &title, &visible); err != nil {
			return st, err
		}
		n := len(st.FeaturedProjects)
		if n == 0 || st.FeaturedProjects[n-1].ContentID != id {
			st.FeaturedProjects = append(st.FeaturedProjects, FeaturedChoice{ContentID: id, Titles: map[string]*string{}, Visible: map[string]bool{}})
			n++
		}
		st.FeaturedProjects[n-1].Titles[locale] = title
		st.FeaturedProjects[n-1].Visible[locale] = visible
	}
	return st, rows.Err()
}

// Update is the body of PUT /admin/site.
type Update struct {
	ExpectedVersion  *int      `json:"expected_version"`
	Intro            Localized `json:"intro"`
	Bio              Localized `json:"bio"`
	ContactEmail     string    `json:"contact_email"`
	Links            []Link    `json:"links"`
	FeaturedProjects []string  `json:"featured_project_ids"`
}

// Validate normalizes u and returns per-field problems.
func (u *Update) Validate() map[string]string {
	errs := map[string]string{}
	if u.ExpectedVersion == nil || *u.ExpectedVersion < 0 {
		errs["expected_version"] = "is required (integer ≥ 0)"
	}
	for field, v := range map[string]*string{"intro.es": &u.Intro.ES, "intro.en": &u.Intro.EN, "bio.es": &u.Bio.ES, "bio.en": &u.Bio.EN} {
		*v = strings.TrimSpace(strings.ReplaceAll(*v, "\r\n", "\n"))
		limit := MaxBio
		if strings.HasPrefix(field, "intro") {
			limit = MaxIntro
		}
		if utf8.RuneCountInString(*v) > limit {
			errs[field] = fmt.Sprintf("at most %d characters", limit)
		} else if hasControl(*v) {
			errs[field] = "contains control characters"
		}
	}
	u.ContactEmail = strings.TrimSpace(u.ContactEmail)
	if u.ContactEmail != "" {
		addr, err := mail.ParseAddress(u.ContactEmail)
		if err != nil || addr.Address != u.ContactEmail || len(u.ContactEmail) > 254 {
			errs["contact_email"] = "must be a plain e-mail address"
		}
	}
	if u.Links == nil {
		u.Links = []Link{}
	}
	if len(u.Links) > MaxLinks {
		errs["links"] = fmt.Sprintf("at most %d links", MaxLinks)
	}
	for i := range u.Links {
		l := &u.Links[i]
		l.Label, l.URL = strings.TrimSpace(l.Label), strings.TrimSpace(l.URL)
		key := fmt.Sprintf("links.%d", i)
		switch {
		case !linkKinds[l.Kind]:
			errs[key+".kind"] = "must be github, linkedin, website or other"
		case l.Label == "" || utf8.RuneCountInString(l.Label) > maxLabel || hasControl(l.Label):
			errs[key+".label"] = fmt.Sprintf("use 1-%d characters", maxLabel)
		}
		if msg := checkURL(l.URL); msg != "" {
			errs[key+".url"] = msg
		}
	}
	if u.FeaturedProjects == nil {
		u.FeaturedProjects = []string{}
	}
	if len(u.FeaturedProjects) > MaxFeatured {
		errs["featured_project_ids"] = fmt.Sprintf("at most %d projects", MaxFeatured)
	}
	seen := map[string]bool{}
	for i, id := range u.FeaturedProjects {
		id = strings.ToLower(id)
		u.FeaturedProjects[i] = id
		if !idPattern.MatchString(id) {
			errs[fmt.Sprintf("featured_project_ids.%d", i)] = "must be a UUID"
		} else if seen[id] {
			errs[fmt.Sprintf("featured_project_ids.%d", i)] = "is repeated"
		}
		seen[id] = true
	}
	return errs
}

// checkURL accepts only absolute https URLs with a host and no credentials.
func checkURL(raw string) string {
	if raw == "" || len(raw) > maxURL || hasControl(raw) {
		return fmt.Sprintf("use an https:// URL of up to %d characters", maxURL)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" {
		return "must be an absolute https:// URL without credentials"
	}
	return ""
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 && r != '\n' || r == 0x7f {
			return true
		}
	}
	return false
}

var (
	errConflict = errors.New("site settings changed")
)

type projectError struct{ index int }

func (projectError) Error() string { return "featured content is not an active project" }

// Save replaces the settings when expected matches the stored version.
func (s *Store) Save(ctx context.Context, u Update, actor string) (Settings, int, error) {
	var out Settings
	var current int
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT version FROM site_settings FOR UPDATE`).Scan(&current); err != nil {
			return err
		}
		if current != *u.ExpectedVersion {
			return errConflict
		}
		for i, id := range u.FeaturedProjects {
			var one int
			err := tx.QueryRow(ctx, `SELECT 1 FROM contents WHERE id = $1 AND kind = 'project' AND archived_at IS NULL FOR KEY SHARE`, id).Scan(&one)
			if errors.Is(err, pgx.ErrNoRows) {
				return projectError{i}
			}
			if err != nil {
				return err
			}
		}
		links, _ := json.Marshal(u.Links)
		if _, err := tx.Exec(ctx, `UPDATE site_settings SET version = version + 1, intro_es = $1, intro_en = $2, bio_es = $3, bio_en = $4,
			contact_email = $5, links = $6, updated_at = now()`, u.Intro.ES, u.Intro.EN, u.Bio.ES, u.Bio.EN, u.ContactEmail, links); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM site_featured_projects`); err != nil {
			return err
		}
		for i, id := range u.FeaturedProjects {
			if _, err := tx.Exec(ctx, `INSERT INTO site_featured_projects (content_id, position) VALUES ($1, $2)`, id, i); err != nil {
				return err
			}
		}
		meta, _ := json.Marshal(map[string]any{"version": current + 1, "featured": u.FeaturedProjects, "links": len(u.Links)})
		if _, err := tx.Exec(ctx, `INSERT INTO audit_events (actor, action, entity_type, metadata) VALUES ($1, 'site.update', 'site', $2)`, actor, meta); err != nil {
			return err
		}
		var err error
		out, err = get(ctx, tx)
		return err
	})
	return out, current, err
}

// Handler serves GET/PUT /api/v1/admin/site (owner only).
type Handler struct {
	store        *Store
	logger       *slog.Logger
	requireOwner func(http.Handler) http.Handler
	actor        func(*http.Request) string
}

func NewHandler(store *Store, logger *slog.Logger, requireOwner func(http.Handler) http.Handler, actor func(*http.Request) string) *Handler {
	return &Handler{store: store, logger: logger, requireOwner: requireOwner, actor: actor}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/admin/site", h.requireOwner(http.HandlerFunc(h.get)))
	mux.Handle("PUT /api/v1/admin/site", h.requireOwner(http.HandlerFunc(h.put)))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	st, err := h.store.Get(r.Context())
	if err != nil {
		h.fail(w, r, "get site settings", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, st)
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	var u Update
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&u); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpapi.WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body is too large")
			return
		}
		h.invalid(w, r, map[string]string{"body": "must be a JSON object with the documented fields"})
		return
	}
	if errs := u.Validate(); len(errs) > 0 {
		h.invalid(w, r, errs)
		return
	}
	st, current, err := h.store.Save(r.Context(), u, h.actor(r))
	var pe projectError
	switch {
	case errors.Is(err, errConflict):
		httpapi.WriteJSON(w, http.StatusConflict, httpapi.Error{Code: "version_conflict", Message: "The site settings changed since expected_version",
			CurrentVersion: &current, RequestID: httpapi.RequestID(r.Context())})
	case errors.As(err, &pe):
		h.invalid(w, r, map[string]string{fmt.Sprintf("featured_project_ids.%d", pe.index): "must be an existing, non-archived project"})
	case err != nil:
		h.fail(w, r, "save site settings", err)
	default:
		httpapi.WriteJSON(w, http.StatusOK, st)
	}
}

func (h *Handler) invalid(w http.ResponseWriter, r *http.Request, errs map[string]string) {
	httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "validation_failed", Message: "Request is invalid",
		Fields: errs, RequestID: httpapi.RequestID(r.Context())})
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.Error(op, "request_id", httpapi.RequestID(r.Context()), "error", err)
	httpapi.WriteError(w, r, http.StatusInternalServerError, "internal", "Internal server error")
}
