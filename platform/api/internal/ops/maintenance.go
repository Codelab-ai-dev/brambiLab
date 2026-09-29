// Package ops holds operational switches (web-v1.md §15.1, WEB-008).
//
//   - Maintenance (table app_maintenance): toggled by the backup script with SQL. While active the
//     API answers unsafe requests with 503 maintenance and every background job skips its pass.
//     When no write request and no job pass is in flight, the API sets acknowledged_at, so the
//     backup starts only after the system is quiet. A dump taken in maintenance restores in
//     maintenance: the safe state after a restore.
//   - BACKGROUND_JOBS=off (environment): background jobs never run, e.g. in an isolated restore.
package ops

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

const PollInterval = time.Second

type Maintenance struct {
	pool     *pgxpool.Pool
	logger   *slog.Logger
	disabled bool // BACKGROUND_JOBS=off
	active   atomic.Bool
	inflight atomic.Int64
}

func NewMaintenance(pool *pgxpool.Pool, logger *slog.Logger, getenv func(string) string) *Maintenance {
	m := &Maintenance{pool: pool, logger: logger}
	switch strings.ToLower(strings.TrimSpace(getenv("BACKGROUND_JOBS"))) {
	case "off", "false", "0", "no":
		m.disabled = true
	}
	return m
}

// BackgroundDisabled reports BACKGROUND_JOBS=off (fixed for the process lifetime).
func (m *Maintenance) BackgroundDisabled() bool { return m.disabled }

// Active is the last observed maintenance state.
func (m *Maintenance) Active() bool { return m.active.Load() }

// Refresh reads the flag once and acknowledges when quiet. Run calls it every PollInterval.
func (m *Maintenance) Refresh(ctx context.Context) error {
	var active bool
	if err := m.pool.QueryRow(ctx, `SELECT active FROM app_maintenance`).Scan(&active); err != nil {
		return err
	}
	if active != m.active.Swap(active) {
		m.logger.Warn("maintenance mode", "active", active)
	}
	if active && m.inflight.Load() == 0 {
		_, err := m.pool.Exec(ctx, `UPDATE app_maintenance SET acknowledged_at = now() WHERE active AND (acknowledged_at IS NULL OR acknowledged_at < activated_at)`)
		return err
	}
	return nil
}

func (m *Maintenance) Run(ctx context.Context) {
	t := time.NewTicker(PollInterval)
	defer t.Stop()
	for {
		if err := m.Refresh(ctx); err != nil && ctx.Err() == nil {
			m.logger.Error("maintenance poll", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Allow is the gate background loops call before each pass.
func (m *Maintenance) Allow() (done func(), ok bool) {
	if m.disabled || m.active.Load() {
		return func() {}, false
	}
	m.inflight.Add(1)
	if m.active.Load() {
		m.inflight.Add(-1)
		return func() {}, false
	}
	return func() { m.inflight.Add(-1) }, true
}

// Middleware rejects unsafe requests during maintenance and counts writes in flight.
func (m *Maintenance) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpapi.IsSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		m.inflight.Add(1)
		defer m.inflight.Add(-1)
		if m.active.Load() {
			w.Header().Set("Retry-After", strconv.Itoa(60))
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "maintenance", "The site is in maintenance; try again in a minute")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Gate is what background loops hold: nil means "always allowed" (tests, tools).
type Gate func() (done func(), ok bool)

// Try asks the gate before a pass.
func (g Gate) Try() (func(), bool) {
	if g == nil {
		return func() {}, true
	}
	return g()
}
