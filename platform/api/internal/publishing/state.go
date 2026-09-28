package publishing

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// State is the editorial state of one translation (OpenAPI EditorialState).
type State struct {
	Status           string     `json:"status"`
	EditorialVersion int        `json:"editorial_version"`
	LatestVersion    int        `json:"latest_version"`
	PublishedVersion *int       `json:"published_version"`
	PublishedAt      *time.Time `json:"published_at"`
	FirstPublishedAt *time.Time `json:"first_published_at"`
	WithdrawnAt      *time.Time `json:"withdrawn_at"`
	Route            *string    `json:"route"`
	ActiveJob        *Job       `json:"active_job"`
	TimeZone         string     `json:"time_zone"`
}

type Job struct {
	ID              string       `json:"id"`
	RevisionVersion int          `json:"revision_version"`
	RunAt           time.Time    `json:"run_at"`
	RunAtLocal      string       `json:"run_at_local"`
	Status          string       `json:"status"`
	Attempts        int          `json:"attempts"`
	MaxAttempts     int          `json:"max_attempts"`
	NextAttemptAt   *time.Time   `json:"next_attempt_at"`
	LastError       *string      `json:"last_error"`
	ErrorKind       *string      `json:"error_kind"`
	CancelReason    *string      `json:"cancel_reason"`
	CreatedAt       time.Time    `json:"created_at"`
	FinishedAt      *time.Time   `json:"finished_at"`
	AttemptLog      []JobAttempt `json:"attempt_log"`
}

type JobAttempt struct {
	Attempt int       `json:"attempt"`
	At      time.Time `json:"at"`
	Outcome string    `json:"outcome"`
	Error   *string   `json:"error"`
}

// utc keeps JSON timestamps in UTC regardless of the server's zone.
func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// LocalTime formats an instant as the editorial wall-clock time.
func LocalTime(t time.Time) string { return t.In(editorialLocation).Format("2006-01-02T15:04") }

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func loadState(ctx context.Context, q querier, contentID, locale string) (State, error) {
	st := State{TimeZone: EditorialZone}
	var translationID string
	err := q.QueryRow(ctx, `
		SELECT t.id, t.editorial_version, t.latest_version, r.version, t.published_at, t.first_published_at, t.withdrawn_at
		FROM translations t LEFT JOIN revisions r ON r.id = t.published_revision_id
		WHERE t.content_id = $1 AND t.locale = $2`, contentID, locale).
		Scan(&translationID, &st.EditorialVersion, &st.LatestVersion, &st.PublishedVersion, &st.PublishedAt, &st.FirstPublishedAt, &st.WithdrawnAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, errNotFound
	}
	if err != nil {
		return st, err
	}
	st.PublishedAt, st.FirstPublishedAt, st.WithdrawnAt = utc(st.PublishedAt), utc(st.FirstPublishedAt), utc(st.WithdrawnAt)
	switch {
	case st.PublishedVersion != nil:
		st.Status = "published"
		route, err := currentRoute(ctx, q, contentID, locale)
		if err != nil {
			return st, err
		}
		st.Route = route
	case st.WithdrawnAt != nil:
		st.Status = "withdrawn"
	default:
		st.Status = "unpublished"
	}
	jobs, err := loadJobs(ctx, q, translationID, `AND j.status = 'scheduled'`, 1, 0)
	if err != nil {
		return st, err
	}
	if len(jobs) > 0 {
		st.ActiveJob = &jobs[0]
	}
	return st, nil
}

// currentRoute builds the public API path of a content's current route (nil if none).
func currentRoute(ctx context.Context, q querier, contentID, locale string) (*string, error) {
	var scope, slug string
	err := q.QueryRow(ctx, `SELECT scope, slug FROM public_routes WHERE content_id = $1 AND locale = $2 AND is_current`, contentID, locale).Scan(&scope, &slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	base := "/api/v1/public/" + locale + "/"
	var path string
	switch {
	case scope == "project":
		path = base + "projects/" + url.PathEscape(slug)
	case scope == "article":
		path = base + "articles/" + url.PathEscape(slug)
	default:
		projectID := strings.TrimPrefix(scope, "log:")
		var projectSlug string
		err := q.QueryRow(ctx, `SELECT slug FROM public_routes WHERE content_id = $1 AND locale = $2 AND is_current`, projectID, locale).Scan(&projectSlug)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // the parent has no route yet; the log is not reachable
		}
		if err != nil {
			return nil, err
		}
		path = base + "projects/" + url.PathEscape(projectSlug) + "/logs/" + url.PathEscape(slug)
	}
	return &path, nil
}

func loadJobs(ctx context.Context, q querier, translationID, filter string, limit, offset int) ([]Job, error) {
	rows, err := q.Query(ctx, `
		SELECT j.id, r.version, j.run_at, j.status, j.attempts, j.max_attempts, j.next_attempt_at, j.last_error,
		       j.error_kind, j.cancel_reason, j.created_at, j.finished_at
		FROM publication_jobs j JOIN revisions r ON r.id = j.revision_id
		WHERE j.translation_id = $1 `+filter+`
		ORDER BY j.created_at DESC, j.id LIMIT $2 OFFSET $3`, translationID, limit, offset)
	if err != nil {
		return nil, err
	}
	jobs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Job, error) {
		var j Job
		err := r.Scan(&j.ID, &j.RevisionVersion, &j.RunAt, &j.Status, &j.Attempts, &j.MaxAttempts, &j.NextAttemptAt,
			&j.LastError, &j.ErrorKind, &j.CancelReason, &j.CreatedAt, &j.FinishedAt)
		j.RunAt, j.CreatedAt = j.RunAt.UTC(), j.CreatedAt.UTC()
		j.NextAttemptAt, j.FinishedAt = utc(j.NextAttemptAt), utc(j.FinishedAt)
		j.RunAtLocal = LocalTime(j.RunAt)
		// Only scheduled jobs have a pending attempt.
		if j.Status != "scheduled" {
			j.NextAttemptAt = nil
		}
		return j, err
	})
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		rows, err := q.Query(ctx, `SELECT attempt, at, outcome, error FROM publication_job_attempts WHERE job_id = $1 ORDER BY attempt`, jobs[i].ID)
		if err != nil {
			return nil, err
		}
		jobs[i].AttemptLog, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (JobAttempt, error) {
			var a JobAttempt
			err := r.Scan(&a.Attempt, &a.At, &a.Outcome, &a.Error)
			a.At = a.At.UTC()
			return a, err
		})
		if err != nil {
			return nil, err
		}
	}
	return jobs, nil
}

// State returns the editorial state outside any mutation.
func (s *Service) State(ctx context.Context, contentID, locale string) (State, error) {
	return loadState(ctx, s.pool, contentID, locale)
}

// Jobs returns one page of the job history (newest first).
func (s *Service) Jobs(ctx context.Context, contentID, locale string, page, size int) ([]Job, int, error) {
	var translationID string
	var total int
	err := s.pool.QueryRow(ctx, `SELECT t.id, (SELECT count(*) FROM publication_jobs WHERE translation_id = t.id)
		FROM translations t WHERE t.content_id = $1 AND t.locale = $2`, contentID, locale).Scan(&translationID, &total)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, errNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	jobs, err := loadJobs(ctx, s.pool, translationID, "", size, (page-1)*size)
	return jobs, total, err
}
