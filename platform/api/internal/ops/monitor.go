package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

// Monitor evaluates the checks periodically and, only if ALERT_WEBHOOK_URL is configured (an
// approved channel), POSTs a JSON summary when the set of problems changes: on a new problem and
// on recovery, never repeatedly. The payload has check names and details only.
type Monitor struct {
	checker *Checker
	url     string
	site    string
	logger  *slog.Logger
	client  *http.Client
	last    string
}

// WebhookURL validates ALERT_WEBHOOK_URL: https (http only for localhost tests), no credentials in
// the log. Empty disables notifications.
func WebhookURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"))) {
		return "", fmt.Errorf("ALERT_WEBHOOK_URL must be an https URL")
	}
	return raw, nil
}

func NewMonitor(c *Checker, webhook, site string, logger *slog.Logger) *Monitor {
	return &Monitor{checker: c, url: webhook, site: site, logger: logger, client: &http.Client{Timeout: 10 * time.Second}}
}

func signature(r Report) string {
	var bad []string
	for _, c := range r.Checks {
		if c.Status != StatusOK {
			bad = append(bad, c.Name+"="+string(c.Status))
		}
	}
	sort.Strings(bad)
	return strings.Join(bad, ",")
}

// Tick runs the checks once, logs problems and notifies on change. Returns whether it notified.
func (m *Monitor) Tick(ctx context.Context) (Report, bool) {
	r := m.checker.Run(ctx)
	sig := signature(r)
	for _, c := range r.Checks {
		if c.Status != StatusOK {
			m.logger.Warn("ops check", "check", c.Name, "status", c.Status, "detail", c.Detail)
		}
	}
	if sig == m.last {
		return r, false
	}
	previous := m.last
	m.last = sig
	if m.url == "" {
		return r, false
	}
	var problems []Check
	for _, c := range r.Checks {
		if c.Status != StatusOK {
			problems = append(problems, c)
		}
	}
	kind := "problem"
	if sig == "" {
		kind = "recovered"
	}
	body, _ := json.Marshal(map[string]any{"site": m.site, "event": kind, "status": r.Status, "checked_at": r.CheckedAt,
		"problems": problems, "previous": previous,
		// Plain text for chat webhooks that only show "text"/"content".
		"text": summary(m.site, kind, r, problems), "content": summary(m.site, kind, r, problems)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.url, bytes.NewReader(body))
	if err != nil {
		return r, false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		m.logger.Error("alert webhook failed", "error", "request failed")
		m.last = previous // retry on the next tick
		return r, false
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		m.logger.Error("alert webhook failed", "status", resp.StatusCode)
		m.last = previous
		return r, false
	}
	return r, true
}

func summary(site, kind string, r Report, problems []Check) string {
	if kind == "recovered" {
		return fmt.Sprintf("BrambiLab %s: todo en orden de nuevo.", site)
	}
	parts := make([]string, len(problems))
	for i, p := range problems {
		parts[i] = fmt.Sprintf("%s [%s]: %s", p.Name, p.Status, p.Detail)
	}
	return fmt.Sprintf("BrambiLab %s (%s): %s", site, r.Status, strings.Join(parts, "; "))
}

func (m *Monitor) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		m.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// AdminHandler serves GET /api/v1/admin/ops for the panel (owner only; no-store via RequireOwner).
type AdminHandler struct {
	checker      *Checker
	pool         *pgxpool.Pool
	requireOwner func(http.Handler) http.Handler
	alerts       bool
}

func NewAdminHandler(c *Checker, pool *pgxpool.Pool, requireOwner func(http.Handler) http.Handler, alertsConfigured bool) *AdminHandler {
	return &AdminHandler{checker: c, pool: pool, requireOwner: requireOwner, alerts: alertsConfigured}
}

type BackupRun struct {
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Status     string     `json:"status"`
	Step       *string    `json:"step"`
	Bytes      *int64     `json:"bytes"`
	Assets     *int       `json:"assets"`
	Error      *string    `json:"error"`
}

func (a *AdminHandler) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/admin/ops", a.requireOwner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		report := a.checker.Run(r.Context())
		rows, err := a.pool.Query(r.Context(), `SELECT started_at, finished_at, status, step, bytes, assets, error FROM ops_backup_runs ORDER BY id DESC LIMIT 10`)
		runs := []BackupRun{}
		if err == nil {
			runs, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (BackupRun, error) {
				var b BackupRun
				err := row.Scan(&b.StartedAt, &b.FinishedAt, &b.Status, &b.Step, &b.Bytes, &b.Assets, &b.Error)
				return b, err
			})
		}
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "internal", "Internal server error")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"report": report, "backups": runs, "alerts_configured": a.alerts})
	})))
}
