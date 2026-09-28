package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errNotFound            = errors.New("not found")
	errArchived            = errors.New("content is archived")
	errPublished           = errors.New("content has published translations")
	errLocaleExists        = errors.New("translation already exists")
	errNothingToCopy       = errors.New("source translation has no revision")
	errIdempotencyMismatch = errors.New("idempotency key reused with a different snapshot")
	errTermSlugTaken       = errors.New("slug already used")
)

// conflictError reports that expected_version is stale.
type conflictError struct{ current int }

func (e conflictError) Error() string { return fmt.Sprintf("version conflict (current %d)", e.current) }

type TranslationSummary struct {
	Locale        string     `json:"locale"`
	LatestVersion int        `json:"latest_version"`
	Title         *string    `json:"title"`
	Published     bool       `json:"published"`
	UpdatedAt     *time.Time `json:"updated_at"`
}

type Content struct {
	ID           string               `json:"id"`
	Kind         string               `json:"kind"`
	ProjectID    *string              `json:"project_id"`
	CreatedAt    time.Time            `json:"created_at"`
	ArchivedAt   *time.Time           `json:"archived_at"`
	Translations []TranslationSummary `json:"translations"`
}

type Revision struct {
	Snapshot
	Version             int       `json:"version"`
	Kind                string    `json:"kind"`
	CreatedAt           time.Time `json:"created_at"`
	BodySchemaVersion   int       `json:"body_schema_version"`
	RestoredFromVersion *int      `json:"restored_from_version"`
	CopiedFromLocale    *string   `json:"copied_from_locale"`
}

type RevisionSummary struct {
	Version             int       `json:"version"`
	Kind                string    `json:"kind"`
	CreatedAt           time.Time `json:"created_at"`
	Title               string    `json:"title"`
	RestoredFromVersion *int      `json:"restored_from_version"`
}

type Translation struct {
	ContentID     string    `json:"content_id"`
	Locale        string    `json:"locale"`
	LatestVersion int       `json:"latest_version"`
	Published     bool      `json:"published"`
	Latest        *Revision `json:"latest"`
}

type Term struct {
	ID     string            `json:"id"`
	Slug   string            `json:"slug"`
	Labels map[string]string `json:"labels"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return Store{pool: pool} }

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// --- Contents -----------------------------------------------------------------------------

type ListFilter struct {
	Kind      string
	ProjectID string
	Archived  bool
	Page      int
	PageSize  int
}

func (s Store) ListContents(ctx context.Context, f ListFilter) ([]Content, int, error) {
	where := `WHERE (archived_at IS NOT NULL) = $1 AND ($2 = '' OR kind = $2) AND ($3 = '' OR project_id::text = $3)`
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM contents `+where, f.Archived, f.Kind, f.ProjectID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id FROM contents `+where+` ORDER BY created_at DESC, id LIMIT $4 OFFSET $5`,
		f.Archived, f.Kind, f.ProjectID, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return nil, 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, 0, err
	}
	items := make([]Content, 0, len(ids))
	for _, id := range ids {
		c, err := s.GetContent(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, c)
	}
	return items, total, nil
}

func (s Store) GetContent(ctx context.Context, id string) (Content, error) {
	return getContent(ctx, s.pool, id)
}

func getContent(ctx context.Context, q querier, id string) (Content, error) {
	var c Content
	err := q.QueryRow(ctx, `SELECT id, kind, project_id, created_at, archived_at FROM contents WHERE id = $1`, id).
		Scan(&c.ID, &c.Kind, &c.ProjectID, &c.CreatedAt, &c.ArchivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Content{}, errNotFound
	}
	if err != nil {
		return Content{}, err
	}
	rows, err := q.Query(ctx, `
		SELECT t.locale, t.latest_version, r.title, t.published_revision_id IS NOT NULL, r.created_at
		FROM translations t
		LEFT JOIN revisions r ON r.translation_id = t.id AND r.version = t.latest_version
		WHERE t.content_id = $1 ORDER BY t.locale`, id)
	if err != nil {
		return Content{}, err
	}
	c.Translations, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (TranslationSummary, error) {
		var t TranslationSummary
		err := r.Scan(&t.Locale, &t.LatestVersion, &t.Title, &t.Published, &t.UpdatedAt)
		return t, err
	})
	return c, err
}

// CreateContent creates the content and its first, empty translation. A log needs an active project.
func (s Store) CreateContent(ctx context.Context, kind, projectID, locale string) (Content, error) {
	var id string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var parent *string
		if kind == "log" {
			var parentKind string
			var archived *time.Time
			err := tx.QueryRow(ctx, `SELECT kind, archived_at FROM contents WHERE id = $1 FOR SHARE`, projectID).Scan(&parentKind, &archived)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && parentKind != "project") {
				return FieldErrors{"project_id": "must reference an existing project"}
			}
			if err != nil {
				return err
			}
			if archived != nil {
				return FieldErrors{"project_id": "the project is archived"}
			}
			parent = &projectID
		}
		if err := tx.QueryRow(ctx, `INSERT INTO contents (kind, project_id) VALUES ($1, $2) RETURNING id`, kind, parent).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO translations (content_id, locale) VALUES ($1, $2)`, id, locale); err != nil {
			return err
		}
		return audit(ctx, tx, "content.create", id, map[string]string{"kind": kind, "locale": locale})
	})
	if err != nil {
		return Content{}, err
	}
	return s.GetContent(ctx, id)
}

// SetArchived archives or restores a content. Archiving published content is rejected: withdrawing
// from the public site is a WEB-005 operation.
func (s Store) SetArchived(ctx context.Context, id string, archived bool) (Content, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT true FROM contents WHERE id = $1 FOR UPDATE`, id).Scan(&exists); errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		} else if err != nil {
			return err
		}
		if archived {
			var published bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM translations WHERE content_id = $1 AND published_revision_id IS NOT NULL)`, id).Scan(&published); err != nil {
				return err
			}
			if published {
				return errPublished
			}
			_, err := tx.Exec(ctx, `UPDATE contents SET archived_at = COALESCE(archived_at, now()) WHERE id = $1`, id)
			if err != nil {
				return err
			}
			return audit(ctx, tx, "content.archive", id, nil)
		}
		if _, err := tx.Exec(ctx, `UPDATE contents SET archived_at = NULL WHERE id = $1`, id); err != nil {
			return err
		}
		return audit(ctx, tx, "content.unarchive", id, nil)
	})
	if err != nil {
		return Content{}, err
	}
	return s.GetContent(ctx, id)
}

// --- Translations and revisions ---------------------------------------------------------------

type translationRow struct {
	id       string
	kind     string
	latest   int
	archived bool
}

func lockTranslation(ctx context.Context, tx pgx.Tx, contentID, locale string) (translationRow, error) {
	var t translationRow
	err := tx.QueryRow(ctx, `
		SELECT t.id, c.kind, t.latest_version, c.archived_at IS NOT NULL
		FROM translations t JOIN contents c ON c.id = t.content_id
		WHERE t.content_id = $1 AND t.locale = $2
		FOR UPDATE OF t`, contentID, locale).Scan(&t.id, &t.kind, &t.latest, &t.archived)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, errNotFound
	}
	return t, err
}

// CreateTranslation adds a locale: empty, or as an explicit copy (kind=copy) of another locale's
// latest revision, marked as pending translation. Never machine-translated.
func (s Store) CreateTranslation(ctx context.Context, contentID, locale, copyFrom string) (Translation, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var archived bool
		if err := tx.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM contents WHERE id = $1 FOR SHARE`, contentID).Scan(&archived); errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		} else if err != nil {
			return err
		}
		if archived {
			return errArchived
		}
		var tID string
		err := tx.QueryRow(ctx, `INSERT INTO translations (content_id, locale) VALUES ($1, $2) ON CONFLICT DO NOTHING RETURNING id`,
			contentID, locale).Scan(&tID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errLocaleExists
		}
		if err != nil {
			return err
		}
		if copyFrom != "" {
			src, err := latestRevision(ctx, tx, contentID, copyFrom)
			if err != nil {
				return err
			}
			if src == nil {
				return errNothingToCopy
			}
			if err := insertRevision(ctx, tx, tID, 1, "copy", nil, &copyFrom, src.Snapshot, plainOf(src), hashOf(ctx, tx, contentID, copyFrom, src.Version), ""); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE translations SET latest_version = 1, updated_at = now() WHERE id = $1`, tID); err != nil {
				return err
			}
		}
		return audit(ctx, tx, "translation.create", contentID, map[string]string{"locale": locale, "copied_from": copyFrom})
	})
	if err != nil {
		return Translation{}, err
	}
	return s.GetTranslation(ctx, contentID, locale)
}

func (s Store) GetTranslation(ctx context.Context, contentID, locale string) (Translation, error) {
	t := Translation{ContentID: contentID, Locale: locale}
	err := s.pool.QueryRow(ctx, `SELECT latest_version, published_revision_id IS NOT NULL FROM translations WHERE content_id = $1 AND locale = $2`,
		contentID, locale).Scan(&t.LatestVersion, &t.Published)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, errNotFound
	}
	if err != nil {
		return t, err
	}
	t.Latest, err = latestRevision(ctx, s.pool, contentID, locale)
	return t, err
}

func (s Store) ListRevisions(ctx context.Context, contentID, locale string, page, size int) ([]RevisionSummary, int, error) {
	var tID string
	var total int
	err := s.pool.QueryRow(ctx, `SELECT id, latest_version FROM translations WHERE content_id = $1 AND locale = $2`, contentID, locale).Scan(&tID, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, errNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT version, kind, created_at, title, restored_from_version FROM revisions
		WHERE translation_id = $1 ORDER BY version DESC LIMIT $2 OFFSET $3`, tID, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (RevisionSummary, error) {
		var rs RevisionSummary
		err := r.Scan(&rs.Version, &rs.Kind, &rs.CreatedAt, &rs.Title, &rs.RestoredFromVersion)
		return rs, err
	})
	return items, total, err
}

func (s Store) GetRevision(ctx context.Context, contentID, locale string, version int) (*Revision, error) {
	r, err := revisionAt(ctx, s.pool, contentID, locale, version)
	if err == nil && r == nil {
		return nil, errNotFound
	}
	return r, err
}

// SaveInput is a validated, normalized snapshot ready to persist.
type SaveInput struct {
	ContentID       string
	Locale          string
	ExpectedVersion int
	Kind            string // manual or auto
	Snapshot        Snapshot
	PlainText       string
	IdempotencyKey  string
}

// SaveRevision appends a revision atomically. Order: idempotent replay, archived check, version
// check (409), no-op if identical to the latest, taxonomy existence, insert.
func (s Store) SaveRevision(ctx context.Context, in SaveInput) (*Revision, bool, error) {
	hash := in.Snapshot.Hash()
	var version int
	created := false
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := lockTranslation(ctx, tx, in.ContentID, in.Locale)
		if err != nil {
			return err
		}
		if in.IdempotencyKey != "" {
			var v int
			var h []byte
			err := tx.QueryRow(ctx, `SELECT version, snapshot_hash FROM revisions WHERE translation_id = $1 AND idempotency_key = $2`,
				t.id, in.IdempotencyKey).Scan(&v, &h)
			if err == nil {
				if !bytes.Equal(h, hash) {
					return errIdempotencyMismatch
				}
				version, created = v, true // replay of a completed save: same answer
				return nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		if t.archived {
			return errArchived
		}
		if t.latest != in.ExpectedVersion {
			return conflictError{current: t.latest}
		}
		if t.latest > 0 {
			var h []byte
			if err := tx.QueryRow(ctx, `SELECT snapshot_hash FROM revisions WHERE translation_id = $1 AND version = $2`, t.id, t.latest).Scan(&h); err != nil {
				return err
			}
			if bytes.Equal(h, hash) {
				version = t.latest
				return nil
			}
		}
		if err := checkTerms(ctx, tx, in.Snapshot); err != nil {
			return err
		}
		version = t.latest + 1
		if err := insertRevision(ctx, tx, t.id, version, in.Kind, nil, nil, in.Snapshot, in.PlainText, hash, in.IdempotencyKey); err != nil {
			return err
		}
		created = true
		return bumpVersion(ctx, tx, t.id, t.latest)
	})
	if err != nil {
		return nil, false, err
	}
	r, err := revisionAt(ctx, s.pool, in.ContentID, in.Locale, version)
	return r, created, err
}

// RestoreRevision copies an old snapshot into a new revision (kind=restore). The original is untouched.
func (s Store) RestoreRevision(ctx context.Context, contentID, locale string, from, expected int) (*Revision, bool, error) {
	var version int
	created := false
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := lockTranslation(ctx, tx, contentID, locale)
		if err != nil {
			return err
		}
		src, err := revisionAt(ctx, tx, contentID, locale, from)
		if err != nil {
			return err
		}
		if src == nil {
			return errNotFound
		}
		if t.archived {
			return errArchived
		}
		if t.latest != expected {
			return conflictError{current: t.latest}
		}
		srcHash := hashOf(ctx, tx, contentID, locale, from)
		var latestHash []byte
		if err := tx.QueryRow(ctx, `SELECT snapshot_hash FROM revisions WHERE translation_id = $1 AND version = $2`, t.id, t.latest).Scan(&latestHash); err != nil {
			return err
		}
		if bytes.Equal(latestHash, srcHash) {
			version = t.latest
			return nil
		}
		version = t.latest + 1
		if err := insertRevision(ctx, tx, t.id, version, "restore", &from, nil, src.Snapshot, plainOf(src), srcHash, ""); err != nil {
			return err
		}
		created = true
		if err := bumpVersion(ctx, tx, t.id, t.latest); err != nil {
			return err
		}
		return audit(ctx, tx, "revision.restore", contentID, map[string]string{"locale": locale, "from": fmt.Sprint(from)})
	})
	if err != nil {
		return nil, false, err
	}
	r, err := revisionAt(ctx, s.pool, contentID, locale, version)
	return r, created, err
}

// bumpVersion advances latest_version only if nobody else did (belt and braces over FOR UPDATE).
func bumpVersion(ctx context.Context, tx pgx.Tx, translationID string, expected int) error {
	tag, err := tx.Exec(ctx, `UPDATE translations SET latest_version = latest_version + 1, updated_at = now()
		WHERE id = $1 AND latest_version = $2`, translationID, expected)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return conflictError{current: -1}
	}
	return nil
}

func checkTerms(ctx context.Context, q querier, s Snapshot) error {
	if s.CategoryID != nil {
		var ok bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM categories WHERE id = $1)`, *s.CategoryID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return FieldErrors{"category_id": "unknown category"}
		}
	}
	if len(s.TagIDs) > 0 {
		var n int
		if err := q.QueryRow(ctx, `SELECT count(*) FROM tags WHERE id = ANY($1::uuid[])`, s.TagIDs).Scan(&n); err != nil {
			return err
		}
		if n != len(s.TagIDs) {
			return FieldErrors{"tag_ids": "unknown tag"}
		}
	}
	return nil
}

func insertRevision(ctx context.Context, tx pgx.Tx, translationID string, version int, kind string,
	restoredFrom *int, copiedFrom *string, s Snapshot, plain string, hash []byte, idemKey string) error {
	body, err := CanonicalJSON(s.Body)
	if err != nil {
		return err
	}
	seo, _ := CanonicalJSON(s.SEO)
	var project []byte
	if s.ProjectFields != nil {
		project, _ = CanonicalJSON(s.ProjectFields)
	}
	var key *string
	if idemKey != "" {
		key = &idemKey
	}
	var revID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO revisions (translation_id, version, kind, restored_from_version, copied_from_locale, title, slug, summary,
			body_json, body_schema_version, plain_text, seo, project_fields, category_id, snapshot_hash, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16) RETURNING id`,
		translationID, version, kind, restoredFrom, copiedFrom, s.Title, s.Slug, s.Summary,
		body, SchemaVersion, plain, seo, project, s.CategoryID, hash, key).Scan(&revID); err != nil {
		return err
	}
	for _, tag := range s.TagIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO revision_tags (revision_id, tag_id) VALUES ($1, $2)`, revID, tag); err != nil {
			return err
		}
	}
	return nil
}

func latestRevision(ctx context.Context, q querier, contentID, locale string) (*Revision, error) {
	var latest int
	err := q.QueryRow(ctx, `SELECT latest_version FROM translations WHERE content_id = $1 AND locale = $2`, contentID, locale).Scan(&latest)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil || latest == 0 {
		return nil, err
	}
	return revisionAt(ctx, q, contentID, locale, latest)
}

// revisionAt returns nil, nil when the version does not exist.
func revisionAt(ctx context.Context, q querier, contentID, locale string, version int) (*Revision, error) {
	var r Revision
	var body, seo, project []byte
	err := q.QueryRow(ctx, `
		SELECT r.version, r.kind, r.created_at, r.body_schema_version, r.restored_from_version, r.copied_from_locale,
			r.title, r.slug, r.summary, r.body_json, r.seo, r.project_fields, r.category_id,
			COALESCE((SELECT array_agg(tag_id::text ORDER BY tag_id) FROM revision_tags WHERE revision_id = r.id), '{}')
		FROM revisions r JOIN translations t ON t.id = r.translation_id
		WHERE t.content_id = $1 AND t.locale = $2 AND r.version = $3`, contentID, locale, version).
		Scan(&r.Version, &r.Kind, &r.CreatedAt, &r.BodySchemaVersion, &r.RestoredFromVersion, &r.CopiedFromLocale,
			&r.Title, &r.Slug, &r.Summary, &body, &seo, &project, &r.CategoryID, &r.TagIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := DecodeStrict(body, &r.Body); err != nil {
		return nil, fmt.Errorf("stored body: %w", err)
	}
	if err := json.Unmarshal(seo, &r.SEO); err != nil {
		return nil, fmt.Errorf("stored seo: %w", err)
	}
	if project != nil {
		r.ProjectFields = &ProjectFields{}
		if err := json.Unmarshal(project, r.ProjectFields); err != nil {
			return nil, fmt.Errorf("stored project fields: %w", err)
		}
	}
	return &r, nil
}

func hashOf(ctx context.Context, q querier, contentID, locale string, version int) []byte {
	var h []byte
	_ = q.QueryRow(ctx, `SELECT r.snapshot_hash FROM revisions r JOIN translations t ON t.id = r.translation_id
		WHERE t.content_id = $1 AND t.locale = $2 AND r.version = $3`, contentID, locale, version).Scan(&h)
	return h
}

func plainOf(r *Revision) string {
	_, plain, err := ValidateDocument(r.Body, DocumentOptions{AllowMedia: true})
	if err != nil {
		return ""
	}
	return plain
}

// --- Taxonomy -----------------------------------------------------------------------------------

// termTable whitelists the two taxonomy tables; the name never comes from the request.
func termTable(kind string) string {
	if kind == "tags" {
		return "tags"
	}
	return "categories"
}

func (s Store) ListTerms(ctx context.Context, kind string) ([]Term, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, slug, label_es, label_en FROM `+termTable(kind)+` ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanTerm)
}

func (s Store) CreateTerm(ctx context.Context, kind, slug, es, en string) (Term, error) {
	row, err := s.pool.Query(ctx, `INSERT INTO `+termTable(kind)+` (slug, label_es, label_en) VALUES ($1, $2, $3)
		RETURNING id, slug, label_es, label_en`, slug, es, en)
	if err != nil {
		return Term{}, err
	}
	t, err := pgx.CollectExactlyOneRow(row, scanTerm)
	return t, uniqueToTaken(err)
}

func (s Store) UpdateTerm(ctx context.Context, kind, id, slug, es, en string) (Term, error) {
	row, err := s.pool.Query(ctx, `UPDATE `+termTable(kind)+` SET slug = $2, label_es = $3, label_en = $4 WHERE id = $1
		RETURNING id, slug, label_es, label_en`, id, slug, es, en)
	if err != nil {
		return Term{}, err
	}
	t, err := pgx.CollectExactlyOneRow(row, scanTerm)
	if errors.Is(err, pgx.ErrNoRows) {
		return Term{}, errNotFound
	}
	return t, uniqueToTaken(err)
}

func scanTerm(r pgx.CollectableRow) (Term, error) {
	var t Term
	var es, en string
	err := r.Scan(&t.ID, &t.Slug, &es, &en)
	t.Labels = map[string]string{"es": es, "en": en}
	return t, err
}

func uniqueToTaken(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errTermSlugTaken
	}
	return err
}

type actorKey struct{}

// WithActor records who performs the operation (e.g. "github:201345228") for audit events.
func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func audit(ctx context.Context, q querier, action, contentID string, metadata map[string]string) error {
	actor, _ := ctx.Value(actorKey{}).(string)
	if actor == "" {
		return errors.New("audit: missing actor")
	}
	meta, _ := CanonicalJSON(metadata)
	if metadata == nil {
		meta = []byte("{}")
	}
	_, err := q.Exec(ctx, `INSERT INTO audit_events (actor, action, entity_type, entity_id, metadata) VALUES ($1, $2, 'content', $3, $4)`,
		actor, action, contentID, meta)
	return err
}
