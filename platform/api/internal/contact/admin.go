package contact

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

var (
	idPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	statuses  = map[string]bool{"pending": true, "processing": true, "retry_wait": true, "accepted_by_provider": true, "failed": true, "unknown": true}
)

type Summary struct {
	ID         string    `json:"id"`
	ReceivedAt time.Time `json:"received_at"`
	Locale     string    `json:"locale"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Status     string    `json:"status"`
	Attempts   int       `json:"attempts"`
}

type Attempt struct {
	Attempt        int       `json:"attempt"`
	At             time.Time `json:"at"`
	IdempotencyKey string    `json:"idempotency_key"`
	Outcome        string    `json:"outcome"`
	HTTPStatus     *int      `json:"http_status"`
	ErrorName      *string   `json:"error_name"`
	RequestID      *string   `json:"provider_request_id"`
}

type Detail struct {
	Summary
	Message         string     `json:"message"`
	Version         int        `json:"version"`
	MaxAttempts     int        `json:"max_attempts"`
	NextAttemptAt   *time.Time `json:"next_attempt_at"`
	FirstAttemptAt  *time.Time `json:"first_attempt_at"`
	WindowEndsAt    *time.Time `json:"window_ends_at"`
	Uncertain       bool       `json:"uncertain"`
	ProviderEmailID *string    `json:"provider_email_id"`
	LastError       *string    `json:"last_error"`
	IdempotencyKey  string     `json:"idempotency_key"`
	Retry           string     `json:"retry"` // "none", "same_key" or "new_key_confirm"
	Attempts        []Attempt  `json:"attempt_log"`
}

// retryMode says how the owner may retry: never an accepted job, the same key while a failed
// job is certainly unsent and inside the window, otherwise a new key after confirming.
func retryMode(status string, uncertain bool, first *time.Time, now time.Time, lastError *string) string {
	switch status {
	case "unknown":
		return "new_key_confirm"
	case "failed":
		// The provider refuses this key for this payload: only a new key can go through.
		mismatch := lastError != nil && *lastError == "invalid_idempotent_request"
		if mismatch || uncertain || (first != nil && now.Sub(*first) >= IdempotencyWindow) {
			return "new_key_confirm"
		}
		return "same_key"
	}
	return "none"
}

// AdminHandler serves /api/v1/admin/contact (owner only; no-store via RequireOwner).
type AdminHandler struct {
	store        *Store
	requireOwner func(http.Handler) http.Handler
	actor        func(*http.Request) string
	fail         func(http.ResponseWriter, *http.Request, string, error)
}

func NewAdminHandler(store *Store, h *Handler, requireOwner func(http.Handler) http.Handler, actor func(*http.Request) string) *AdminHandler {
	return &AdminHandler{store: store, requireOwner: requireOwner, actor: actor, fail: func(w http.ResponseWriter, r *http.Request, op string, err error) {
		h.fail(reply{w: w, r: r}, op, err)
	}}
}

func (a *AdminHandler) Register(mux *http.ServeMux) {
	route := func(p string, fn http.HandlerFunc) { mux.Handle(p, a.requireOwner(fn)) }
	route("GET /api/v1/admin/contact/messages", a.list)
	route("GET /api/v1/admin/contact/messages/{id}", a.detail)
	route("POST /api/v1/admin/contact/messages/{id}/retry", a.retry)
}

func invalid(w http.ResponseWriter, r *http.Request, fields map[string]string) {
	httpapi.WriteJSON(w, http.StatusUnprocessableEntity, httpapi.Error{Code: "validation_failed", Message: "Request is invalid", Fields: fields, RequestID: httpapi.RequestID(r.Context())})
}

func (a *AdminHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, size, status := 1, 20, q.Get("status")
	fields := map[string]string{}
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 {
			fields["page"] = "must be a positive integer"
		} else {
			page = n
		}
	}
	if v := q.Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 100 {
			fields["page_size"] = "must be between 1 and 100"
		} else {
			size = n
		}
	}
	if status != "" && !statuses[status] {
		fields["status"] = "unknown status"
	}
	if len(fields) > 0 {
		invalid(w, r, fields)
		return
	}
	rows, err := a.store.pool.Query(r.Context(), `
		SELECT m.id::text, m.received_at, m.locale, m.name, m.email, j.status, j.attempts, count(*) OVER ()
		FROM contact_messages m JOIN contact_jobs j ON j.message_id = m.id
		WHERE $1 = '' OR j.status = $1
		ORDER BY m.received_at DESC, m.id LIMIT $2 OFFSET $3`, status, size, (page-1)*size)
	if err != nil {
		a.fail(w, r, "list contact messages", err)
		return
	}
	defer rows.Close()
	items := []Summary{}
	total := 0
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.ID, &s.ReceivedAt, &s.Locale, &s.Name, &s.Email, &s.Status, &s.Attempts, &total); err != nil {
			a.fail(w, r, "list contact messages", err)
			return
		}
		s.ReceivedAt = s.ReceivedAt.UTC()
		items = append(items, s)
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "page": page, "page_size": size, "total": total})
}

func (a *AdminHandler) load(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, id string) (Detail, error) {
	var d Detail
	err := q.QueryRow(ctx, `
		SELECT m.id::text, m.received_at, m.locale, m.name, m.email, m.message, j.status, j.attempts, j.version, j.max_attempts,
		       CASE WHEN j.status IN ('pending', 'retry_wait') THEN j.next_attempt_at END, j.first_attempt_at, j.uncertain,
		       j.provider_email_id, j.last_error, j.idempotency_key
		FROM contact_messages m JOIN contact_jobs j ON j.message_id = m.id WHERE m.id = $1`, id).
		Scan(&d.ID, &d.ReceivedAt, &d.Locale, &d.Name, &d.Email, &d.Message, &d.Status, &d.Summary.Attempts, &d.Version, &d.MaxAttempts,
			&d.NextAttemptAt, &d.FirstAttemptAt, &d.Uncertain, &d.ProviderEmailID, &d.LastError, &d.IdempotencyKey)
	if err != nil {
		return d, err
	}
	d.ReceivedAt = d.ReceivedAt.UTC()
	if d.FirstAttemptAt != nil {
		end := d.FirstAttemptAt.Add(IdempotencyWindow).UTC()
		d.WindowEndsAt = &end
	}
	d.Retry = retryMode(d.Status, d.Uncertain, d.FirstAttemptAt, a.store.Now(), d.LastError)
	rows, err := q.Query(ctx, `SELECT attempt, at, idempotency_key, outcome, http_status, error_name, provider_request_id
		FROM contact_attempts WHERE message_id = $1 ORDER BY attempt`, id)
	if err != nil {
		return d, err
	}
	d.Attempts, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Attempt, error) {
		var at Attempt
		err := r.Scan(&at.Attempt, &at.At, &at.IdempotencyKey, &at.Outcome, &at.HTTPStatus, &at.ErrorName, &at.RequestID)
		at.At = at.At.UTC()
		return at, err
	})
	return d, err
}

func (a *AdminHandler) detail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	d, err := a.load(r.Context(), a.store.pool, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	if err != nil {
		a.fail(w, r, "contact message", err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, d)
}

var errStale = errors.New("stale version")

func (a *AdminHandler) retry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	var body struct {
		ExpectedVersion  *int `json:"expected_version"`
		ConfirmDuplicate bool `json:"confirm_possible_duplicate"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.ExpectedVersion == nil {
		invalid(w, r, map[string]string{"expected_version": "is required"})
		return
	}
	ctx := r.Context()
	now := a.store.Now()
	var out Detail
	var code, msg string
	err := pgx.BeginFunc(ctx, a.store.pool, func(tx pgx.Tx) error {
		var status, key string
		var version, attempts int
		var uncertain bool
		var first *time.Time
		var lastError *string
		err := tx.QueryRow(ctx, `SELECT status, version, attempts, uncertain, first_attempt_at, idempotency_key, last_error FROM contact_jobs WHERE message_id = $1 FOR UPDATE`, id).
			Scan(&status, &version, &attempts, &uncertain, &first, &key, &lastError)
		if err != nil {
			return err
		}
		if version != *body.ExpectedVersion {
			return errStale
		}
		mode := retryMode(status, uncertain, first, now, lastError)
		switch {
		case mode == "none":
			code, msg = "not_retryable", "Only failed or unknown messages can be retried; accepted ones are never resent"
			return nil
		case mode == "new_key_confirm" && !body.ConfirmDuplicate:
			code, msg = "confirmation_required", "The provider may already have this message; confirm a possible duplicate to send it with a new key"
			return nil
		}
		newKey := key
		if mode == "new_key_confirm" {
			newKey = "contact/" + id + "/r" + strconv.Itoa(attempts)
			if _, err := tx.Exec(ctx, `UPDATE contact_jobs SET first_attempt_at = NULL, uncertain = false WHERE message_id = $1`, id); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE contact_jobs SET status = 'retry_wait', idempotency_key = $2, next_attempt_at = $3, max_attempts = attempts + $4,
			last_error = NULL, finished_at = NULL, version = version + 1, updated_at = $3 WHERE message_id = $1`, id, newKey, now, maxAttempts); err != nil {
			return err
		}
		meta, _ := json.Marshal(map[string]any{"mode": mode, "previous_status": status, "new_key": newKey != key})
		if _, err := tx.Exec(ctx, `INSERT INTO audit_events (actor, action, entity_type, entity_id, metadata) VALUES ($1, 'contact.retry', 'contact', $2, $3)`,
			a.actor(r), id, meta); err != nil {
			return err
		}
		out, err = a.load(ctx, tx, id)
		return err
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		httpapi.WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, errStale):
		httpapi.WriteError(w, r, http.StatusConflict, "version_conflict", "The message changed; reload it")
	case err != nil:
		a.fail(w, r, "retry contact message", err)
	case code != "":
		httpapi.WriteError(w, r, http.StatusConflict, code, msg)
	default:
		httpapi.WriteJSON(w, http.StatusOK, out)
	}
}
