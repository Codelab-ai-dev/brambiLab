package contact

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// Email is the frozen payload of a job (template v1): fixed From/To, the visitor as Reply-To,
// fixed subject with the message id and a plain-text body. No HTML, headers or attachments.
type Email struct {
	From    string `json:"from"`
	To      string `json:"to"`
	ReplyTo string `json:"reply_to"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

// Outcome classifies one provider call (web-v1.md §13.1).
type Outcome string

const (
	Accepted  Outcome = "accepted"  // provider returned an email id
	Transient Outcome = "transient" // not sent; retry later (429, concurrent idempotent request…)
	Uncertain Outcome = "uncertain" // may have been sent (timeout, network, 5xx, invalid answer)
	Permanent Outcome = "permanent" // will not succeed by retrying (auth, validation, idempotency mismatch)
)

// Result is sanitized: no response body, only what the panel may show.
type Result struct {
	Outcome    Outcome
	EmailID    string
	HTTPStatus int
	ErrorName  string
	RequestID  string
	RetryAfter time.Duration
}

// Provider sends one email with an idempotency key. Implementations never return raw bodies.
type Provider interface {
	Send(ctx context.Context, e Email, idempotencyKey string) Result
}

// Resend calls POST {base}/emails (https://resend.com/docs/api-reference/emails/send-email).
type Resend struct {
	base   string
	apiKey string
	client *http.Client
}

const (
	sendTimeout = 15 * time.Second // below the 60 s job lease
	maxResponse = 16 << 10
)

// SetTimeout shortens the call timeout in tests (production keeps sendTimeout, below the lease).
func (p *Resend) SetTimeout(d time.Duration) { p.client.Timeout = d }

func NewResend(base, apiKey string) *Resend {
	return &Resend{base: base, apiKey: apiKey, client: &http.Client{Timeout: sendTimeout,
		// The endpoint is fixed by configuration; never follow redirects elsewhere.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

var (
	safeName  = regexp.MustCompile(`^[a-z_]{1,64}$`)
	safeID    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	transient = map[string]bool{"rate_limit_exceeded": true, "daily_quota_exceeded": true, "monthly_quota_exceeded": true,
		"concurrent_idempotent_requests": true, "resource_locked": true}
)

func (p *Resend) Send(ctx context.Context, e Email, key string) Result {
	body, _ := json.Marshal(map[string]any{"from": e.From, "to": []string{e.To}, "reply_to": e.ReplyTo, "subject": e.Subject, "text": e.Text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/emails", bytes.NewReader(body))
	if err != nil {
		return Result{Outcome: Permanent, ErrorName: "request_build_failed"}
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	resp, err := p.client.Do(req)
	if err != nil {
		// Timeout or network failure: the request may have been processed.
		name := "network_error"
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			name = "timeout"
		}
		return Result{Outcome: Uncertain, ErrorName: name}
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	res := Result{HTTPStatus: resp.StatusCode}
	if id := resp.Header.Get("X-Request-Id"); safeID.MatchString(id) {
		res.RequestID = id
	}
	var parsed struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if safeName.MatchString(parsed.Name) {
		res.ErrorName = parsed.Name
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if readErr != nil || !safeID.MatchString(parsed.ID) {
			res.Outcome, res.ErrorName = Uncertain, "invalid_response"
			return res
		}
		res.Outcome, res.EmailID = Accepted, parsed.ID
	case resp.StatusCode == http.StatusTooManyRequests || transient[res.ErrorName]:
		res.Outcome = Transient
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 && s <= 86400 {
			res.RetryAfter = time.Duration(s) * time.Second
		}
	case resp.StatusCode >= 500:
		res.Outcome = Uncertain
	default:
		res.Outcome = Permanent
	}
	return res
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}
