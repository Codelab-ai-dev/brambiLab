// Package publishing implements publication, scheduling and withdrawal per translation, public
// routes and the minimal public reader (web-v1.md §8.1, WEB-005).
//
// Lock order, the same in every operation (manual, scheduler, cancel, withdraw, archive):
//
//	parent project translation (FOR SHARE) → content (FOR SHARE) + translation (FOR UPDATE)
//	→ publication_jobs rows → public_routes rows
//
// A single order prevents deadlocks and serializes parent/child races.
package publishing

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	_ "time/tzdata" // IANA zones embedded: the image may not ship tzdata

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/ops"
)

// EditorialZone is the only zone for editorial dates (web-v1.md §8).
const EditorialZone = "America/Mexico_City"

var editorialLocation = func() *time.Location {
	loc, err := time.LoadLocation(EditorialZone)
	if err != nil {
		panic(err)
	}
	return loc
}()

// Service holds the transactional operations. Now is injectable for tests.
type Service struct {
	pool *pgxpool.Pool
	Now  func() time.Time
	// MaxAttempts and backoff for transient scheduler errors.
	MaxAttempts int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration

	hooks hooks
	// Gate pauses the scheduler (maintenance, BACKGROUND_JOBS=off).
	Gate ops.Gate
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, Now: time.Now, MaxAttempts: 5, BaseBackoff: 30 * time.Second, MaxBackoff: 10 * time.Minute}
}

// --- Errors -------------------------------------------------------------------------------------

var (
	errNotFound          = errors.New("not found")
	errNoActiveJob       = errors.New("no active job")
	errScheduleExists    = errors.New("schedule exists")
	errNothingToWithdraw = errors.New("nothing to withdraw")
	errIdempotencyReuse  = errors.New("idempotency key reused")
	errJobNotFailed      = errors.New("job is not failed")
)

type conflictError struct{ state *State }

func (conflictError) Error() string { return "editorial version conflict" }

type slugTakenError struct{ slug string }

func (e slugTakenError) Error() string { return "slug taken: " + e.slug }

// Problem is one reason a revision cannot be published (reported, never auto-fixed).
type Problem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type blockedError struct{ problems []Problem }

func (e blockedError) Error() string {
	return fmt.Sprintf("publish blocked: %d problem(s)", len(e.problems))
}

type dependencyError struct{ logs []Dependency }

func (dependencyError) Error() string { return "project has published logs" }

type Dependency struct {
	ContentID string `json:"content_id"`
	Title     string `json:"title"`
}

type invalidError struct{ field, message string }

func (e invalidError) Error() string { return e.field + ": " + e.message }

// --- Locked translation -------------------------------------------------------------------------

type translation struct {
	id, contentID, locale, kind string
	projectID                   *string
	archived                    bool
	editorial, latest           int
	publishedRevision           *string
	withdrawnAt                 *time.Time
}

// lockTranslation takes the locks in the canonical order.
func lockTranslation(ctx context.Context, tx pgx.Tx, contentID, locale string) (translation, error) {
	var t translation
	// Kind and parent never change (DB trigger), so reading them unlocked is safe.
	err := tx.QueryRow(ctx, `SELECT kind, project_id FROM contents WHERE id = $1`, contentID).Scan(&t.kind, &t.projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, errNotFound
	}
	if err != nil {
		return t, err
	}
	if t.projectID != nil {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM translations WHERE content_id = $1 AND locale = $2 FOR SHARE`, *t.projectID, locale); err != nil {
			return t, err
		}
	}
	err = tx.QueryRow(ctx, `
		SELECT t.id, t.content_id, t.locale, c.archived_at IS NOT NULL, t.editorial_version, t.latest_version,
		       t.published_revision_id, t.withdrawn_at
		FROM translations t JOIN contents c ON c.id = t.content_id
		WHERE t.content_id = $1 AND t.locale = $2
		FOR UPDATE OF t FOR SHARE OF c`, contentID, locale).
		Scan(&t.id, &t.contentID, &t.locale, &t.archived, &t.editorial, &t.latest, &t.publishedRevision, &t.withdrawnAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, errNotFound
	}
	return t, err
}

// --- Idempotency --------------------------------------------------------------------------------

type replay struct {
	status int
	body   json.RawMessage
}

func payloadHash(action string, payload any) []byte {
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(append([]byte(action+"\x00"), b...))
	return sum[:]
}

func lookupIdempotent(ctx context.Context, tx pgx.Tx, translationID, key string, hash []byte) (*replay, error) {
	if key == "" {
		return nil, nil
	}
	var r replay
	var stored []byte
	err := tx.QueryRow(ctx, `SELECT payload_hash, response_status, response_body FROM editorial_requests WHERE translation_id = $1 AND idempotency_key = $2`,
		translationID, key).Scan(&stored, &r.status, &r.body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if string(stored) != string(hash) {
		return nil, errIdempotencyReuse
	}
	return &r, nil
}

func storeIdempotent(ctx context.Context, tx pgx.Tx, translationID, key, action string, hash []byte, status int, body any) error {
	if key == "" {
		return nil
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO editorial_requests (translation_id, idempotency_key, action, payload_hash, response_status, response_body)
		VALUES ($1, $2, $3, $4, $5, $6)`, translationID, key, action, hash, status, b)
	return err
}

// Result of an editorial mutation (a replay carries the original answer).
type Result struct {
	Status int
	State  *State
	Replay *replay
}

// --- Revision lookup and validation ---------------------------------------------------------

type revision struct {
	id, slug, title string
	version         int
}

func revisionByVersion(ctx context.Context, tx pgx.Tx, translationID string, version int) (revision, error) {
	var r revision
	err := tx.QueryRow(ctx, `SELECT id, version, slug, title FROM revisions WHERE translation_id = $1 AND version = $2`, translationID, version).
		Scan(&r.id, &r.version, &r.slug, &r.title)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, invalidError{"revision_version", "esa revisión no existe en esta traducción"}
	}
	return r, err
}

func scopeFor(t translation) string {
	switch {
	case t.kind == "log" && t.projectID != nil:
		return "log:" + *t.projectID
	default:
		return t.kind
	}
}

// validate collects every problem preventing publication of rev. parentDeadline, when set
// (scheduling), also accepts a parent project scheduled at or before that instant.
func validate(ctx context.Context, tx pgx.Tx, t translation, rev revision, parentDeadline *time.Time) ([]Problem, error) {
	var problems []Problem
	if t.archived {
		problems = append(problems, Problem{"content", "el contenido está archivado"})
	}
	if rev.title == "" {
		problems = append(problems, Problem{"title", "falta el título"})
	}
	if !slugPattern.MatchString(rev.slug) || len(rev.slug) > 120 {
		problems = append(problems, Problem{"slug", "el slug no es válido"})
	}
	rows, err := tx.Query(ctx, `
		SELECT a.id::text, a.original_name, a.kind, a.status, a.public_enabled, a.downloadable, ra.usage
		FROM revision_assets ra JOIN assets a ON a.id = ra.asset_id
		WHERE ra.revision_id = $1 ORDER BY a.original_name, ra.usage
		FOR KEY SHARE OF a`, rev.id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, name, kind, status, usage string
		var public, downloadable bool
		if err := rows.Scan(&id, &name, &kind, &status, &public, &downloadable, &usage); err != nil {
			rows.Close()
			return nil, err
		}
		field := "assets." + id
		switch {
		case status != "ready":
			problems = append(problems, Problem{field, fmt.Sprintf("«%s» no está listo", name)})
		case !public:
			problems = append(problems, Problem{field, fmt.Sprintf("«%s» no está marcado como público", name)})
		case usage == "download" && !downloadable:
			problems = append(problems, Problem{field, fmt.Sprintf("«%s» no permite descarga", name)})
		case (usage == "cover" || usage == "poster" || usage == "image") && kind != "image":
			problems = append(problems, Problem{field, fmt.Sprintf("«%s» no es una imagen", name)})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if t.kind == "log" && t.projectID != nil {
		var visible, archived bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM visible_translations WHERE content_id = $1 AND locale = $2),
			(SELECT archived_at IS NOT NULL FROM contents WHERE id = $1)`, *t.projectID, t.locale).Scan(&visible, &archived); err != nil {
			return nil, err
		}
		ok := visible
		if !ok && parentDeadline != nil && !archived {
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM publication_jobs j JOIN translations pt ON pt.id = j.translation_id
				WHERE pt.content_id = $1 AND pt.locale = $2 AND j.status = 'scheduled' AND j.run_at <= $3)`, *t.projectID, t.locale, *parentDeadline).Scan(&ok); err != nil {
				return nil, err
			}
		}
		if !ok {
			problems = append(problems, Problem{"project", "el proyecto de esta bitácora no está publicado en este idioma"})
		}
	}
	return problems, nil
}

// routeFree fails with slugTakenError when another content owns the path (current or alias).
func routeFree(ctx context.Context, tx pgx.Tx, t translation, slug string) error {
	var owner string
	err := tx.QueryRow(ctx, `SELECT content_id::text FROM public_routes WHERE locale = $1 AND scope = $2 AND slug = $3`, t.locale, scopeFor(t), slug).Scan(&owner)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil
	case err != nil:
		return err
	case owner != t.contentID:
		return slugTakenError{slug}
	}
	return nil
}

// claimRoute makes (locale, scope, slug) the current route of the content; its previous current
// route becomes an alias. A concurrent claim of the same path fails on the primary key.
func claimRoute(ctx context.Context, tx pgx.Tx, t translation, slug string) error {
	scope := scopeFor(t)
	var owner string
	err := tx.QueryRow(ctx, `SELECT content_id::text FROM public_routes WHERE locale = $1 AND scope = $2 AND slug = $3 FOR UPDATE`, t.locale, scope, slug).Scan(&owner)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if exists && owner != t.contentID {
		return slugTakenError{slug}
	}
	if _, err := tx.Exec(ctx, `UPDATE public_routes SET is_current = false, updated_at = now()
		WHERE content_id = $1 AND locale = $2 AND is_current AND NOT (scope = $3 AND slug = $4)`, t.contentID, t.locale, scope, slug); err != nil {
		return err
	}
	if exists {
		_, err = tx.Exec(ctx, `UPDATE public_routes SET is_current = true, updated_at = now() WHERE locale = $1 AND scope = $2 AND slug = $3`, t.locale, scope, slug)
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public_routes (locale, scope, slug, content_id, is_current) VALUES ($1, $2, $3, $4, true)`, t.locale, scope, slug, t.contentID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return slugTakenError{slug}
	}
	return err
}

// publishLocked sets the public pointer to rev. It returns false when nothing changed.
func (s *Service) publishLocked(ctx context.Context, tx pgx.Tx, t translation, rev revision) (bool, error) {
	var currentSlug string
	_ = tx.QueryRow(ctx, `SELECT slug FROM public_routes WHERE content_id = $1 AND locale = $2 AND is_current`, t.contentID, t.locale).Scan(&currentSlug)
	if t.publishedRevision != nil && *t.publishedRevision == rev.id && currentSlug == rev.slug {
		return false, nil
	}
	if err := claimRoute(ctx, tx, t, rev.slug); err != nil {
		return false, err
	}
	now := s.Now()
	_, err := tx.Exec(ctx, `UPDATE translations SET published_revision_id = $2, published_at = $3,
		first_published_at = COALESCE(first_published_at, $3), withdrawn_at = NULL, updated_at = now() WHERE id = $1`, t.id, rev.id, now)
	return true, err
}

func bumpEditorial(ctx context.Context, tx pgx.Tx, translationID string) error {
	_, err := tx.Exec(ctx, `UPDATE translations SET editorial_version = editorial_version + 1 WHERE id = $1`, translationID)
	return err
}

func cancelActive(ctx context.Context, tx pgx.Tx, translationID, reason string, now time.Time) (int64, error) {
	tag, err := tx.Exec(ctx, `UPDATE publication_jobs SET status = 'cancelled', cancel_reason = $2, finished_at = $3, updated_at = $3
		WHERE translation_id = $1 AND status = 'scheduled'`, translationID, reason, now)
	return tag.RowsAffected(), err
}

func audit(ctx context.Context, tx pgx.Tx, actor, action string, t translation, detail map[string]any) error {
	detail["locale"] = t.locale
	detail["translation_id"] = t.id
	meta, _ := json.Marshal(detail)
	_, err := tx.Exec(ctx, `INSERT INTO audit_events (actor, action, entity_type, entity_id, metadata) VALUES ($1, $2, 'content', $3, $4)`,
		actor, action, t.contentID, meta)
	return err
}

// --- Operations ---------------------------------------------------------------------------------

type Request struct {
	ContentID, Locale, Actor, IdempotencyKey string
	ExpectedEditorialVersion                 int
}

// mutate runs one editorial operation with the shared prologue (locks, replay, version check)
// and epilogue (bump, idempotent answer).
func (s *Service) mutate(ctx context.Context, req Request, action string, payload any, op func(tx pgx.Tx, t translation) (changed bool, status int, err error)) (Result, error) {
	var res Result
	hash := payloadHash(action, payload)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := lockTranslation(ctx, tx, req.ContentID, req.Locale)
		if err != nil {
			return err
		}
		if rp, err := lookupIdempotent(ctx, tx, t.id, req.IdempotencyKey, hash); err != nil {
			return err
		} else if rp != nil {
			res.Replay = rp
			return nil
		}
		if t.editorial != req.ExpectedEditorialVersion {
			st, err := loadState(ctx, tx, t.contentID, t.locale)
			if err != nil {
				return err
			}
			return conflictError{&st}
		}
		changed, status, err := op(tx, t)
		if err != nil {
			return err
		}
		if changed {
			if err := bumpEditorial(ctx, tx, t.id); err != nil {
				return err
			}
		}
		st, err := loadState(ctx, tx, t.contentID, t.locale)
		if err != nil {
			return err
		}
		res.Status, res.State = status, &st
		return storeIdempotent(ctx, tx, t.id, req.IdempotencyKey, action, hash, status, st)
	})
	return res, err
}

func (s *Service) Publish(ctx context.Context, req Request, version int) (Result, error) {
	return s.mutate(ctx, req, "publish", map[string]any{"revision_version": version, "expected": req.ExpectedEditorialVersion},
		func(tx pgx.Tx, t translation) (bool, int, error) {
			rev, err := revisionByVersion(ctx, tx, t.id, version)
			if err != nil {
				return false, 0, err
			}
			problems, err := validate(ctx, tx, t, rev, nil)
			if err != nil {
				return false, 0, err
			}
			if len(problems) > 0 {
				return false, 0, blockedError{problems}
			}
			changed, err := s.publishLocked(ctx, tx, t, rev)
			if err != nil {
				return false, 0, err
			}
			// v1 decision: a manual publication cancels any pending schedule of this translation.
			cancelled, err := cancelActive(ctx, tx, t.id, "manual_publish", s.Now())
			if err != nil {
				return false, 0, err
			}
			if changed || cancelled > 0 {
				if err := audit(ctx, tx, req.Actor, "publication.publish", t, map[string]any{"revision_id": rev.id, "version": rev.version, "cancelled_jobs": cancelled}); err != nil {
					return false, 0, err
				}
			}
			return changed || cancelled > 0, 200, nil
		})
}

// ParseLocal converts "YYYY-MM-DDTHH:MM" in the editorial zone to an instant.
func ParseLocal(local, zone string) (time.Time, error) {
	if zone != EditorialZone {
		return time.Time{}, invalidError{"time_zone", "la zona editorial es " + EditorialZone}
	}
	at, err := time.ParseInLocation("2006-01-02T15:04", local, editorialLocation)
	if err != nil {
		return time.Time{}, invalidError{"run_at_local", "usa el formato AAAA-MM-DDTHH:MM"}
	}
	return at, nil
}

func (s *Service) Schedule(ctx context.Context, req Request, version int, runAtLocal, zone string, replace bool) (Result, error) {
	runAt, err := ParseLocal(runAtLocal, zone)
	if err != nil {
		return Result{}, err
	}
	return s.mutate(ctx, req, "schedule", map[string]any{"revision_version": version, "run_at_local": runAtLocal, "zone": zone, "replace": replace, "expected": req.ExpectedEditorialVersion},
		func(tx pgx.Tx, t translation) (bool, int, error) {
			now := s.Now()
			if !runAt.After(now) {
				return false, 0, invalidError{"run_at_local", "la fecha ya pasó; usa «Publicar ahora» para publicar de inmediato"}
			}
			rev, err := revisionByVersion(ctx, tx, t.id, version)
			if err != nil {
				return false, 0, err
			}
			var active int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM publication_jobs WHERE translation_id = $1 AND status = 'scheduled'`, t.id).Scan(&active); err != nil {
				return false, 0, err
			}
			if active > 0 && !replace {
				return false, 0, errScheduleExists
			}
			if active > 0 {
				if _, err := cancelActive(ctx, tx, t.id, "replaced", now); err != nil {
					return false, 0, err
				}
			}
			problems, err := validate(ctx, tx, t, rev, &runAt)
			if err != nil {
				return false, 0, err
			}
			if len(problems) > 0 {
				return false, 0, blockedError{problems}
			}
			if err := routeFree(ctx, tx, t, rev.slug); err != nil {
				return false, 0, err
			}
			var jobID string
			if err := tx.QueryRow(ctx, `INSERT INTO publication_jobs (translation_id, revision_id, run_at, next_attempt_at, max_attempts, created_by)
				VALUES ($1, $2, $3, $3, $4, $5) RETURNING id`, t.id, rev.id, runAt, s.MaxAttempts, req.Actor).Scan(&jobID); err != nil {
				return false, 0, err
			}
			if err := audit(ctx, tx, req.Actor, "publication.schedule", t, map[string]any{"revision_id": rev.id, "version": rev.version, "job_id": jobID, "run_at": runAt.UTC(), "replaced": active > 0}); err != nil {
				return false, 0, err
			}
			return true, 200, nil
		})
}

func (s *Service) Cancel(ctx context.Context, req Request) (Result, error) {
	return s.mutate(ctx, req, "cancel", map[string]any{"expected": req.ExpectedEditorialVersion},
		func(tx pgx.Tx, t translation) (bool, int, error) {
			n, err := cancelActive(ctx, tx, t.id, "cancelled", s.Now())
			if err != nil {
				return false, 0, err
			}
			if n == 0 {
				return false, 0, errNoActiveJob
			}
			return true, 200, audit(ctx, tx, req.Actor, "publication.cancel", t, map[string]any{})
		})
}

func (s *Service) Withdraw(ctx context.Context, req Request) (Result, error) {
	return s.mutate(ctx, req, "withdraw", map[string]any{"expected": req.ExpectedEditorialVersion},
		func(tx pgx.Tx, t translation) (bool, int, error) {
			if t.kind == "project" && t.publishedRevision != nil {
				rows, err := tx.Query(ctx, `
					SELECT lc.id::text, COALESCE(r.title, '')
					FROM contents lc JOIN translations lt ON lt.content_id = lc.id
					LEFT JOIN revisions r ON r.id = lt.published_revision_id
					WHERE lc.project_id = $1 AND lt.locale = $2 AND lt.published_revision_id IS NOT NULL
					ORDER BY r.title`, t.contentID, t.locale)
				if err != nil {
					return false, 0, err
				}
				deps, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Dependency, error) {
					var d Dependency
					err := r.Scan(&d.ContentID, &d.Title)
					return d, err
				})
				if err != nil {
					return false, 0, err
				}
				if len(deps) > 0 {
					return false, 0, dependencyError{deps}
				}
			}
			now := s.Now()
			cancelled, err := cancelActive(ctx, tx, t.id, "withdrawn", now)
			if err != nil {
				return false, 0, err
			}
			if t.publishedRevision == nil && cancelled == 0 {
				return false, 0, errNothingToWithdraw
			}
			if t.publishedRevision != nil {
				if _, err := tx.Exec(ctx, `UPDATE translations SET published_revision_id = NULL, published_at = NULL, withdrawn_at = $2, updated_at = now() WHERE id = $1`, t.id, now); err != nil {
					return false, 0, err
				}
			}
			return true, 200, audit(ctx, tx, req.Actor, "publication.withdraw", t, map[string]any{"cancelled_jobs": cancelled, "was_published": t.publishedRevision != nil})
		})
}
