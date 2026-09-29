package contact

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/ops"
)

const (
	MaxBody       = 32 << 10
	MaxName       = 120
	MaxEmail      = 254
	MinMessage    = 10
	MaxMessage    = 5000
	honeypotField = "bl_hp"
	maxAttempts   = 5
)

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

// Store holds the database side of contact: receipt, limits, jobs and retention.
type Store struct {
	pool *pgxpool.Pool
	cfg  Config
	Now  func() time.Time
	// Gate pauses sending and purge (maintenance, BACKGROUND_JOBS=off).
	Gate ops.Gate

	secretOnce sync.Once
	secret     []byte
	secretErr  error
}

func NewStore(pool *pgxpool.Pool, cfg Config) *Store {
	return &Store{pool: pool, cfg: cfg, Now: time.Now}
}

// hmacSecret is generated once and kept in the database, so buckets survive restarts.
func (s *Store) hmacSecret(ctx context.Context) ([]byte, error) {
	s.secretOnce.Do(func() {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		_, s.secretErr = s.pool.Exec(ctx, `INSERT INTO app_secrets (name, value) VALUES ('contact_rate_hmac', $1) ON CONFLICT (name) DO NOTHING`, b)
		if s.secretErr == nil {
			s.secretErr = s.pool.QueryRow(ctx, `SELECT value FROM app_secrets WHERE name = 'contact_rate_hmac'`).Scan(&s.secret)
		}
	})
	return s.secret, s.secretErr
}

// window returns the start of the fixed window containing now.
func window(now time.Time, size time.Duration) time.Time { return now.UTC().Truncate(size) }

// hit increments a counter and reports whether it stays within limit, with the retry delay.
func hit(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, bucket string, start time.Time, size time.Duration, limit int, now time.Time) (bool, time.Duration, error) {
	var n int
	if err := q.QueryRow(ctx, `INSERT INTO contact_rate (bucket, window_start, count) VALUES ($1, $2, 1)
		ON CONFLICT (bucket, window_start) DO UPDATE SET count = contact_rate.count + 1 RETURNING count`, bucket, start).Scan(&n); err != nil {
		return false, 0, err
	}
	retry := start.Add(size).Sub(now)
	if retry < time.Second {
		retry = time.Second
	}
	return n <= limit, retry, nil
}

// Message is a validated submission.
type Message struct {
	Name, Email, Message, Locale, Key string
}

func (m Message) hash() []byte {
	b, _ := json.Marshal([]string{m.Name, m.Email, m.Message, m.Locale})
	sum := sha256.Sum256(b)
	return sum[:]
}

func singleLine(field, v string, min, max int, errs map[string]string) string {
	v = strings.TrimSpace(v)
	switch n := utf8.RuneCountInString(v); {
	case !utf8.ValidString(v):
		errs[field] = "must be valid UTF-8"
	case n < min || n > max:
		errs[field] = fmt.Sprintf("use %d-%d characters", min, max)
	case strings.IndexFunc(v, unicode.IsControl) >= 0:
		errs[field] = "must be a single line without control characters"
	}
	return v
}

// validate normalizes the raw fields and returns per-field problems.
func validate(raw map[string]string) (Message, map[string]string) {
	errs := map[string]string{}
	m := Message{Locale: raw["locale"], Key: raw["key"]}
	m.Name = singleLine("name", raw["name"], 1, MaxName, errs)
	m.Email = singleLine("email", raw["email"], 3, MaxEmail, errs)
	if errs["email"] == "" {
		if a, err := mail.ParseAddress(m.Email); err != nil || a.Name != "" || a.Address != m.Email {
			errs["email"] = "must be a single plain e-mail address"
		}
	}
	msg := strings.TrimSpace(strings.ReplaceAll(raw["message"], "\r\n", "\n"))
	switch n := utf8.RuneCountInString(msg); {
	case !utf8.ValidString(msg):
		errs["message"] = "must be valid UTF-8"
	case n < MinMessage || n > MaxMessage:
		errs["message"] = fmt.Sprintf("use %d-%d characters", MinMessage, MaxMessage)
	case strings.IndexFunc(msg, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) >= 0:
		errs["message"] = "contains control characters"
	}
	m.Message = msg
	if m.Locale != "es" && m.Locale != "en" {
		errs["locale"] = "must be es or en"
	}
	if !keyPattern.MatchString(m.Key) {
		errs["key"] = "use 16-128 letters, digits, '-' or '_'"
	}
	return m, errs
}

// emailFor freezes the payload sent to the owner (template v1).
func emailFor(cfg Config, id string, m Message, received time.Time) Email {
	text := fmt.Sprintf("Nuevo mensaje del formulario de contacto de BrambiLab.\n\nId: %s\nIdioma: %s\nRecibido: %s\nNombre: %s\nCorreo: %s\n\n%s\n",
		id, m.Locale, received.UTC().Format(time.RFC3339), m.Name, m.Email, m.Message)
	return Email{From: cfg.From, To: cfg.To, ReplyTo: m.Email, Subject: "Contacto BrambiLab #" + id[:8], Text: text}
}

var errConflict = errors.New("idempotency key used with other content")

type limitedError struct{ retry time.Duration }

func (limitedError) Error() string { return "rate limited" }

// Accept stores the message and its job in one transaction (after the global quota), or reports
// a replay of the same key and content.
func (s *Store) Accept(ctx context.Context, m Message) error {
	now := s.Now()
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var stored []byte
		err := tx.QueryRow(ctx, `SELECT payload_hash FROM contact_messages WHERE client_key = $1`, m.Key).Scan(&stored)
		if err == nil {
			if !hmac.Equal(stored, m.hash()) {
				return errConflict
			}
			return nil // replay: same acknowledgement, no new job
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		ok, retry, err := hit(ctx, tx, "global", window(now, s.cfg.GlobalWindow), s.cfg.GlobalWindow, s.cfg.Global, now)
		if err != nil {
			return err
		}
		if !ok {
			return limitedError{retry}
		}
		var id string
		err = tx.QueryRow(ctx, `INSERT INTO contact_messages (locale, name, email, message, client_key, payload_hash, received_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`, m.Locale, m.Name, m.Email, m.Message, m.Key, m.hash(), now).Scan(&id)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errConflict // a concurrent first use of the same key; the client retries
		}
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(emailFor(s.cfg, id, m, now))
		_, err = tx.Exec(ctx, `INSERT INTO contact_jobs (message_id, payload, template_version, idempotency_key, max_attempts, next_attempt_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6, $6)`, id, payload, TemplateVersion, "contact/"+id, maxAttempts, now)
		return err
	})
}

// Handler serves POST /api/v1/contact and GET /api/v1/public/contact.
type Handler struct {
	store  *Store
	cfg    Config
	logger *slog.Logger
}

func NewHandler(store *Store, cfg Config, logger *slog.Logger) *Handler {
	return &Handler{store: store, cfg: cfg, logger: logger}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/contact", h.receive)
	mux.HandleFunc("GET /api/v1/public/contact", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"available": h.cfg.Enabled, "retention_days": int(h.cfg.Retention.Hours() / 24)})
	})
}

var contactPaths = map[string]string{"es": "/es/contacto", "en": "/en/contact"}

// reply answers JSON clients with status codes, and HTML forms with a 303 back to the contact
// page carrying only a status word (never personal data).
type reply struct {
	w      http.ResponseWriter
	r      *http.Request
	form   bool
	locale string
}

func (rp reply) send(status int, code, message string, fields map[string]string) {
	rp.w.Header().Set("Cache-Control", "no-store")
	if rp.form {
		state := map[int]string{http.StatusAccepted: "recibido", http.StatusUnprocessableEntity: "invalido", http.StatusTooManyRequests: "limite",
			http.StatusServiceUnavailable: "no-disponible", http.StatusConflict: "conflicto"}[status]
		if state == "" {
			state = "error"
		}
		path := contactPaths[rp.locale]
		if path == "" {
			path = contactPaths["es"]
		}
		http.Redirect(rp.w, rp.r, path+"?estado="+state+"#formulario", http.StatusSeeOther)
		return
	}
	if status == http.StatusAccepted {
		httpapi.WriteJSON(rp.w, status, map[string]string{"status": "received"})
		return
	}
	httpapi.WriteJSON(rp.w, status, httpapi.Error{Code: code, Message: message, Fields: fields, RequestID: httpapi.RequestID(rp.r.Context())})
}

var allowedFields = map[string]bool{"name": true, "email": true, "message": true, "locale": true, "key": true, honeypotField: true}

func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	rp := reply{w: w, r: r}
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	switch mediaType {
	case "application/json":
	case "application/x-www-form-urlencoded":
		rp.form = true
	default:
		rp.send(http.StatusUnsupportedMediaType, "unsupported_media_type", "Use application/json or a form post", nil)
		return
	}
	raw, err := readFields(w, r, rp.form)
	rp.locale = raw["locale"]
	if !h.cfg.Enabled {
		rp.send(http.StatusServiceUnavailable, "contact_unavailable", "The contact form is not available", nil)
		return
	}
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		rp.send(http.StatusRequestEntityTooLarge, "payload_too_large", "Request body is too large", nil)
		return
	case err != nil:
		rp.send(http.StatusUnprocessableEntity, "validation_failed", "Request is invalid", map[string]string{"body": err.Error()})
		return
	}
	ctx := r.Context()
	secret, err := h.store.hmacSecret(ctx)
	if err != nil {
		h.fail(rp, "contact secret", err)
		return
	}
	mac := hmac.New(sha256.New, secret)
	now := h.store.Now()
	start := window(now, h.cfg.ClientWindow)
	fmt.Fprintf(mac, "%s|%d", ClientIP(r, h.cfg.TrustedProxies), start.Unix())
	ok, retry, err := hit(ctx, h.store.pool, "client:"+hex.EncodeToString(mac.Sum(nil))[:32], start, h.cfg.ClientWindow, h.cfg.PerClient, now)
	if err != nil {
		h.fail(rp, "contact rate limit", err)
		return
	}
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds()+0.5)))
		rp.send(http.StatusTooManyRequests, "rate_limited", "Too many messages; try again later", nil)
		return
	}
	// Honeypot: same acknowledgement, nothing stored (the only 202 without persistence).
	if raw[honeypotField] != "" {
		rp.send(http.StatusAccepted, "", "", nil)
		return
	}
	m, errs := validate(raw)
	if len(errs) > 0 {
		rp.send(http.StatusUnprocessableEntity, "validation_failed", "Request is invalid", errs)
		return
	}
	var limited limitedError
	switch err := h.store.Accept(ctx, m); {
	case errors.Is(err, errConflict):
		rp.send(http.StatusConflict, "idempotency_conflict", "This key was used for a different message; write it again", nil)
	case errors.As(err, &limited):
		w.Header().Set("Retry-After", strconv.Itoa(int(limited.retry.Seconds()+0.5)))
		rp.send(http.StatusTooManyRequests, "rate_limited", "Too many messages; try again later", nil)
	case err != nil:
		h.fail(rp, "store contact message", err)
	default:
		rp.send(http.StatusAccepted, "", "", nil)
	}
}

// readFields reads a strict JSON object of strings or a form without unknown/repeated keys.
func readFields(w http.ResponseWriter, r *http.Request, form bool) (map[string]string, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBody))
	if err != nil {
		return map[string]string{}, err
	}
	out := map[string]string{}
	if form {
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return out, errors.New("malformed form")
		}
		for k, v := range values {
			if !allowedFields[k] || len(v) != 1 {
				return out, fmt.Errorf("unexpected field %q", k)
			}
			out[k] = v[0]
		}
		return out, nil
	}
	var obj map[string]json.RawMessage
	dec := json.NewDecoder(strings.NewReader(string(body)))
	if err := dec.Decode(&obj); err != nil || dec.More() {
		return out, errors.New("must be one JSON object")
	}
	for k, v := range obj {
		var s string
		if !allowedFields[k] || json.Unmarshal(v, &s) != nil {
			return out, fmt.Errorf("unexpected field %q", k)
		}
		out[k] = s
	}
	return out, nil
}

func (h *Handler) fail(rp reply, op string, err error) {
	// Never log personal data: only the operation and the error.
	h.logger.Error(op, "request_id", httpapi.RequestID(rp.r.Context()), "error", err)
	rp.send(http.StatusInternalServerError, "internal", "Internal server error", nil)
}
