package publishing_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/authtest"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/publishing"
)

// runAt moves the clock and runs one scheduler pass on svc (e.svc when nil).
func (e *env) runAt(t *testing.T, svc *publishing.Service, at time.Time) int {
	t.Helper()
	e.setNow(at)
	if svc == nil {
		svc = e.svc
	}
	n, err := svc.RunDue(context.Background())
	if err != nil {
		t.Fatalf("RunDue: %v", err)
	}
	return n
}

// executor is a second scheduler sharing the database and the test clock.
func (e *env) executor() *publishing.Service {
	s := publishing.NewService(e.pool)
	s.Now = e.svc.Now
	return s
}

type jobRow struct {
	Status, ErrorKind, LastError string
	Attempts, AttemptRows        int
	NextAttemptAt                time.Time
}

func (e *env) job(t *testing.T, contentID string) jobRow {
	t.Helper()
	var j jobRow
	if err := e.pool.QueryRow(context.Background(), `
		SELECT j.status, COALESCE(j.error_kind, ''), COALESCE(j.last_error, ''), j.attempts, j.next_attempt_at,
		       (SELECT count(*) FROM publication_job_attempts a WHERE a.job_id = j.id)
		FROM publication_jobs j JOIN translations t ON t.id = j.translation_id
		WHERE t.content_id = $1 ORDER BY j.created_at DESC LIMIT 1`, contentID).
		Scan(&j.Status, &j.ErrorKind, &j.LastError, &j.Attempts, &j.NextAttemptAt, &j.AttemptRows); err != nil {
		t.Fatal(err)
	}
	return j
}

func (e *env) jobID(t *testing.T, contentID string) string {
	t.Helper()
	var id string
	if err := e.pool.QueryRow(context.Background(), `SELECT j.id FROM publication_jobs j JOIN translations t ON t.id = j.translation_id
		WHERE t.content_id = $1 ORDER BY j.created_at DESC LIMIT 1`, contentID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

var (
	base   = time.Date(2026, 9, 28, 16, 0, 0, 0, time.UTC)  // start() clock: 10:00 Mexico City
	due    = time.Date(2026, 10, 1, 15, 30, 0, 0, time.UTC) // 2026-10-01T09:30 Mexico City
	dueLoc = "2026-10-01T09:30"
)

func TestScheduledJobPublishesTheFrozenRevisionOnTime(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	v1 := e.save(t, a, "es", snap("Plan", "plan"))
	requireStatus(t, "schedule", e.schedule(t, a, "es", v1, dueLoc, false), http.StatusOK, "")
	e.save(t, a, "es", snap("Plan borrador", "plan")) // newer draft after scheduling

	if n := e.runAt(t, nil, due.Add(-time.Second)); n != 0 {
		t.Fatalf("ran %d jobs before their time", n)
	}
	e.requireGone(t, "/api/v1/public/es/articles/plan")
	before := e.state(t, a, "es").EditorialVersion

	if n := e.runAt(t, nil, due); n != 1 {
		t.Fatalf("due pass ran %d jobs", n)
	}
	if got := e.publicTitle(t, "/api/v1/public/es/articles/plan"); got != "Plan" {
		t.Fatalf("published %q, want the frozen revision", got)
	}
	st := e.state(t, a, "es")
	if st.Status != "published" || *st.PublishedVersion != v1 || st.ActiveJob != nil || st.EditorialVersion != before+1 || *st.PublishedAt != "2026-10-01T15:30:00Z" {
		t.Fatalf("after run %+v", st)
	}
	if j := e.job(t, a); j.Status != "succeeded" || j.Attempts != 1 || j.AttemptRows != 1 {
		t.Fatalf("job %+v", j)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_events WHERE entity_id = $1 AND actor = 'scheduler' AND action = 'publication.scheduled_publish'`, a); n != 1 {
		t.Fatalf("scheduler audit events = %d", n)
	}
	// A later pass does nothing.
	if n := e.runAt(t, nil, due.Add(time.Hour)); n != 0 {
		t.Fatalf("succeeded job ran again: %d", n)
	}
}

func TestProjectAndLogAtTheSameTime(t *testing.T) {
	e := start(t)
	p := e.create(t, "project", "es", "")
	l := e.create(t, "log", "es", p)
	// The log is created first so that ordering by id alone would run it before its project.
	requireStatus(t, "project", e.schedule(t, p, "es", e.save(t, p, "es", snap("Rover", "rover")), dueLoc, false), http.StatusOK, "")
	requireStatus(t, "log", e.schedule(t, l, "es", e.save(t, l, "es", snap("Día 1", "dia-1")), dueLoc, false), http.StatusOK, "")
	if n := e.runAt(t, nil, due); n != 2 {
		t.Fatalf("ran %d", n)
	}
	if got := e.publicTitle(t, "/api/v1/public/es/projects/rover/logs/dia-1"); got != "Día 1" {
		t.Fatalf("log %q", got)
	}
}

func TestLogWaitsForAProjectInBackoffAndFailsWithoutOne(t *testing.T) {
	e := start(t)
	p := e.create(t, "project", "es", "")
	l := e.create(t, "log", "es", p)
	requireStatus(t, "project", e.schedule(t, p, "es", e.save(t, p, "es", snap("Rover", "rover")), dueLoc, false), http.StatusOK, "")
	requireStatus(t, "log", e.schedule(t, l, "es", e.save(t, l, "es", snap("Día 1", "dia-1")), dueLoc, false), http.StatusOK, "")

	// The project's attempt fails transiently in the same pass; the log must wait, not fail.
	publishing.SetBeforeCommit(e.svc, func(string) error { return errors.New("database hiccup") })
	if n := e.runAt(t, nil, due); n != 2 {
		t.Fatalf("ran %d", n)
	}
	publishing.SetBeforeCommit(e.svc, nil)
	pj, lj := e.job(t, p), e.job(t, l)
	if pj.Status != "scheduled" || pj.Attempts != 1 {
		t.Fatalf("project job %+v", pj)
	}
	if lj.Status != "scheduled" || lj.Attempts != 0 || lj.AttemptRows != 0 || lj.NextAttemptAt.Before(pj.NextAttemptAt) {
		t.Fatalf("log should wait for its project %+v (project %+v)", lj, pj)
	}
	// Once the project is published, the log follows in the same pass.
	e.runAt(t, nil, pj.NextAttemptAt)
	if got := e.publicTitle(t, "/api/v1/public/es/projects/rover/logs/dia-1"); got != "Día 1" {
		t.Fatalf("log %q", got)
	}

	// A log whose project is not published nor scheduled fails as terminal, with the reason.
	q := e.create(t, "project", "es", "")
	m := e.create(t, "log", "es", q)
	requireStatus(t, "q", e.schedule(t, q, "es", e.save(t, q, "es", snap("Brazo", "brazo")), "2026-10-02T09:30", false), http.StatusOK, "")
	requireStatus(t, "m", e.schedule(t, m, "es", e.save(t, m, "es", snap("Día 1", "dia-1")), "2026-10-02T09:30", false), http.StatusOK, "")
	requireStatus(t, "cancel project", e.simple(t, q, "es", "cancel"), http.StatusOK, "")
	e.runAt(t, nil, due.Add(24*time.Hour))
	if j := e.job(t, m); j.Status != "failed" || j.ErrorKind != "terminal" || !strings.Contains(j.LastError, "proyecto") {
		t.Fatalf("orphan log job %+v", j)
	}
}

func TestTerminalFailureAndExplicitRetry(t *testing.T) {
	e := start(t)
	img := e.upload(t, "text.png", "placa.png")
	e.setAsset(t, img, true, false)
	a := e.create(t, "article", "es", "")
	v := e.save(t, a, "es", snap("Placa", "placa", map[string]any{"type": "image", "attrs": map[string]any{"assetId": img, "alt": "Placa"}}))
	requireStatus(t, "schedule", e.schedule(t, a, "es", v, dueLoc, false), http.StatusOK, "")
	e.setAsset(t, img, false, false) // becomes private after scheduling: revalidated at run time

	e.runAt(t, nil, due)
	j := e.job(t, a)
	if j.Status != "failed" || j.ErrorKind != "terminal" || j.Attempts != 1 || !strings.Contains(j.LastError, "placa.png") {
		t.Fatalf("job %+v", j)
	}
	e.requireGone(t, "/api/v1/public/es/articles/placa")
	st := e.state(t, a, "es")
	if st.Status != "unpublished" || st.ActiveJob != nil {
		t.Fatalf("state %+v", st)
	}

	id := e.jobID(t, a)
	retry := func() authtest.Response { return e.simple(t, a, "es", "jobs/"+id+"/retry") }
	blocked := requireStatus(t, "retry still blocked", retry(), http.StatusUnprocessableEntity, "publish_blocked")
	if blocked.Fields["assets."+img] == "" {
		t.Fatalf("problems %+v", blocked.Fields)
	}
	e.setAsset(t, img, true, false)
	r := retry()
	requireStatus(t, "retry", r, http.StatusOK, "")
	if st := decode[state](t, r); st.ActiveJob == nil || st.ActiveJob.ID != id || st.ActiveJob.RevisionVersion != v {
		t.Fatalf("retried state %+v", st)
	}
	requireStatus(t, "retry scheduled", retry(), http.StatusConflict, "job_not_failed")
	e.runAt(t, nil, due.Add(time.Minute))
	if got := e.publicTitle(t, "/api/v1/public/es/articles/placa"); got != "Placa" {
		t.Fatalf("after retry %q", got)
	}
	if j := e.job(t, a); j.Status != "succeeded" || j.Attempts != 2 || j.AttemptRows != 2 {
		t.Fatalf("job after retry %+v", j)
	}
	requireStatus(t, "unknown job", e.simple(t, a, "es", "jobs/00000000-0000-4000-8000-000000000000/retry"), http.StatusNotFound, "not_found")
}

func TestTransientFailuresBackOffThenFail(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	requireStatus(t, "schedule", e.schedule(t, a, "es", e.save(t, a, "es", snap("Plan", "plan")), dueLoc, false), http.StatusOK, "")
	publishing.SetBeforeCommit(e.svc, func(string) error { return errors.New("connection reset") })

	now := due
	for i, wait := range []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute} {
		e.runAt(t, nil, now)
		j := e.job(t, a)
		if j.Status != "scheduled" || j.ErrorKind != "transient" || j.Attempts != i+1 || !j.NextAttemptAt.Equal(now.Add(wait)) {
			t.Fatalf("attempt %d: %+v, want next at %v", i+1, j, now.Add(wait))
		}
		// Not retried before the backoff expires.
		if n := e.runAt(t, nil, now.Add(wait-time.Second)); n != 0 {
			t.Fatalf("attempt %d retried early", i+1)
		}
		now = now.Add(wait)
	}
	e.runAt(t, nil, now)
	if j := e.job(t, a); j.Status != "failed" || j.ErrorKind != "transient" || j.Attempts != 5 || j.AttemptRows != 5 {
		t.Fatalf("after max attempts %+v", j)
	}
	// Nothing leaked from the rolled-back attempts.
	e.requireGone(t, "/api/v1/public/es/articles/plan")
	if n := e.count(t, `SELECT count(*) FROM public_routes`); n != 0 {
		t.Fatalf("routes from rolled-back attempts = %d", n)
	}
	if st := e.state(t, a, "es"); st.EditorialVersion != 1+5 {
		t.Fatalf("each result bumps editorial_version: %+v", st)
	}
}

func TestBackoffIsCapped(t *testing.T) {
	e := start(t)
	e.svc.MaxAttempts = 8
	a := e.create(t, "article", "es", "")
	requireStatus(t, "schedule", e.schedule(t, a, "es", e.save(t, a, "es", snap("Plan", "plan")), dueLoc, false), http.StatusOK, "")
	publishing.SetBeforeCommit(e.svc, func(string) error { return errors.New("down") })
	now := due
	var last time.Duration
	for range 7 {
		e.runAt(t, nil, now)
		j := e.job(t, a)
		last = j.NextAttemptAt.Sub(now)
		now = j.NextAttemptAt
	}
	if last != 10*time.Minute {
		t.Fatalf("last backoff %v, want the 10 min cap", last)
	}
}

// A crash before commit leaves no trace; the next process publishes the job exactly once.
func TestCrashBeforeCommitIsRecovered(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	requireStatus(t, "schedule", e.schedule(t, a, "es", e.save(t, a, "es", snap("Plan", "plan")), dueLoc, false), http.StatusOK, "")

	crashing := e.executor()
	ctx, kill := context.WithCancel(context.Background())
	publishing.SetBeforeCommit(crashing, func(string) error {
		kill() // the process dies while holding the transaction
		<-ctx.Done()
		return ctx.Err()
	})
	e.setNow(due)
	if _, err := crashing.RunDue(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("crashed pass: %v", err)
	}
	if j := e.job(t, a); j.Status != "scheduled" || j.Attempts != 0 || j.AttemptRows != 0 {
		t.Fatalf("after crash %+v", j)
	}
	e.requireGone(t, "/api/v1/public/es/articles/plan")

	// "Restart": a fresh executor recovers it.
	if n := e.runAt(t, e.executor(), due.Add(15*time.Second)); n != 1 {
		t.Fatalf("recovery ran %d", n)
	}
	if got := e.publicTitle(t, "/api/v1/public/es/articles/plan"); got != "Plan" {
		t.Fatalf("recovered %q", got)
	}
	if j := e.job(t, a); j.Status != "succeeded" || j.Attempts != 1 || j.AttemptRows != 1 {
		t.Fatalf("recovered job %+v", j)
	}
}

// Two executors on the same due jobs: every job runs exactly once.
func TestTwoExecutorsNeverRunAJobTwice(t *testing.T) {
	e := start(t)
	ids := make([]string, 8)
	for i := range ids {
		ids[i] = e.create(t, "article", "es", "")
		slug := "nota-" + string(rune('a'+i))
		requireStatus(t, "schedule", e.schedule(t, ids[i], "es", e.save(t, ids[i], "es", snap("Nota", slug)), dueLoc, false), http.StatusOK, "")
	}
	e.setNow(due)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, svc := range []*publishing.Service{e.executor(), e.executor()} {
		wg.Go(func() { _, errs[i] = svc.RunDue(context.Background()) })
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM publication_jobs WHERE status = 'succeeded'`); n != len(ids) {
		t.Fatalf("succeeded = %d", n)
	}
	if n := e.count(t, `SELECT count(*) FROM publication_job_attempts`); n != len(ids) {
		t.Fatalf("attempts = %d, want one per job", n)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_events WHERE action = 'publication.scheduled_publish'`); n != len(ids) {
		t.Fatalf("audit events = %d", n)
	}

	// Failing executors: a job one of them already moved into backoff is not retried early by
	// the other, which selected it before (one attempt per job).
	later := make([]string, 8)
	for i := range later {
		later[i] = e.create(t, "article", "es", "")
		requireStatus(t, "schedule", e.schedule(t, later[i], "es", e.save(t, later[i], "es", snap("Nota", "otra-"+string(rune('a'+i)))), "2026-10-02T09:30", false), http.StatusOK, "")
	}
	e.setNow(due.Add(24 * time.Hour))
	for i, svc := range []*publishing.Service{e.executor(), e.executor()} {
		publishing.SetBeforeCommit(svc, func(string) error { return errors.New("down") })
		wg.Go(func() { _, errs[i] = svc.RunDue(context.Background()) })
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM publication_jobs WHERE status = 'scheduled' AND attempts = 1`); n != len(later) {
		t.Fatalf("jobs with exactly one attempt = %d", n)
	}
}

// An executor holding the job finishes first; a withdrawal waiting on the same locks then sees
// the new state (stale editorial version) instead of being lost.
func TestRunningJobSerializesWithWithdraw(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	requireStatus(t, "schedule", e.schedule(t, a, "es", e.save(t, a, "es", snap("Plan", "plan")), dueLoc, false), http.StatusOK, "")
	expected := e.state(t, a, "es").EditorialVersion

	locked, release := make(chan struct{}), make(chan struct{})
	runner := e.executor()
	publishing.SetAfterLock(runner, func(string) { close(locked); <-release })
	e.setNow(due)
	done := make(chan error, 1)
	go func() { _, err := runner.RunDue(context.Background()); done <- err }()
	<-locked

	withdrawn := make(chan authtest.Response, 1)
	go func() {
		withdrawn <- e.act(a, "es", "withdraw", map[string]any{"expected_editorial_version": expected})
	}()
	e.waitForLockWaiter(t) // the withdrawal is blocked behind the running job
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, "stale withdraw", <-withdrawn, http.StatusConflict, "editorial_conflict")
	if got := e.publicTitle(t, "/api/v1/public/es/articles/plan"); got != "Plan" {
		t.Fatalf("published %q", got)
	}
}

// A cancellation, withdrawal or manual publication committed while an executor waits on the
// translation lock wins: the executor re-reads the job under the lock and never resurrects it.
func TestNoResurrectionAfterCancelWithdrawOrManualPublish(t *testing.T) {
	for action, sql := range map[string]string{
		"cancel": `UPDATE publication_jobs SET status = 'cancelled', cancel_reason = 'cancelled' WHERE status = 'scheduled'`,
		"withdraw": `UPDATE publication_jobs SET status = 'cancelled', cancel_reason = 'withdrawn' WHERE status = 'scheduled';
			UPDATE translations SET published_revision_id = NULL, published_at = NULL, withdrawn_at = now()`,
		"publish": `UPDATE publication_jobs SET status = 'cancelled', cancel_reason = 'manual_publish' WHERE status = 'scheduled'`,
	} {
		t.Run(action, func(t *testing.T) {
			e := start(t)
			a := e.create(t, "article", "es", "")
			v1 := e.save(t, a, "es", snap("Plan", "plan"))
			e.publish(t, a, "es", v1)
			v2 := e.save(t, a, "es", snap("Plan programado", "plan"))
			requireStatus(t, "schedule", e.schedule(t, a, "es", v2, dueLoc, false), http.StatusOK, "")

			// Hold the translation lock as the API does, start the executor, then commit the action.
			ctx := context.Background()
			tx, err := e.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
			if _, err := tx.Exec(ctx, `SELECT 1 FROM translations WHERE content_id = $1 FOR UPDATE`, a); err != nil {
				t.Fatal(err)
			}
			e.setNow(due)
			done := make(chan error, 1)
			go func() { _, err := e.executor().RunDue(ctx); done <- err }()
			e.waitForLockWaiter(t)
			if _, err := tx.Exec(ctx, sql); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			st := e.state(t, a, "es")
			if j := e.job(t, a); j.Status != "cancelled" || j.AttemptRows != 0 || (st.PublishedVersion != nil && *st.PublishedVersion == v2) {
				t.Fatalf("resurrected: job %+v state %+v", j, st)
			}
		})
	}
}

// waitForLockWaiter blocks until some backend waits on a row lock (the executor).
func (e *env) waitForLockWaiter(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if e.count(t, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the executor never waited on the lock")
}

// The loop runs a pass at startup (recovery of jobs that became due while down) and then on
// every tick.
func TestRunLoopRecoversAtStartupAndTicks(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	b := e.create(t, "article", "es", "")
	requireStatus(t, "a", e.schedule(t, a, "es", e.save(t, a, "es", snap("A", "a")), dueLoc, false), http.StatusOK, "")
	requireStatus(t, "b", e.schedule(t, b, "es", e.save(t, b, "es", snap("B", "b")), "2026-10-01T09:31", false), http.StatusOK, "")
	e.setNow(due.Add(10 * time.Second)) // a is overdue when the process starts

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.executor().Run(ctx, 20*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	t.Cleanup(func() { stop(); <-done })

	waitFor := func(id string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for e.job(t, id).Status != "succeeded" {
			if time.Now().After(deadline) {
				t.Fatalf("job of %s not run", id)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitFor(a)
	if j := e.job(t, b); j.Status != "scheduled" {
		t.Fatalf("b ran early: %+v", j)
	}
	e.setNow(due.Add(time.Minute))
	waitFor(b)
}

// Maintenance or BACKGROUND_JOBS=off: the loop skips its passes; due jobs wait, then run.
func TestRunLoopRespectsTheGate(t *testing.T) {
	e := start(t)
	a := e.create(t, "article", "es", "")
	requireStatus(t, "a", e.schedule(t, a, "es", e.save(t, a, "es", snap("A", "a")), dueLoc, false), http.StatusOK, "")
	e.setNow(due.Add(time.Minute))
	var open atomic.Bool
	runner := e.executor()
	runner.Gate = func() (func(), bool) { return func() {}, open.Load() }
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runner.Run(ctx, 20*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	t.Cleanup(func() { stop(); <-done })
	time.Sleep(200 * time.Millisecond)
	if j := e.job(t, a); j.Status != "scheduled" {
		t.Fatalf("ran while paused: %+v", j)
	}
	open.Store(true)
	deadline := time.Now().Add(5 * time.Second)
	for e.job(t, a).Status != "succeeded" {
		if time.Now().After(deadline) {
			t.Fatal("did not run after the gate opened")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
