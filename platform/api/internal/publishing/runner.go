package publishing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Scheduler execution (web-v1.md §8.1). There is no "running" state: each job executes in one
// transaction that holds the translation and job row locks, which act as the lease.
//   - A crash before commit rolls everything back; the job is still scheduled and runs again.
//   - A crash after commit leaves the job succeeded; it is never repeated.
//   - A second executor waits on the locks and then finds the job resolved.

const (
	// DefaultTick and BatchSize keep a due job under the 60 s target with a healthy API and DB.
	DefaultTick = 15 * time.Second
	BatchSize   = 10
	// SchedulerActor names the scheduler in audit events.
	SchedulerActor = "scheduler"
)

// hooks let tests stop an execution at a precise point (crash, races). Nil in production.
type hooks struct {
	afterLock    func(jobID string)
	beforeCommit func(jobID string) error
}

// Run executes due jobs every tick until ctx ends. The first pass runs immediately, so jobs that
// became due while the process was down are recovered at startup.
func (s *Service) Run(ctx context.Context, tick time.Duration, logger *slog.Logger) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		if done, ok := s.Gate.Try(); ok {
			if n, err := s.RunDue(ctx); err != nil && ctx.Err() == nil {
				logger.Error("scheduler pass", "error", err)
			} else if n > 0 {
				logger.Info("scheduler pass", "jobs", n)
			}
			done()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// RunDue executes at most BatchSize due jobs and returns how many it handled. Projects go
// before logs so a log scheduled at the same time as its project finds it published.
func (s *Service) RunDue(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT j.id FROM publication_jobs j
		JOIN translations t ON t.id = j.translation_id JOIN contents c ON c.id = t.content_id
		WHERE j.status = 'scheduled' AND j.next_attempt_at <= $1
		ORDER BY j.next_attempt_at, c.kind = 'log', j.id LIMIT $2`, s.Now(), BatchSize)
	if err != nil {
		return 0, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return 0, err
	}
	for i, id := range ids {
		if err := s.executeJob(ctx, id); err != nil {
			return i, fmt.Errorf("job %s: %w", id, err)
		}
	}
	return len(ids), nil
}

// terminalError is a failure that retrying cannot fix without the owner (validation, route).
type terminalError struct{ message string }

func (e terminalError) Error() string { return e.message }

// deferral postpones a log until its parent's pending job has run; it is not an attempt.
type deferral struct{ until time.Time }

func (deferral) Error() string { return "waiting for the project's scheduled publication" }

// executeJob runs one job. Validation and route problems end it as terminal; other errors
// (database) are transient and retried with backoff. Outcomes are recorded after a rollback in
// their own transaction, re-locking in the canonical order.
func (s *Service) executeJob(ctx context.Context, jobID string) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, job, ok, err := s.lockJob(ctx, tx, jobID)
		if err != nil || !ok {
			return err
		}
		if s.hooks.afterLock != nil {
			s.hooks.afterLock(jobID)
		}
		rev, err := revisionByID(ctx, tx, job.revisionID)
		if err != nil {
			return err
		}
		problems, err := validate(ctx, tx, t, rev, nil)
		if err != nil {
			return err
		}
		if len(problems) > 0 {
			if until, waiting, err := parentPending(ctx, tx, t, problems); err != nil {
				return err
			} else if waiting {
				return deferral{until}
			}
			return terminalError{describe(problems)}
		}
		if _, err := s.publishLocked(ctx, tx, t, rev); err != nil {
			var taken slugTakenError
			if errors.As(err, &taken) {
				return terminalError{fmt.Sprintf("la ruta «%s» ya la usa otro contenido", taken.slug)}
			}
			return err
		}
		now := s.Now()
		attempt := job.attempts + 1
		if _, err := tx.Exec(ctx, `UPDATE publication_jobs SET status = 'succeeded', attempts = $2, last_error = NULL, error_kind = NULL,
			finished_at = $3, updated_at = $3 WHERE id = $1`, jobID, attempt, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO publication_job_attempts (job_id, attempt, at, outcome) VALUES ($1, $2, $3, 'succeeded')`, jobID, attempt, now); err != nil {
			return err
		}
		if err := bumpEditorial(ctx, tx, t.id); err != nil {
			return err
		}
		if err := audit(ctx, tx, SchedulerActor, "publication.scheduled_publish", t, map[string]any{"job_id": jobID, "revision_id": rev.id, "version": rev.version, "attempt": attempt}); err != nil {
			return err
		}
		if s.hooks.beforeCommit != nil {
			return s.hooks.beforeCommit(jobID)
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err() // shutting down: nothing was committed; the job runs again later
	}
	var term terminalError
	var wait deferral
	switch {
	case errors.As(err, &wait):
		return s.deferJob(ctx, jobID, wait.until)
	case errors.As(err, &term):
		return s.recordFailure(ctx, jobID, term.message, true)
	default:
		return s.recordFailure(ctx, jobID, err.Error(), false)
	}
}

type lockedJob struct {
	revisionID string
	attempts   int
	max        int
}

// lockJob takes the canonical locks and reports whether the job is still due and scheduled.
func (s *Service) lockJob(ctx context.Context, tx pgx.Tx, jobID string) (translation, lockedJob, bool, error) {
	var contentID, locale string
	var job lockedJob
	// Immutable facts, read before locking.
	err := tx.QueryRow(ctx, `SELECT t.content_id, t.locale FROM publication_jobs j JOIN translations t ON t.id = j.translation_id WHERE j.id = $1`, jobID).
		Scan(&contentID, &locale)
	if errors.Is(err, pgx.ErrNoRows) {
		return translation{}, job, false, nil
	}
	if err != nil {
		return translation{}, job, false, err
	}
	t, err := lockTranslation(ctx, tx, contentID, locale)
	if err != nil {
		return t, job, false, err
	}
	var status string
	var next time.Time
	if err := tx.QueryRow(ctx, `SELECT status, next_attempt_at, revision_id, attempts, max_attempts FROM publication_jobs WHERE id = $1 FOR UPDATE`, jobID).
		Scan(&status, &next, &job.revisionID, &job.attempts, &job.max); err != nil {
		return t, job, false, err
	}
	return t, job, status == "scheduled" && !next.After(s.Now()), nil
}

func revisionByID(ctx context.Context, tx pgx.Tx, id string) (revision, error) {
	var r revision
	err := tx.QueryRow(ctx, `SELECT id, version, slug, title FROM revisions WHERE id = $1`, id).Scan(&r.id, &r.version, &r.slug, &r.title)
	return r, err
}

// parentPending reports whether the only problem is a project that still has a pending job; the
// log then waits for it instead of failing.
func parentPending(ctx context.Context, tx pgx.Tx, t translation, problems []Problem) (time.Time, bool, error) {
	if t.projectID == nil || len(problems) != 1 || problems[0].Field != "project" {
		return time.Time{}, false, nil
	}
	var next time.Time
	err := tx.QueryRow(ctx, `SELECT j.next_attempt_at FROM publication_jobs j JOIN translations pt ON pt.id = j.translation_id
		WHERE pt.content_id = $1 AND pt.locale = $2 AND j.status = 'scheduled'`, *t.projectID, t.locale).Scan(&next)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	return next, err == nil, err
}

func describe(problems []Problem) string {
	msgs := make([]string, len(problems))
	for i, p := range problems {
		msgs[i] = p.Message
	}
	return strings.Join(msgs, "; ")
}

// backoff is BaseBackoff × 2^(attempt-1), capped at MaxBackoff.
func (s *Service) backoff(attempt int) time.Duration {
	d := s.BaseBackoff
	for i := 1; i < attempt && d < s.MaxBackoff; i++ {
		d *= 2
	}
	return min(d, s.MaxBackoff)
}

func (s *Service) deferJob(ctx context.Context, jobID string, until time.Time) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, _, ok, err := s.lockJob(ctx, tx, jobID)
		if err != nil || !ok {
			return err
		}
		// Never earlier than a short pause, so a stuck parent cannot make the loop spin.
		until = maxTime(until, s.Now().Add(s.BaseBackoff))
		_, err = tx.Exec(ctx, `UPDATE publication_jobs SET next_attempt_at = $2, updated_at = $3 WHERE id = $1`, jobID, until, s.Now())
		return err
	})
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// recordFailure stores one failed attempt: terminal, or transient with backoff until the
// attempts run out. Every result bumps editorial_version.
func (s *Service) recordFailure(ctx context.Context, jobID, message string, terminal bool) error {
	const maxMessage = 500
	if len(message) > maxMessage {
		message = message[:maxMessage]
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, job, ok, err := s.lockJob(ctx, tx, jobID)
		if err != nil || !ok {
			return err
		}
		now := s.Now()
		attempt := job.attempts + 1
		kind, outcome := "transient", "transient_error"
		if terminal {
			kind, outcome = "terminal", "terminal_error"
		}
		if terminal || attempt >= job.max {
			_, err = tx.Exec(ctx, `UPDATE publication_jobs SET status = 'failed', attempts = $2, last_error = $3, error_kind = $4,
				finished_at = $5, updated_at = $5 WHERE id = $1`, jobID, attempt, message, kind, now)
		} else {
			_, err = tx.Exec(ctx, `UPDATE publication_jobs SET attempts = $2, last_error = $3, error_kind = $4, next_attempt_at = $5,
				updated_at = $6 WHERE id = $1`, jobID, attempt, message, kind, now.Add(s.backoff(attempt)), now)
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO publication_job_attempts (job_id, attempt, at, outcome, error) VALUES ($1, $2, $3, $4, $5)`,
			jobID, attempt, now, outcome, message); err != nil {
			return err
		}
		if err := bumpEditorial(ctx, tx, t.id); err != nil {
			return err
		}
		return audit(ctx, tx, SchedulerActor, "publication.job_failed", t, map[string]any{"job_id": jobID, "attempt": attempt, "kind": kind, "error": message})
	})
}

// Retry puts a failed job back in the queue with the same frozen revision. It revalidates now,
// so a job that would fail again is rejected with the same publish_blocked problems.
func (s *Service) Retry(ctx context.Context, req Request, jobID string) (Result, error) {
	return s.mutate(ctx, req, "retry", map[string]any{"job_id": jobID, "expected": req.ExpectedEditorialVersion},
		func(tx pgx.Tx, t translation) (bool, int, error) {
			var status, revisionID string
			var attempts int
			err := tx.QueryRow(ctx, `SELECT status, revision_id, attempts FROM publication_jobs WHERE id = $1 AND translation_id = $2 FOR UPDATE`, jobID, t.id).
				Scan(&status, &revisionID, &attempts)
			if errors.Is(err, pgx.ErrNoRows) {
				return false, 0, errNotFound
			}
			if err != nil {
				return false, 0, err
			}
			if status != "failed" {
				return false, 0, errJobNotFailed
			}
			var active int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM publication_jobs WHERE translation_id = $1 AND status = 'scheduled'`, t.id).Scan(&active); err != nil {
				return false, 0, err
			}
			if active > 0 {
				return false, 0, errScheduleExists
			}
			rev, err := revisionByID(ctx, tx, revisionID)
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
			if err := routeFree(ctx, tx, t, rev.slug); err != nil {
				return false, 0, err
			}
			now := s.Now()
			// A fresh budget of attempts; the attempt log keeps numbering after the old ones.
			if _, err := tx.Exec(ctx, `UPDATE publication_jobs SET status = 'scheduled', next_attempt_at = $2, max_attempts = attempts + $3,
				last_error = NULL, error_kind = NULL, finished_at = NULL, updated_at = $2 WHERE id = $1`, jobID, now, s.MaxAttempts); err != nil {
				return false, 0, err
			}
			return true, 200, audit(ctx, tx, req.Actor, "publication.retry", t, map[string]any{"job_id": jobID, "previous_attempts": attempts})
		})
}
