package media

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errNotFound = errors.New("asset not found")
	errInUse    = errors.New("asset referenced by a revision")
)

type Text struct {
	Alt     string `json:"alt"`
	Caption string `json:"caption"`
}

type Asset struct {
	ID            string          `json:"id"`
	Kind          Kind            `json:"kind"`
	Status        string          `json:"status"`
	OriginalName  string          `json:"original_name"`
	MIME          string          `json:"mime"`
	Bytes         int64           `json:"bytes"`
	SHA256        string          `json:"sha256"`
	Width         *int            `json:"width"`
	Height        *int            `json:"height"`
	PublicEnabled bool            `json:"public_enabled"`
	Downloadable  bool            `json:"downloadable"`
	CreatedAt     time.Time       `json:"created_at"`
	Texts         map[string]Text `json:"texts"`

	objectKey string
	backend   string
}

type Reference struct {
	ContentID string `json:"content_id"`
	Locale    string `json:"locale"`
	Version   int    `json:"version"`
	Usage     string `json:"usage"`
	Published bool   `json:"published"`
}

type AssetDetail struct {
	Asset
	References []Reference `json:"references"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return Store{pool: pool} }

const assetColumns = `a.id, a.kind, a.status, a.original_name, a.mime, a.bytes, a.sha256, a.width, a.height,
	a.public_enabled, a.downloadable, a.created_at, a.object_key, a.storage_backend`

func scanAsset(row pgx.Row) (Asset, error) {
	var a Asset
	var sha []byte
	err := row.Scan(&a.ID, &a.Kind, &a.Status, &a.OriginalName, &a.MIME, &a.Bytes, &sha, &a.Width, &a.Height,
		&a.PublicEnabled, &a.Downloadable, &a.CreatedAt, &a.objectKey, &a.backend)
	a.SHA256 = hex.EncodeToString(sha)
	return a, err
}

func (s Store) texts(ctx context.Context, assets []Asset) error {
	if len(assets) == 0 {
		return nil
	}
	idx := map[string]int{}
	ids := make([]string, len(assets))
	for i := range assets {
		assets[i].Texts = map[string]Text{"es": {}, "en": {}}
		idx[assets[i].ID] = i
		ids[i] = assets[i].ID
	}
	rows, err := s.pool.Query(ctx, `SELECT asset_id, locale, alt, caption FROM asset_translations WHERE asset_id = ANY($1::uuid[])`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, locale string
		var t Text
		if err := rows.Scan(&id, &locale, &t.Alt, &t.Caption); err != nil {
			return err
		}
		assets[idx[id]].Texts[locale] = t
	}
	return rows.Err()
}

// CreatePending records an upload before any byte is read, so a crash leaves a traceable row.
func (s Store) CreatePending(ctx context.Context, kind Kind, backend, key, name string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `INSERT INTO assets (kind, storage_backend, object_key, original_name) VALUES ($1, $2, $3, $4) RETURNING id`,
		kind, backend, key, name).Scan(&id)
	return id, err
}

// MarkReady is called only after the object exists with the expected size.
func (s Store) MarkReady(ctx context.Context, id string, d Detected, size int64, sha []byte) error {
	var w, h *int
	if d.Width > 0 {
		w, h = &d.Width, &d.Height
	}
	tag, err := s.pool.Exec(ctx, `UPDATE assets SET status = 'ready', mime = $2, bytes = $3, sha256 = $4, width = $5, height = $6, updated_at = now()
		WHERE id = $1 AND status = 'pending'`, id, d.MIME, size, sha, w, h)
	if err == nil && tag.RowsAffected() != 1 {
		err = fmt.Errorf("asset %s is no longer pending", id)
	}
	return err
}

func (s Store) MarkFailed(ctx context.Context, id, reason string) error {
	_, err := s.pool.Exec(ctx, `UPDATE assets SET status = 'failed', failure = $2, updated_at = now() WHERE id = $1 AND status = 'pending'`, id, reason)
	return err
}

func (s Store) Get(ctx context.Context, id string) (Asset, error) {
	a, err := scanAsset(s.pool.QueryRow(ctx, `SELECT `+assetColumns+` FROM assets a WHERE a.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, errNotFound
	}
	if err != nil {
		return Asset{}, err
	}
	list := []Asset{a}
	err = s.texts(ctx, list)
	return list[0], err
}

func (s Store) Detail(ctx context.Context, id string) (AssetDetail, error) {
	a, err := s.Get(ctx, id)
	if err != nil {
		return AssetDetail{}, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT t.content_id, t.locale, r.version, ra.usage, t.published_revision_id = r.id
		FROM revision_assets ra JOIN revisions r ON r.id = ra.revision_id JOIN translations t ON t.id = r.translation_id
		WHERE ra.asset_id = $1 ORDER BY t.content_id, t.locale, r.version DESC`, id)
	if err != nil {
		return AssetDetail{}, err
	}
	refs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Reference, error) {
		var ref Reference
		var published *bool
		err := r.Scan(&ref.ContentID, &ref.Locale, &ref.Version, &ref.Usage, &published)
		ref.Published = published != nil && *published
		return ref, err
	})
	if refs == nil {
		refs = []Reference{}
	}
	return AssetDetail{Asset: a, References: refs}, err
}

// List returns ready assets (optionally of one kind), newest first.
func (s Store) List(ctx context.Context, kind Kind, page, size int) ([]Asset, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM assets a WHERE a.status = 'ready' AND ($1 = '' OR a.kind = $1)`, string(kind)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+assetColumns+` FROM assets a WHERE a.status = 'ready' AND ($1 = '' OR a.kind = $1)
		ORDER BY a.created_at DESC, a.id LIMIT $2 OFFSET $3`, string(kind), size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Asset, error) { return scanAsset(r) })
	if err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []Asset{}
	}
	return list, total, s.texts(ctx, list)
}

type Update struct {
	PublicEnabled *bool           `json:"public_enabled"`
	Downloadable  *bool           `json:"downloadable"`
	Texts         map[string]Text `json:"texts"`
}

// Apply changes library metadata and permissions only; bytes and revisions are never touched.
func (s Store) Apply(ctx context.Context, id, actor string, u Update) (Asset, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE assets SET public_enabled = COALESCE($2, public_enabled), downloadable = COALESCE($3, downloadable), updated_at = now()
			WHERE id = $1 AND status = 'ready'`, id, u.PublicEnabled, u.Downloadable)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errNotFound
		}
		for locale, t := range u.Texts {
			if _, err := tx.Exec(ctx, `INSERT INTO asset_translations (asset_id, locale, alt, caption) VALUES ($1, $2, $3, $4)
				ON CONFLICT (asset_id, locale) DO UPDATE SET alt = EXCLUDED.alt, caption = EXCLUDED.caption`, id, locale, t.Alt, t.Caption); err != nil {
				return err
			}
		}
		return audit(ctx, tx, actor, "asset.update", id)
	})
	if err != nil {
		return Asset{}, err
	}
	return s.Get(ctx, id)
}

// Delete removes the row only when no retained revision references the asset. The FK
// (ON DELETE RESTRICT) makes the check race-free against a concurrent save. It returns the
// object key so the caller deletes the bytes after the row is gone.
func (s Store) Delete(ctx context.Context, id, actor string) (string, error) {
	var key string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `DELETE FROM assets WHERE id = $1 AND status <> 'pending' RETURNING object_key`, id).Scan(&key)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		}
		var pgErr *pgconn.PgError
		// ON DELETE RESTRICT raises restrict_violation (23001); a plain FK raises 23503.
		if errors.As(err, &pgErr) && (pgErr.Code == "23001" || pgErr.Code == "23503") {
			return errInUse
		}
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "asset.delete", id)
	})
	return key, err
}

// StalePending returns uploads stuck in pending (crash between write and transaction).
func (s Store) StalePending(ctx context.Context, olderThan time.Duration) ([]Asset, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+assetColumns+` FROM assets a WHERE a.status = 'pending' AND a.created_at < now() - make_interval(secs => $1)`,
		olderThan.Seconds())
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Asset, error) { return scanAsset(r) })
}

func audit(ctx context.Context, tx pgx.Tx, actor, action, id string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events (actor, action, entity_type, entity_id) VALUES ($1, $2, 'asset', $3)`, actor, action, id)
	return err
}
