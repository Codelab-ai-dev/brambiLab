package ops

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Operational checks (web-v1.md §15.3, WEB-008). Thresholds are explicit and conservative; they
// are starting points to adjust with real measurements, not capacity promises. No check reads or
// reports personal data.

type Status string

const (
	StatusOK       Status = "ok"
	StatusWarn     Status = "warn"
	StatusCritical Status = "critical"
)

type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

type Report struct {
	Status    Status    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
	Checks    []Check   `json:"checks"`
}

type Thresholds struct {
	BackupMaxAge   time.Duration // last successful backup older than this: critical
	DiskWarnBytes  uint64        // free space on the media volume
	DiskCritBytes  uint64
	JobLag         time.Duration // a due job waiting longer than this: the worker is stuck
	MaintenanceMax time.Duration // maintenance active longer than this: critical
	RecentFailures time.Duration // failed jobs within this window: warn
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		BackupMaxAge: 26 * time.Hour, DiskWarnBytes: 2 << 30, DiskCritBytes: 512 << 20,
		JobLag: 5 * time.Minute, MaintenanceMax: 30 * time.Minute, RecentFailures: 7 * 24 * time.Hour,
	}
}

type Checker struct {
	pool           *pgxpool.Pool
	MediaRoot      string
	ContactEnabled bool
	Background     bool // false when BACKGROUND_JOBS=off
	Thresholds     Thresholds
	Now            func() time.Time
	// Free reports free and total bytes of a directory's filesystem (injectable for tests).
	Free func(dir string) (free, total uint64, err error)
}

func NewChecker(pool *pgxpool.Pool, mediaRoot string, contactEnabled, background bool) *Checker {
	return &Checker{pool: pool, MediaRoot: mediaRoot, ContactEnabled: contactEnabled, Background: background,
		Thresholds: DefaultThresholds(), Now: time.Now, Free: statfs}
}

func statfs(dir string) (uint64, uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), uint64(st.Blocks) * uint64(st.Bsize), nil //nolint:unconvert // field types differ per OS
}

func gib(b uint64) string { return fmt.Sprintf("%.1f GiB", float64(b)/(1<<30)) }

// ago is the operator-facing duration (Spanish, like the panel).
func ago(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%.1f h", d.Hours())
	default:
		return fmt.Sprintf("%d días", int(d.Hours()/24))
	}
}

// Run evaluates every check; the report status is the worst one.
func (c *Checker) Run(ctx context.Context) Report {
	now := c.Now()
	r := Report{Status: StatusOK, CheckedAt: now.UTC()}
	add := func(name string, s Status, format string, args ...any) {
		r.Checks = append(r.Checks, Check{name, s, fmt.Sprintf(format, args...)})
		if s == StatusCritical || (s == StatusWarn && r.Status == StatusOK) {
			r.Status = s
		}
	}
	t := c.Thresholds

	if err := c.pool.Ping(ctx); err != nil {
		add("database", StatusCritical, "no responde")
		return r
	}
	add("database", StatusOK, "responde")

	// Backups.
	var lastOK *time.Time
	var lastStatus, lastStep *string
	err := c.pool.QueryRow(ctx, `SELECT (SELECT max(finished_at) FROM ops_backup_runs WHERE status = 'succeeded'),
		(SELECT status FROM ops_backup_runs WHERE status <> 'running' ORDER BY id DESC LIMIT 1),
		(SELECT step FROM ops_backup_runs WHERE status <> 'running' ORDER BY id DESC LIMIT 1)`).Scan(&lastOK, &lastStatus, &lastStep)
	switch {
	case err != nil:
		add("backup", StatusCritical, "no se puede leer el registro de backups")
	case lastStatus != nil && *lastStatus == "failed":
		add("backup", StatusCritical, "el último falló en el paso %s", deref(lastStep))
	case lastOK == nil:
		add("backup", StatusWarn, "ningún backup correcto registrado (¿destino sin configurar?)")
	case now.Sub(*lastOK) > t.BackupMaxAge:
		add("backup", StatusCritical, "último correcto hace %s (límite %s)", ago(now.Sub(*lastOK)), ago(t.BackupMaxAge))
	default:
		add("backup", StatusOK, "último correcto hace %s", ago(now.Sub(*lastOK)))
	}

	// Disk of the media volume (usually the same disk as the database on a single VPS).
	if free, total, err := c.Free(c.MediaRoot); err != nil {
		add("disk", StatusCritical, "no se puede leer el espacio libre del volumen de medios")
	} else {
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(free) / float64(total)
		}
		s := StatusOK
		if free < t.DiskCritBytes {
			s = StatusCritical
		} else if free < t.DiskWarnBytes {
			s = StatusWarn
		}
		add("disk", s, "%s libres de %s (%.0f %%)", gib(free), gib(total), pct)
	}

	// Maintenance and background jobs.
	var active bool
	var since *time.Time
	if err := c.pool.QueryRow(ctx, `SELECT active, activated_at FROM app_maintenance`).Scan(&active, &since); err == nil && active {
		d := time.Duration(0)
		if since != nil {
			d = now.Sub(*since)
		}
		s := StatusWarn
		if d > t.MaintenanceMax {
			s = StatusCritical
		}
		add("maintenance", s, "activo desde hace %s", ago(d))
	} else {
		add("maintenance", StatusOK, "desactivado")
	}
	if !c.Background {
		add("background_jobs", StatusCritical, "BACKGROUND_JOBS=off: no se publica ni se envía nada")
	}

	// Scheduled publications: due but not run means the scheduler is stuck.
	c.jobs(ctx, add, "publication_jobs", `SELECT count(*) FILTER (WHERE status = 'scheduled' AND next_attempt_at < $1),
		count(*) FILTER (WHERE status = 'failed' AND updated_at > $2) FROM publication_jobs`, now, t, active || !c.Background)
	if c.ContactEnabled {
		c.jobs(ctx, add, "contact_jobs", `SELECT count(*) FILTER (WHERE status IN ('pending', 'retry_wait') AND next_attempt_at < $1),
			count(*) FILTER (WHERE status IN ('failed', 'unknown') AND updated_at > $2) FROM contact_jobs`, now, t, active || !c.Background)
	}
	return r
}

func (c *Checker) jobs(ctx context.Context, add func(string, Status, string, ...any), name, sql string, now time.Time, t Thresholds, paused bool) {
	var lagging, failed int
	if err := c.pool.QueryRow(ctx, sql, now.Add(-t.JobLag), now.Add(-t.RecentFailures)).Scan(&lagging, &failed); err != nil {
		if err == pgx.ErrNoRows {
			return
		}
		add(name, StatusCritical, "no se puede leer")
		return
	}
	switch {
	case lagging > 0 && !paused:
		add(name, StatusCritical, "%d vencidas hace más de %s (¿ejecutor atascado?)", lagging, ago(t.JobLag))
	case failed > 0:
		add(name, StatusWarn, "%d fallidas o inciertas en los últimos %s: revísalas en el panel", failed, ago(t.RecentFailures))
	case lagging > 0:
		add(name, StatusWarn, "%d vencidas durante la pausa", lagging)
	default:
		add(name, StatusOK, "sin retraso")
	}
}

func deref(s *string) string {
	if s == nil {
		return "?"
	}
	return *s
}
