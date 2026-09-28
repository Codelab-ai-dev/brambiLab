package contact_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/authtest"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/contact"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/fakeresend"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

type env struct {
	*authtest.Server
	pool   *pgxpool.Pool
	store  *contact.Store
	fake   *fakeresend.Server
	resend *contact.Resend
	now    atomic.Pointer[time.Time]
}

var t0 = time.Date(2026, 9, 28, 16, 0, 0, 0, time.UTC)

func enabledEnv(extra map[string]string) func(string) string {
	m := map[string]string{"CONTACT_ENABLED": "true", "RESEND_API_KEY": "re_test_key", "CONTACT_FROM": "BrambiLab <contacto@example.com>", "CONTACT_TO": "owner@example.com"}
	for k, v := range extra {
		m[k] = v
	}
	return func(k string) string { return m[k] }
}

func start(t *testing.T, getenv func(string) string) *env {
	t.Helper()
	pool := testdb.New(t)
	fake := fakeresend.New("re_test_key")
	fs := httptest.NewServer(fake.Handler())
	t.Cleanup(fs.Close)
	cfg := contact.LoadConfig(func(k string) string {
		if k == "RESEND_API_URL" {
			return fs.URL
		}
		return getenv(k)
	})
	e := &env{pool: pool, fake: fake, store: contact.NewStore(pool, cfg)}
	e.setNow(t0)
	e.store.Now = func() time.Time { return *e.now.Load() }
	e.resend = contact.NewResend(cfg.APIURL, cfg.APIKey)
	e.resend.SetTimeout(300 * time.Millisecond)
	e.Server = authtest.Start(t, pool, func(a *auth.Handler, logger *slog.Logger) []httpapi.Module {
		h := contact.NewHandler(e.store, cfg, logger)
		return []httpapi.Module{h, contact.NewAdminHandler(e.store, h, a.RequireOwner, auth.Actor)}
	})
	return e
}

func (e *env) setNow(t time.Time)      { e.now.Store(&t) }
func (e *env) advance(d time.Duration) { e.setNow(e.now.Load().Add(d)) }

func (e *env) worker() *contact.Worker {
	w := contact.NewWorker(e.store, e.resend, slog.New(slog.NewTextHandler(io.Discard, nil)))
	w.Jitter = func() float64 { return 1 }
	return w
}

func (e *env) run(t *testing.T) int {
	t.Helper()
	n, err := e.worker().RunDue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

var keySeq atomic.Int64

func newKey() string { return fmt.Sprintf("test-key-%016d", keySeq.Add(1)) }

func body(key string, over map[string]string) map[string]string {
	b := map[string]string{"name": "Ana Pérez", "email": "ana@example.org", "message": "Hola, me interesa el rover.\nSaludos.", "locale": "es", "key": key}
	for k, v := range over {
		if v == "<delete>" {
			delete(b, k)
		} else {
			b[k] = v
		}
	}
	return b
}

// post sends as an anonymous visitor (no session) from the given client IP (via the trusted
// local proxy hop), with the same Origin as the site.
func (e *env) post(t *testing.T, payload any, contentType string, headers ...string) authtest.Response {
	t.Helper()
	var raw string
	switch p := payload.(type) {
	case string:
		raw = p
	case url.Values:
		raw = p.Encode()
	default:
		b, _ := json.Marshal(p)
		raw = string(b)
	}
	req, _ := http.NewRequest("POST", e.Origin+"/api/v1/contact", strings.NewReader(raw))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Origin", e.Origin)
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i+1] == "" {
			req.Header.Del(headers[i])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	c := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return authtest.Response{Status: resp.StatusCode, Header: resp.Header, Body: b}
}

func (e *env) postJSON(t *testing.T, payload any, ip string) authtest.Response {
	t.Helper()
	return e.post(t, payload, "application/json", "X-Forwarded-For", ip)
}

func (e *env) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func code(t *testing.T, r authtest.Response) string {
	t.Helper()
	var e struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(r.Body, &e)
	return e.Code
}

type job struct {
	ID, Status, Key string
	Attempts        int
	Uncertain       bool
	ProviderID      *string
	LastError       *string
	Next            time.Time
}

func (e *env) jobs(t *testing.T) []job {
	t.Helper()
	rows, err := e.pool.Query(context.Background(), `SELECT message_id::text, status, idempotency_key, attempts, uncertain, provider_email_id, last_error, next_attempt_at FROM contact_jobs ORDER BY created_at, message_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.ID, &j.Status, &j.Key, &j.Attempts, &j.Uncertain, &j.ProviderID, &j.LastError, &j.Next); err != nil {
			t.Fatal(err)
		}
		out = append(out, j)
	}
	return out
}

// --- reception --------------------------------------------------------------------------------

func TestDisabledAcceptsNothing(t *testing.T) {
	e := start(t, func(string) string { return "" })
	if r := e.get(t, "/api/v1/public/contact"); string(r.Body) != "{\"available\":false}\n" || r.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("public status %s %v", r.Body, r.Header)
	}
	if r := e.postJSON(t, body(newKey(), nil), "203.0.113.1"); r.Status != http.StatusServiceUnavailable || code(t, r) != "contact_unavailable" {
		t.Fatalf("disabled post: %d %s", r.Status, r.Body)
	}
	form := url.Values{}
	for k, v := range body(newKey(), map[string]string{"locale": "en"}) {
		form.Set(k, v)
	}
	if r := e.post(t, form, "application/x-www-form-urlencoded"); r.Status != http.StatusSeeOther || r.Header.Get("Location") != "/en/contact?estado=no-disponible#formulario" {
		t.Fatalf("disabled form: %d %v", r.Status, r.Header)
	}
	if n := e.count(t, `SELECT count(*) FROM contact_messages`) + e.count(t, `SELECT count(*) FROM contact_jobs`); n != 0 {
		t.Fatalf("stored %d rows while disabled", n)
	}
	if n := e.run(t); n != 0 || e.fake.Calls() != 0 {
		t.Fatalf("sent while disabled")
	}
}

func (e *env) get(t *testing.T, path string) authtest.Response {
	t.Helper()
	resp, err := http.Get(e.Origin + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return authtest.Response{Status: resp.StatusCode, Header: resp.Header, Body: b}
}

func TestReceiptValidationAndIdempotency(t *testing.T) {
	e := start(t, enabledEnv(nil))
	if r := e.get(t, "/api/v1/public/contact"); string(r.Body) != "{\"available\":true}\n" {
		t.Fatalf("public status %s", r.Body)
	}
	key := newKey()
	r := e.postJSON(t, body(key, nil), "203.0.113.1")
	if r.Status != http.StatusAccepted || string(r.Body) != "{\"status\":\"received\"}\n" || r.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("accept: %d %s %v", r.Status, r.Body, r.Header)
	}
	// Same key and content: same acknowledgement, no second job. Other content: conflict.
	if r := e.postJSON(t, body(key, nil), "203.0.113.2"); r.Status != http.StatusAccepted {
		t.Fatalf("replay: %d %s", r.Status, r.Body)
	}
	if r := e.postJSON(t, body(key, map[string]string{"message": "Otro mensaje diferente."}), "203.0.113.3"); r.Status != http.StatusConflict || code(t, r) != "idempotency_conflict" {
		t.Fatalf("conflict: %d %s", r.Status, r.Body)
	}
	// Different messages with the same text are not deduplicated.
	if r := e.postJSON(t, body(newKey(), nil), "203.0.113.4"); r.Status != http.StatusAccepted {
		t.Fatalf("second message: %d", r.Status)
	}
	if n := e.count(t, `SELECT count(*) FROM contact_jobs WHERE status = 'pending'`); n != 2 {
		t.Fatalf("jobs = %d", n)
	}
	// Frozen payload: fixed From/To, visitor only as Reply-To, plain text with line breaks kept.
	var p contact.Email
	var raw []byte
	if err := e.pool.QueryRow(context.Background(), `SELECT payload FROM contact_jobs j JOIN contact_messages m ON m.id = j.message_id WHERE m.client_key = $1`, key).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &p)
	if p.From != `"BrambiLab" <contacto@example.com>` || p.To != "owner@example.com" || p.ReplyTo != "ana@example.org" ||
		!strings.HasPrefix(p.Subject, "Contacto BrambiLab #") || !strings.Contains(p.Text, "Hola, me interesa el rover.\nSaludos.") || !strings.Contains(p.Text, "Nombre: Ana Pérez") {
		t.Fatalf("payload %+v", p)
	}

	ip := 10
	bad := func(name string, payload any, status int, ct ...string) {
		t.Helper()
		ip++
		contentType := "application/json"
		if len(ct) > 0 {
			contentType = ct[0]
		}
		r := e.post(t, payload, contentType, "X-Forwarded-For", fmt.Sprintf("203.0.113.%d", ip))
		if r.Status != status {
			t.Errorf("%s: %d %s, want %d", name, r.Status, r.Body, status)
		}
	}
	for name, over := range map[string]map[string]string{
		"CRLF in name":        {"name": "Ana\r\nBcc: x@evil.test"},
		"CRLF in email":       {"email": "ana@example.org\r\nBcc: x@evil.test"},
		"named email":         {"email": "Ana <ana@example.org>"},
		"two addresses":       {"email": "a@example.org, b@example.org"},
		"not an email":        {"email": "ana"},
		"empty name":          {"name": "   "},
		"long name":           {"name": strings.Repeat("a", contact.MaxName+1)},
		"short message":       {"message": "hola"},
		"long message":        {"message": strings.Repeat("a", contact.MaxMessage+1)},
		"control in message":  {"message": "Hola\x07 mundo con campana"},
		"bad locale":          {"locale": "fr"},
		"short key":           {"key": "abc"},
		"missing key":         {"key": "<delete>"},
		"forbidden field to":  {"to": "x@evil.test"},
		"forbidden field bcc": {"bcc": "x@evil.test"},
	} {
		bad(name, body(newKey(), over), http.StatusUnprocessableEntity)
	}
	// Invalid UTF-8 can only arrive through a form (JSON replaces it); it is rejected.
	k := newKey()
	bad("invalid utf8", "name=Ana&email=ana%40example.org&locale=es&key="+k+"&message=Hola+%FF%FE+mundo+largo", http.StatusSeeOther, "application/x-www-form-urlencoded")
	bad("array value", `{"name":["a"],"email":"a@example.org","message":"0123456789","locale":"es","key":"`+newKey()+`"}`, http.StatusUnprocessableEntity)
	bad("two objects", `{"name":"a"}{}`, http.StatusUnprocessableEntity)
	bad("attachment", `{"attachments":[{"content":"x"}]}`, http.StatusUnprocessableEntity)
	bad("too large", `{"message":"`+strings.Repeat("a", contact.MaxBody)+`"}`, http.StatusRequestEntityTooLarge)
	bad("text/plain", "hola", http.StatusUnsupportedMediaType, "text/plain")
	bad("multipart", "x", http.StatusUnsupportedMediaType, "multipart/form-data; boundary=x")
	// HTML is only text: stored as typed (the owner's e-mail is plain text).
	if r := e.postJSON(t, body(newKey(), map[string]string{"message": `<script>alert("x")</script> hola`}), "203.0.113.90"); r.Status != http.StatusAccepted {
		t.Fatalf("html as text: %d %s", r.Status, r.Body)
	}
	// Origin is required like any mutation (no owner session or CSRF involved).
	if r := e.post(t, body(newKey(), nil), "application/json", "Origin", "", "X-Forwarded-For", "203.0.113.91"); r.Status != http.StatusForbidden {
		t.Fatalf("no origin: %d", r.Status)
	}
	if r := e.post(t, body(newKey(), nil), "application/json", "Origin", "https://evil.test", "X-Forwarded-For", "203.0.113.92"); r.Status != http.StatusForbidden {
		t.Fatalf("cross origin: %d", r.Status)
	}
	if n := e.count(t, `SELECT count(*) FROM contact_messages`); n != 3 {
		t.Fatalf("messages = %d, want 3", n)
	}
}

func TestFormPostsRedirectWithoutPersonalData(t *testing.T) {
	e := start(t, enabledEnv(nil))
	form := func(over map[string]string) url.Values {
		v := url.Values{}
		for k, val := range body(newKey(), over) {
			v.Set(k, val)
		}
		return v
	}
	cases := []struct {
		over     map[string]string
		location string
	}{
		{nil, "/es/contacto?estado=recibido#formulario"},
		{map[string]string{"locale": "en"}, "/en/contact?estado=recibido#formulario"},
		{map[string]string{"message": "corto"}, "/es/contacto?estado=invalido#formulario"},
		{map[string]string{"bl_hp": "http://spam.test"}, "/es/contacto?estado=recibido#formulario"},
	}
	for i, c := range cases {
		r := e.post(t, form(c.over), "application/x-www-form-urlencoded", "X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i))
		if r.Status != http.StatusSeeOther || r.Header.Get("Location") != c.location {
			t.Fatalf("case %d: %d %v", i, r.Status, r.Header.Get("Location"))
		}
		if strings.Contains(r.Header.Get("Location"), "ana") {
			t.Fatal("personal data in the redirect")
		}
	}
	// Repeated fields are rejected.
	v := form(nil)
	v.Add("name", "Otra")
	if r := e.post(t, v, "application/x-www-form-urlencoded", "X-Forwarded-For", "198.51.100.50"); r.Header.Get("Location") != "/es/contacto?estado=invalido#formulario" {
		t.Fatalf("repeated: %v", r.Header.Get("Location"))
	}
	// Two accepted (es, en); the honeypot one stored nothing.
	if n := e.count(t, `SELECT count(*) FROM contact_messages`); n != 2 {
		t.Fatalf("messages = %d", n)
	}
}

func TestHoneypotStoresNothing(t *testing.T) {
	e := start(t, enabledEnv(nil))
	r := e.postJSON(t, body(newKey(), map[string]string{"bl_hp": "filled by a bot"}), "203.0.113.1")
	if r.Status != http.StatusAccepted || string(r.Body) != "{\"status\":\"received\"}\n" {
		t.Fatalf("honeypot answer %d %s", r.Status, r.Body)
	}
	if n := e.count(t, `SELECT count(*) FROM contact_messages`) + e.count(t, `SELECT count(*) FROM contact_jobs`); n != 0 {
		t.Fatalf("honeypot stored %d rows", n)
	}
}

func TestRateLimitsPerClientAndGlobal(t *testing.T) {
	e := start(t, enabledEnv(map[string]string{"CONTACT_RATE_GLOBAL": "8"}))
	// Five per client per 15 minutes, counted on every attempt, concurrently.
	var wg sync.WaitGroup
	statuses := make([]int, 10)
	for i := range statuses {
		wg.Go(func() { statuses[i] = e.postJSON(t, body(newKey(), nil), "203.0.113.7").Status })
	}
	wg.Wait()
	ok, limited := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusAccepted:
			ok++
		case http.StatusTooManyRequests:
			limited++
		}
	}
	if ok != 5 || limited != 5 {
		t.Fatalf("statuses %v", statuses)
	}
	r := e.postJSON(t, body(newKey(), nil), "203.0.113.7")
	if r.Status != http.StatusTooManyRequests || r.Header.Get("Retry-After") != "900" {
		t.Fatalf("limited: %d Retry-After %q", r.Status, r.Header.Get("Retry-After"))
	}
	// A spoofed left part does not change the identity (the rightmost untrusted hop decides).
	if r := e.postJSON(t, body(newKey(), nil), "198.51.100.99, 203.0.113.7"); r.Status != http.StatusTooManyRequests {
		t.Fatalf("spoofed chain escaped the limit: %d", r.Status)
	}
	// Another legitimate client is not affected.
	if r := e.postJSON(t, body(newKey(), nil), "203.0.113.8"); r.Status != http.StatusAccepted {
		t.Fatalf("other client: %d", r.Status)
	}
	// Global: 8 accepted per hour across all clients (5 + 1 accepted so far).
	for i, want := range []int{202, 202, 429} {
		if r := e.postJSON(t, body(newKey(), nil), fmt.Sprintf("203.0.113.%d", 20+i)); r.Status != want {
			t.Fatalf("global %d: %d", i, r.Status)
		}
	}
	// Counters are HMACs, not IPs; a new window resets the client quota.
	if n := e.count(t, `SELECT count(*) FROM contact_rate WHERE bucket LIKE '%203.0.113%'`); n != 0 {
		t.Fatal("clear IP stored in counters")
	}
	e.advance(15 * time.Minute)
	if r := e.postJSON(t, body(newKey(), nil), "203.0.113.7"); r.Status != http.StatusTooManyRequests || r.Header.Get("Retry-After") == "" {
		t.Fatalf("global limit still applies: %d", r.Status)
	}
	e.advance(time.Hour)
	if r := e.postJSON(t, body(newKey(), nil), "203.0.113.7"); r.Status != http.StatusAccepted {
		t.Fatalf("after windows: %d", r.Status)
	}
}

// --- sending ----------------------------------------------------------------------------------

func (e *env) accepted(t *testing.T, n int) {
	t.Helper()
	for i := range n {
		if r := e.postJSON(t, body(newKey(), map[string]string{"name": fmt.Sprintf("Visitante %d", i)}), fmt.Sprintf("192.0.2.%d", i+1)); r.Status != http.StatusAccepted {
			t.Fatalf("post: %d %s", r.Status, r.Body)
		}
	}
}

func TestSendAcceptedWithFrozenPayload(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 1)
	if n := e.run(t); n != 1 {
		t.Fatalf("ran %d", n)
	}
	j := e.jobs(t)[0]
	sent := e.fake.Sent()
	if j.Status != "accepted_by_provider" || j.ProviderID == nil || len(sent) != 1 || *j.ProviderID != sent[0].ID || sent[0].IdempotencyKey != "contact/"+j.ID {
		t.Fatalf("job %+v sent %+v", j, sent)
	}
	var payload map[string]any
	_ = json.Unmarshal(sent[0].Body, &payload)
	if payload["from"] != `"BrambiLab" <contacto@example.com>` || payload["reply_to"] != "ana@example.org" || fmt.Sprint(payload["to"]) != "[owner@example.com]" || payload["html"] != nil {
		t.Fatalf("sent payload %v", payload)
	}
	// Accepted jobs are never processed again.
	e.advance(time.Hour)
	if n := e.run(t); n != 0 || e.fake.Calls() != 1 {
		t.Fatalf("accepted job resent")
	}
}

func TestProviderOutcomes(t *testing.T) {
	cases := []struct {
		name      string
		steps     []fakeresend.Step
		status    string
		uncertain bool
		nextAfter time.Duration
		lastError string
	}{
		{"validation error is permanent", []fakeresend.Step{{Status: 422, Name: "validation_error"}}, "failed", false, 0, "validation_error"},
		{"unverified domain is permanent", []fakeresend.Step{{Status: 403, Name: "validation_error"}}, "failed", false, 0, "validation_error"},
		{"idempotency mismatch is permanent", []fakeresend.Step{{Status: 409, Name: "invalid_idempotent_request"}}, "failed", false, 0, "invalid_idempotent_request"},
		{"429 honours Retry-After", []fakeresend.Step{{Status: 429, Name: "rate_limit_exceeded", RetryAfter: 120}}, "retry_wait", false, 120 * time.Second, "rate_limit_exceeded"},
		{"concurrent idempotent request retries", []fakeresend.Step{{Status: 409, Name: "concurrent_idempotent_requests"}}, "retry_wait", false, 30 * time.Second, "concurrent_idempotent_requests"},
		{"5xx is uncertain", []fakeresend.Step{{Status: 500, Name: "application_error"}}, "retry_wait", true, 30 * time.Second, "application_error"},
		{"timeout is uncertain", []fakeresend.Step{{Delay: time.Second}}, "retry_wait", true, 30 * time.Second, "timeout"},
		{"invalid answer is uncertain", []fakeresend.Step{{BadBody: true}}, "retry_wait", true, 30 * time.Second, "invalid_response"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := start(t, enabledEnv(nil))
			e.accepted(t, 1)
			e.fake.Script(c.steps...)
			e.run(t)
			j := e.jobs(t)[0]
			if j.Status != c.status || j.Uncertain != c.uncertain || j.LastError == nil || *j.LastError != c.lastError {
				t.Fatalf("job %+v last %v", j, j.LastError)
			}
			if c.nextAfter > 0 && !j.Next.Equal(t0.Add(c.nextAfter)) {
				t.Fatalf("next attempt %v, want %v", j.Next, t0.Add(c.nextAfter))
			}
			// Permanent failures are not retried in a loop.
			if c.status == "failed" {
				e.advance(time.Hour)
				if n := e.run(t); n != 0 {
					t.Fatal("failed job retried")
				}
			}
		})
	}
}

func TestAcceptedButAnswerLostIsNotDuplicated(t *testing.T) {
	for _, step := range []fakeresend.Step{{Accept: true, Status: 500, Name: "application_error"}, {Accept: true, Delay: time.Second}} {
		e := start(t, enabledEnv(nil))
		e.accepted(t, 1)
		e.fake.Script(step)
		e.run(t)
		if j := e.jobs(t)[0]; j.Status != "retry_wait" || !j.Uncertain {
			t.Fatalf("after lost answer %+v", j)
		}
		e.advance(time.Minute)
		e.run(t)
		j := e.jobs(t)[0]
		sent := e.fake.Sent()
		// Same key and payload: the provider answers the original id, no second email.
		if j.Status != "accepted_by_provider" || len(sent) != 1 || *j.ProviderID != sent[0].ID || e.fake.Calls() != 2 {
			t.Fatalf("job %+v sent %d calls %d", j, len(sent), e.fake.Calls())
		}
	}
}

// A worker crashes after the provider accepted, before recording it; recovery after the lease
// resends with the same key inside the window and gets the same id.
func TestCrashAfterAcceptanceIsRecovered(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 1)
	ctx, crash := context.WithCancel(context.Background())
	w := contact.NewWorker(e.store, crashAfter{e.resend, crash}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := w.RunDue(ctx); err == nil {
		t.Fatal("expected the crash")
	}
	if j := e.jobs(t)[0]; j.Status != "processing" || len(e.fake.Sent()) != 1 {
		t.Fatalf("after crash %+v", j)
	}
	e.advance(30 * time.Second) // lease still valid: nobody touches it
	if n := e.run(t); n != 0 {
		t.Fatal("took a leased job")
	}
	e.advance(contact.Lease)
	e.run(t)
	j := e.jobs(t)[0]
	if j.Status != "accepted_by_provider" || len(e.fake.Sent()) != 1 || e.count(t, `SELECT count(*) FROM contact_attempts WHERE outcome = 'lease_expired'`) != 1 {
		t.Fatalf("recovered %+v sent %d", j, len(e.fake.Sent()))
	}
}

type crashAfter struct {
	p     contact.Provider
	crash context.CancelFunc
}

func (c crashAfter) Send(ctx context.Context, m contact.Email, key string) contact.Result {
	r := c.p.Send(ctx, m, key)
	c.crash()
	return r
}

func TestTwoWorkersNeverDuplicate(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 5)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			if _, err := e.worker().RunDue(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	e.run(t) // whatever a worker skipped
	if n := len(e.fake.Sent()); n != 5 || e.fake.Calls() != 5 {
		t.Fatalf("sent %d with %d calls, want 5", n, e.fake.Calls())
	}
	for _, j := range e.jobs(t) {
		if j.Status != "accepted_by_provider" || j.Attempts != 1 {
			t.Fatalf("job %+v", j)
		}
	}
}

func TestUncertainPastTheWindowBecomesUnknown(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 1)
	e.fake.Script(fakeresend.Step{Status: 503, Name: "service_unavailable"})
	e.run(t)
	calls := e.fake.Calls()
	// The process was down for a day: the provider forgot the key, so resending could duplicate.
	e.advance(contact.IdempotencyWindow)
	e.run(t)
	if j := e.jobs(t)[0]; j.Status != "unknown" || e.fake.Calls() != calls {
		t.Fatalf("job %+v, calls %d→%d", j, calls, e.fake.Calls())
	}
}

func TestAttemptsExhausted(t *testing.T) {
	// Only transient, certainly unsent answers: failed. Any uncertain one: unknown.
	for _, c := range []struct {
		step   fakeresend.Step
		status string
	}{{fakeresend.Step{Status: 429, Name: "rate_limit_exceeded"}, "failed"}, {fakeresend.Step{Status: 500, Name: "application_error"}, "unknown"}} {
		e := start(t, enabledEnv(nil))
		e.accepted(t, 1)
		for range 5 {
			e.fake.Script(c.step)
		}
		waits := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute}
		for i := range 5 {
			e.run(t)
			if i < 4 {
				j := e.jobs(t)[0]
				if !j.Next.Equal(e.now.Load().Add(waits[i])) {
					t.Fatalf("backoff %d: %v", i, j.Next.Sub(*e.now.Load()))
				}
				e.advance(waits[i])
			}
		}
		if j := e.jobs(t)[0]; j.Status != c.status || j.Attempts != 5 {
			t.Fatalf("%s: %+v", c.step.Name, j)
		}
	}
}

// --- panel ------------------------------------------------------------------------------------

type detail struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	Version         int     `json:"version"`
	Message         string  `json:"message"`
	Email           string  `json:"email"`
	Retry           string  `json:"retry"`
	IdempotencyKey  string  `json:"idempotency_key"`
	ProviderEmailID *string `json:"provider_email_id"`
	AttemptLog      []struct {
		Outcome    string  `json:"outcome"`
		HTTPStatus *int    `json:"http_status"`
		ErrorName  *string `json:"error_name"`
	} `json:"attempt_log"`
}

func (e *env) detail(t *testing.T, id string) detail {
	t.Helper()
	r := e.Do("GET", "/api/v1/admin/contact/messages/"+id, nil)
	if r.Status != http.StatusOK {
		t.Fatalf("detail: %d %s", r.Status, r.Body)
	}
	var d detail
	r.JSON(t, &d)
	return d
}

func (e *env) retry(id string, version int, confirm bool) authtest.Response {
	return e.Do("POST", "/api/v1/admin/contact/messages/"+id+"/retry", map[string]any{"expected_version": version, "confirm_possible_duplicate": confirm})
}

func TestPanelListDetailAndConsciousRetry(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 3)
	// One accepted, one permanent failure, one uncertain (later past the window → unknown).
	e.fake.Script(fakeresend.Step{}, fakeresend.Step{Status: 422, Name: "validation_error"}, fakeresend.Step{Status: 500, Name: "application_error"})
	e.run(t)
	byStatus := map[string]job{}
	for _, j := range e.jobs(t) {
		byStatus[j.Status] = j
	}
	if len(byStatus) != 3 {
		t.Fatalf("statuses %v", byStatus)
	}
	accID, failedID, uncertainID := byStatus["accepted_by_provider"].ID, byStatus["failed"].ID, byStatus["retry_wait"].ID

	acc := e.detail(t, accID)
	if acc.Status != "accepted_by_provider" || acc.ProviderEmailID == nil || acc.Retry != "none" || acc.Message == "" || len(acc.AttemptLog) != 1 {
		t.Fatalf("accepted detail %+v", acc)
	}
	if r := e.retry(acc.ID, acc.Version, true); r.Status != http.StatusConflict || code(t, r) != "not_retryable" {
		t.Fatalf("retry accepted: %d %s", r.Status, r.Body)
	}

	// A certain failure inside the window: same key, back in the queue.
	failed := e.detail(t, failedID)
	if failed.Retry != "same_key" || *failed.AttemptLog[0].ErrorName != "validation_error" || *failed.AttemptLog[0].HTTPStatus != 422 {
		t.Fatalf("failed detail %+v", failed)
	}
	if r := e.retry(failed.ID, failed.Version-1, false); r.Status != http.StatusConflict || code(t, r) != "version_conflict" {
		t.Fatalf("stale retry: %d", r.Status)
	}
	var after detail
	r := e.retry(failed.ID, failed.Version, false)
	r.JSON(t, &after)
	if r.Status != http.StatusOK || after.Status != "retry_wait" || after.IdempotencyKey != failed.IdempotencyKey {
		t.Fatalf("retry failed: %d %s", r.Status, r.Body)
	}
	// Double click: the second request carries the old version.
	if r := e.retry(failed.ID, failed.Version, false); r.Status != http.StatusConflict {
		t.Fatalf("double retry: %d", r.Status)
	}
	e.fake.Script(fakeresend.Step{Status: 422, Name: "validation_error"}) // fails again: stays failed

	// The uncertain one waits past the window and becomes unknown.
	e.advance(contact.IdempotencyWindow)
	e.run(t)
	list := struct {
		Items []struct{ ID, Status, Email string } `json:"items"`
		Total int                                  `json:"total"`
	}{}
	e.Do("GET", "/api/v1/admin/contact/messages", nil).JSON(t, &list)
	if list.Total != 3 {
		t.Fatalf("list %+v", list)
	}
	e.Do("GET", "/api/v1/admin/contact/messages?status=unknown", nil).JSON(t, &list)
	if list.Total != 1 || list.Items[0].ID != uncertainID {
		t.Fatalf("filtered %+v", list)
	}
	if r := e.Do("GET", "/api/v1/admin/contact/messages?status=delivered", nil); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("bad status filter %d", r.Status)
	}
	// Outside the window even a certain failure needs a new key and confirmation.
	if f := e.detail(t, failedID); f.Status != "failed" || f.Retry != "new_key_confirm" {
		t.Fatalf("failed outside window %+v", f)
	}
	jobs := []job{{ID: accID}, {ID: failedID}, {ID: uncertainID}}

	// Unknown: only with explicit confirmation, a new key, and audited.
	unk := e.detail(t, jobs[2].ID)
	if unk.Status != "unknown" || unk.Retry != "new_key_confirm" {
		t.Fatalf("unknown detail %+v", unk)
	}
	if r := e.retry(unk.ID, unk.Version, false); r.Status != http.StatusConflict || code(t, r) != "confirmation_required" {
		t.Fatalf("unconfirmed: %d %s", r.Status, r.Body)
	}
	r = e.retry(unk.ID, unk.Version, true)
	r.JSON(t, &after)
	if r.Status != http.StatusOK || after.IdempotencyKey == unk.IdempotencyKey || !strings.HasPrefix(after.IdempotencyKey, "contact/"+unk.ID+"/r") {
		t.Fatalf("confirmed retry: %d %s", r.Status, r.Body)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_events WHERE action = 'contact.retry'`); n != 2 { // failed + unknown
		t.Fatalf("audit rows %d", n)
	}
	var auditMeta string
	_ = e.pool.QueryRow(context.Background(), `SELECT string_agg(metadata::text, ' ') FROM audit_events WHERE action = 'contact.retry'`).Scan(&auditMeta)
	if strings.Contains(auditMeta, "@") || strings.Contains(auditMeta, "Visitante") {
		t.Fatalf("personal data in audit: %s", auditMeta)
	}
	e.run(t)
	if len(e.fake.Sent()) != 2 { // the accepted one and the confirmed unknown one
		t.Fatalf("sent %d after retries", len(e.fake.Sent()))
	}

	for _, path := range []string{"/api/v1/admin/contact/messages", "/api/v1/admin/contact/messages/" + jobs[0].ID} {
		if r := e.Anonymous("GET", path, nil); r.Status != http.StatusUnauthorized {
			t.Fatalf("anonymous %s: %d", path, r.Status)
		}
	}
	if r := e.WithoutCSRF("POST", "/api/v1/admin/contact/messages/"+jobs[1].ID+"/retry", map[string]any{"expected_version": 0}); r.Status != http.StatusForbidden {
		t.Fatalf("no csrf: %d", r.Status)
	}
	if r := e.Do("GET", "/api/v1/admin/contact/messages", nil); !strings.Contains(r.Header.Get("Cache-Control"), "no-store") {
		t.Fatalf("admin cache %q", r.Header.Get("Cache-Control"))
	}
}

func TestRetentionPurgesAllCopies(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 2)
	e.fake.Script(fakeresend.Step{Status: 500, Name: "application_error"})
	e.run(t) // job A: uncertain with an attempt row; job B: accepted
	// Lease one job as if a worker held it right now.
	jobs := e.jobs(t)
	e.advance(31 * 24 * time.Hour)
	if _, err := e.pool.Exec(context.Background(), `UPDATE contact_jobs SET status = 'processing', lease_token = gen_random_uuid(), lease_until = $2 WHERE message_id = $1`,
		jobs[0].ID, e.now.Load().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := e.store.Purge(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := e.count(t, `SELECT count(*) FROM contact_messages`); n != 1 {
		t.Fatalf("messages left %d, want only the leased one", n)
	}
	e.advance(2 * time.Minute)
	if err := e.store.Purge(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"contact_messages", "contact_jobs", "contact_attempts", "contact_rate"} {
		if n := e.count(t, `SELECT count(*) FROM `+table); n != 0 {
			t.Fatalf("%s keeps %d rows after retention", table, n)
		}
	}
	// A purged job can never be sent.
	if n := e.run(t); n != 0 {
		t.Fatal("ran a purged job")
	}
}

// A crash whose lease expires past the window: it may have been sent, so it becomes unknown.
func TestCrashPastTheWindowBecomesUnknown(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 1)
	ctx, crash := context.WithCancel(context.Background())
	w := contact.NewWorker(e.store, crashAfter{e.resend, crash}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, _ = w.RunDue(ctx)
	calls := e.fake.Calls()
	e.advance(contact.IdempotencyWindow)
	e.run(t)
	if j := e.jobs(t)[0]; j.Status != "unknown" || !j.Uncertain || e.fake.Calls() != calls {
		t.Fatalf("job %+v calls %d→%d", j, calls, e.fake.Calls())
	}
}

// A permanent answer after an uncertain attempt is not "failed": the earlier one may have gone.
func TestPermanentAfterUncertainIsUnknown(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 1)
	e.fake.Script(fakeresend.Step{Accept: true, Status: 500, Name: "application_error"}, fakeresend.Step{Status: 401, Name: "missing_api_key"})
	e.run(t)
	e.advance(time.Minute)
	e.run(t)
	if j := e.jobs(t)[0]; j.Status != "unknown" {
		t.Fatalf("job %+v", j)
	}
}

// A slow worker whose lease expired cannot overwrite the result recorded by the next worker.
func TestLateWorkerCannotOverwrite(t *testing.T) {
	e := start(t, enabledEnv(nil))
	e.accepted(t, 1)
	release, started := make(chan struct{}), make(chan struct{})
	slow := contact.NewWorker(e.store, blocking{started, release}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan error, 1)
	go func() { _, err := slow.RunDue(context.Background()); done <- err }()
	<-started
	e.advance(contact.Lease + time.Second)
	e.run(t) // takes over the expired lease and gets accepted
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if j := e.jobs(t)[0]; j.Status != "accepted_by_provider" || j.ProviderID == nil {
		t.Fatalf("late result overwrote the job: %+v", j)
	}
}

type blocking struct{ started, release chan struct{} }

func (b blocking) Send(context.Context, contact.Email, string) contact.Result {
	close(b.started)
	<-b.release
	return contact.Result{Outcome: contact.Permanent, HTTPStatus: 422, ErrorName: "validation_error"}
}
