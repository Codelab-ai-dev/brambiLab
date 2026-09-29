package contact

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	// Resend keeps idempotency keys for 24 h; stay conservatively inside that window.
	IdempotencyWindow = 23 * time.Hour
	Lease             = 60 * time.Second
	WorkerTick        = 10 * time.Second
	workerBatch       = 5
	baseBackoff       = 30 * time.Second
	maxBackoff        = 10 * time.Minute
)

// Worker sends due jobs. It is independent of the editorial scheduler.
type Worker struct {
	store    *Store
	provider Provider
	logger   *slog.Logger
	// Jitter returns a factor in [0.8, 1.2); injectable for exact tests.
	Jitter func() float64
}

func NewWorker(store *Store, provider Provider, logger *slog.Logger) *Worker {
	return &Worker{store: store, provider: provider, logger: logger, Jitter: func() float64 { return 0.8 + rand.Float64()*0.4 }}
}

// Run processes due jobs every tick until ctx ends; the first pass recovers expired leases.
func (w *Worker) Run(ctx context.Context, tick time.Duration) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		if done, ok := w.store.Gate.Try(); ok {
			if n, err := w.RunDue(ctx); err != nil && ctx.Err() == nil {
				w.logger.Error("contact worker pass", "error", err)
			} else if n > 0 {
				w.logger.Info("contact worker pass", "jobs", n)
			}
			if err := w.store.Purge(ctx); err != nil && ctx.Err() == nil {
				w.logger.Error("contact purge", "error", err)
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

type claim struct {
	id, token, key string
	attempt        int
	email          Email
}

// claimDue leases due jobs (and jobs whose lease expired after a crash) with SKIP LOCKED, so two
// workers never take the same job. A job past its window with an uncertain attempt becomes
// unknown here instead of being sent again.
func (s *Store) claimDue(ctx context.Context, limit int) ([]claim, error) {
	now := s.Now()
	var claims []claim
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT message_id::text, status, attempts, max_attempts, first_attempt_at, uncertain
			FROM contact_jobs
			WHERE (status IN ('pending', 'retry_wait') AND next_attempt_at <= $1) OR (status = 'processing' AND lease_until < $1)
			ORDER BY next_attempt_at LIMIT $2 FOR UPDATE SKIP LOCKED`, now, limit)
		if err != nil {
			return err
		}
		type due struct {
			id, status    string
			attempts, max int
			first         *time.Time
			uncertain     bool
		}
		var list []due
		for rows.Next() {
			var d due
			if err := rows.Scan(&d.id, &d.status, &d.attempts, &d.max, &d.first, &d.uncertain); err != nil {
				rows.Close()
				return err
			}
			list = append(list, d)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, d := range list {
			if d.status == "processing" {
				// The worker holding it died: its call may have reached the provider.
				d.uncertain = true
				if _, err := tx.Exec(ctx, `INSERT INTO contact_attempts (message_id, attempt, at, idempotency_key, outcome)
					SELECT message_id, attempts, $2, idempotency_key, 'lease_expired' FROM contact_jobs WHERE message_id = $1
					ON CONFLICT DO NOTHING`, d.id, now); err != nil {
					return err
				}
			}
			outOfWindow := d.first != nil && now.Sub(*d.first) >= IdempotencyWindow
			if d.uncertain && (outOfWindow || d.attempts >= d.max) {
				if _, err := tx.Exec(ctx, `UPDATE contact_jobs SET status = 'unknown', uncertain = true, lease_token = NULL, lease_until = NULL,
					last_error = 'result_unknown', version = version + 1, updated_at = $2, finished_at = $2 WHERE message_id = $1`, d.id, now); err != nil {
					return err
				}
				continue
			}
			if d.attempts >= d.max {
				if _, err := tx.Exec(ctx, `UPDATE contact_jobs SET status = 'failed', lease_token = NULL, lease_until = NULL,
					version = version + 1, updated_at = $2, finished_at = $2 WHERE message_id = $1`, d.id, now); err != nil {
					return err
				}
				continue
			}
			var c claim
			var payload []byte
			if err := tx.QueryRow(ctx, `UPDATE contact_jobs SET status = 'processing', lease_token = gen_random_uuid(), lease_until = $2,
				attempts = attempts + 1, first_attempt_at = COALESCE(first_attempt_at, $3), uncertain = $4, version = version + 1, updated_at = $3
				WHERE message_id = $1 RETURNING message_id::text, lease_token::text, idempotency_key, attempts, payload`,
				d.id, now.Add(Lease), now, d.uncertain).Scan(&c.id, &c.token, &c.key, &c.attempt, &payload); err != nil {
				return err
			}
			if err := json.Unmarshal(payload, &c.email); err != nil {
				return err
			}
			claims = append(claims, c)
		}
		return nil
	})
	return claims, err
}

// RunDue claims and sends a batch; returns how many jobs were handled.
func (w *Worker) RunDue(ctx context.Context) (int, error) {
	claims, err := w.store.claimDue(ctx, workerBatch)
	if err != nil {
		return 0, err
	}
	for _, c := range claims {
		// The HTTP call runs outside any transaction; its timeout is below the lease.
		res := w.provider.Send(ctx, c.email, c.key)
		if ctx.Err() != nil {
			return 0, ctx.Err() // shutting down: the lease expires and recovery treats it as uncertain
		}
		if err := w.store.finish(ctx, c, res, w.Jitter()); err != nil {
			return 0, err
		}
	}
	return len(claims), nil
}

func backoff(attempt int, jitter float64) time.Duration {
	d := baseBackoff
	for i := 1; i < attempt && d < maxBackoff; i++ {
		d *= 2
	}
	return time.Duration(float64(min(d, maxBackoff)) * jitter)
}

// finish records the result only if this worker still holds the lease (its token).
func (s *Store) finish(ctx context.Context, c claim, res Result, jitter float64) error {
	now := s.Now()
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var maxAtt int
		var first time.Time
		var uncertain bool
		err := tx.QueryRow(ctx, `SELECT max_attempts, first_attempt_at, uncertain FROM contact_jobs
			WHERE message_id = $1 AND status = 'processing' AND lease_token = $2 FOR UPDATE`, c.id, c.token).Scan(&maxAtt, &first, &uncertain)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // lease lost: another worker owns the job now (same key, same payload)
		}
		if err != nil {
			return err
		}
		outcome := string(res.Outcome)
		if _, err := tx.Exec(ctx, `INSERT INTO contact_attempts (message_id, attempt, at, idempotency_key, outcome, http_status, error_name, provider_request_id)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, 0), NULLIF($7, ''), NULLIF($8, ''))`, c.id, c.attempt, now, c.key, outcome, res.HTTPStatus, res.ErrorName, res.RequestID); err != nil {
			return err
		}
		errName := res.ErrorName
		if errName == "" && res.HTTPStatus != 0 {
			errName = "http_" + itoa(res.HTTPStatus)
		}
		set := func(sql string, args ...any) error {
			_, err := tx.Exec(ctx, `UPDATE contact_jobs SET lease_token = NULL, lease_until = NULL, version = version + 1, updated_at = $2, `+sql+` WHERE message_id = $1`,
				append([]any{c.id, now}, args...)...)
			return err
		}
		switch res.Outcome {
		case Accepted:
			return set(`status = 'accepted_by_provider', provider_email_id = $3, last_error = NULL, finished_at = $2`, res.EmailID)
		case Permanent:
			if uncertain {
				// An earlier attempt may have been sent: never call it plainly failed.
				return set(`status = 'unknown', last_error = $3, finished_at = $2`, errName)
			}
			return set(`status = 'failed', last_error = $3, finished_at = $2`, errName)
		}
		uncertain = uncertain || res.Outcome == Uncertain
		next := now.Add(backoff(c.attempt, jitter))
		if res.RetryAfter > 0 && now.Add(res.RetryAfter).After(next) {
			next = now.Add(res.RetryAfter)
		}
		switch {
		case uncertain && (c.attempt >= maxAtt || next.Sub(first) >= IdempotencyWindow):
			return set(`status = 'unknown', uncertain = true, last_error = $3, finished_at = $2`, errName)
		case c.attempt >= maxAtt:
			return set(`status = 'failed', last_error = $3, finished_at = $2`, errName)
		default:
			return set(`status = 'retry_wait', uncertain = $3, next_attempt_at = $4, last_error = $5`, uncertain, next, errName)
		}
	})
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Purge deletes messages past retention with their job, attempts and idempotency data, except
// jobs currently leased (the next pass takes them). Rate counters expire after 24 h.
func (s *Store) Purge(ctx context.Context) error {
	now := s.Now()
	if _, err := s.pool.Exec(ctx, `DELETE FROM contact_rate WHERE window_start < $1`, now.Add(-24*time.Hour)); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM contact_messages m
		WHERE m.received_at < $1
		  AND m.id IN (SELECT j.message_id FROM contact_jobs j WHERE j.message_id = m.id
		               AND NOT (j.status = 'processing' AND j.lease_until >= $2) FOR UPDATE SKIP LOCKED)`,
		now.Add(-s.cfg.Retention), now)
	return err
}

// RunPurge applies retention while the worker is not running (contact disabled).
func (s *Store) RunPurge(ctx context.Context, every time.Duration, logger *slog.Logger) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if done, ok := s.Gate.Try(); ok {
			if err := s.Purge(ctx); err != nil && ctx.Err() == nil {
				logger.Error("contact purge", "error", err)
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
