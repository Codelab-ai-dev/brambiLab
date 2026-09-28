package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/fakegithub"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/testdb"
)

const (
	ownerID      = 201345228
	clientSecret = "test-client-secret"
)

var owner = fakegithub.User{ID: ownerID, Login: "owner"}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type env struct {
	t      *testing.T
	pool   *pgxpool.Pool
	fake   *fakegithub.Server
	origin string
	logs   *syncBuffer
}

// start runs the real router (same-origin check, auth, sessions) against a fake GitHub.
func start(t *testing.T, pool *pgxpool.Pool, adminID int64) *env {
	t.Helper()
	fake := fakegithub.New("test-client", clientSecret, owner)
	gh := httptest.NewServer(fake.Handler())
	t.Cleanup(gh.Close)

	api := httptest.NewUnstartedServer(nil)
	origin := "http://" + api.Listener.Addr().String()
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	cfg := auth.Config{
		PublicOrigin: origin, AdminUserID: adminID, ClientID: "test-client", ClientSecret: clientSecret,
		SessionTTL:   3600e9,
		AuthorizeURL: gh.URL + "/login/oauth/authorize", TokenURL: gh.URL + "/login/oauth/access_token", UserURL: gh.URL + "/user",
	}
	api.Config.Handler = httpapi.NewRouter(logger, origin, auth.NewHandler(cfg, pool, logger))
	api.Start()
	t.Cleanup(api.Close)
	return &env{t: t, pool: pool, fake: fake, origin: origin, logs: logs}
}

// browser follows redirects across API and fake GitHub, stopping when it lands on a web page.
func (e *env) browser() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if strings.HasPrefix(e.origin, req.URL.Scheme+"://"+req.URL.Host) && !strings.HasPrefix(req.URL.Path, "/api/") {
			return http.ErrUseLastResponse
		}
		return nil
	}}
}

// login runs the whole browser flow and returns where the API finally sends the browser.
func (e *env) login(c *http.Client, returnTo string) string {
	e.t.Helper()
	resp, err := c.Get(e.origin + "/api/v1/auth/github/start?return_to=" + url.QueryEscape(returnTo))
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		e.t.Fatalf("login ended with %d, want a redirect to the web", resp.StatusCode)
	}
	return resp.Header.Get("Location")
}

func (e *env) sessionCookie(c *http.Client) *http.Cookie {
	u, _ := url.Parse(e.origin)
	for _, ck := range c.Jar.Cookies(u) {
		if ck.Name == "bl_session" {
			return ck
		}
	}
	return nil
}

type me struct {
	GitHubUserID int64  `json:"github_user_id"`
	GitHubLogin  string `json:"github_login"`
	CSRFToken    string `json:"csrf_token"`
}

func (e *env) me(c *http.Client) (int, me, http.Header) {
	e.t.Helper()
	resp, err := c.Get(e.origin + "/api/v1/auth/me")
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var m me
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m, resp.Header
}

func (e *env) post(c *http.Client, path string, headers map[string]string, body string) *http.Response {
	e.t.Helper()
	req, _ := http.NewRequest(http.MethodPost, e.origin+path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func (e *env) count(sql string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func TestOwnerLoginSessionAndLogout(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	c := e.browser()

	if loc := e.login(c, "/admin/posts"); loc != e.origin+"/admin/posts" {
		t.Fatalf("owner redirected to %q", loc)
	}
	if e.sessionCookie(c) == nil {
		t.Fatal("no session cookie after owner login")
	}

	status, m, h := e.me(c)
	if status != http.StatusOK || m.GitHubUserID != ownerID || m.GitHubLogin != "owner" || m.CSRFToken == "" {
		t.Fatalf("/auth/me = %d %+v", status, m)
	}
	if !strings.Contains(h.Get("Cache-Control"), "no-store") {
		t.Fatalf("/auth/me Cache-Control = %q", h.Get("Cache-Control"))
	}

	// Mutations: wrong origin, then missing CSRF, are both rejected before logout succeeds.
	if r := e.post(c, "/api/v1/auth/logout", map[string]string{"Origin": "https://evil.example", "X-CSRF-Token": m.CSRFToken}, ""); r.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin logout = %d, want 403", r.StatusCode)
	}
	if r := e.post(c, "/api/v1/auth/logout", map[string]string{"Origin": e.origin}, ""); r.StatusCode != http.StatusForbidden {
		t.Fatalf("logout without CSRF = %d, want 403", r.StatusCode)
	}
	if r := e.post(c, "/api/v1/auth/logout", map[string]string{"Origin": e.origin, "X-CSRF-Token": "wrong"}, ""); r.StatusCode != http.StatusForbidden {
		t.Fatalf("logout with wrong CSRF = %d, want 403", r.StatusCode)
	}
	if r := e.post(c, "/api/v1/auth/logout", map[string]string{"Origin": e.origin, "X-CSRF-Token": m.CSRFToken}, ""); r.StatusCode != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", r.StatusCode)
	}
	if status, _, _ := e.me(c); status != http.StatusUnauthorized {
		t.Fatalf("/auth/me after logout = %d, want 401", status)
	}
	if n := e.count(`SELECT count(*) FROM sessions WHERE revoked_at IS NOT NULL`); n != 1 {
		t.Fatalf("revoked sessions = %d, want 1", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_events WHERE actor = $1 AND action IN ('auth.login','auth.logout')`, "github:201345228"); n != 2 {
		t.Fatalf("audit events = %d, want 2", n)
	}

	// Nothing secret reaches the logs: session token, client secret, GitHub token.
	for _, s := range []string{clientSecret, "access_token", m.CSRFToken} {
		if strings.Contains(e.logs.String(), s) {
			t.Fatalf("logs contain %q", s)
		}
	}
}

func TestFormLogoutWithoutJavaScript(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	c := e.browser()
	e.login(c, "/admin")
	_, m, _ := e.me(c)
	r := e.post(c, "/api/v1/auth/logout",
		map[string]string{"Origin": e.origin, "Content-Type": "application/x-www-form-urlencoded"},
		"csrf_token="+url.QueryEscape(m.CSRFToken))
	if r.StatusCode != http.StatusSeeOther || r.Header.Get("Location") != e.origin+"/admin/login?logged_out=1" {
		t.Fatalf("form logout = %d %q", r.StatusCode, r.Header.Get("Location"))
	}
}

func TestOtherAccountIsRejected(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	e.fake.SetUser(fakegithub.User{ID: 42, Login: "intruder"})
	c := e.browser()
	if loc := e.login(c, "/admin"); loc != e.origin+"/admin/login?error=forbidden" {
		t.Fatalf("intruder redirected to %q", loc)
	}
	if e.sessionCookie(c) != nil {
		t.Fatal("intruder received a session cookie")
	}
	if status, _, _ := e.me(c); status != http.StatusUnauthorized {
		t.Fatalf("/auth/me for intruder = %d", status)
	}
	if n := e.count(`SELECT count(*) FROM sessions`); n != 0 {
		t.Fatalf("sessions = %d, want 0", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_events WHERE action = 'auth.login_rejected' AND actor = 'github:42'`); n != 1 {
		t.Fatalf("rejection audit events = %d, want 1", n)
	}
}

func TestDeniedAuthorization(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	e.fake.SetUser(fakegithub.User{})
	if loc := e.login(e.browser(), "/admin"); loc != e.origin+"/admin/login?error=denied" {
		t.Fatalf("denied redirected to %q", loc)
	}
}

func TestCallbackRequiresBrowserBoundSingleUseState(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	// Obtain a valid callback URL (code + state) without following it.
	resp, err := noRedirect.Get(e.origin + "/api/v1/auth/github/start")
	if err != nil {
		t.Fatal(err)
	}
	stateCookie := resp.Cookies()[0]
	resp, err = noRedirect.Get(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	callback := resp.Header.Get("Location")

	get := func(withCookie bool) string {
		req, _ := http.NewRequest(http.MethodGet, callback, nil)
		if withCookie {
			req.AddCookie(stateCookie)
		}
		r, err := noRedirect.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.Header.Get("Location")
	}
	// An attacker replaying the callback in another browser (no state cookie) fails.
	if loc := get(false); loc != e.origin+"/admin/login?error=failed" {
		t.Fatalf("callback without state cookie → %q", loc)
	}
	if loc := get(true); loc != e.origin+"/admin" {
		t.Fatalf("legit callback → %q", loc)
	}
	// The state is single-use.
	if loc := get(true); loc != e.origin+"/admin/login?error=failed" {
		t.Fatalf("replayed callback → %q", loc)
	}
}

func TestSessionsAreBoundToCurrentOwnerAndExpire(t *testing.T) {
	pool := testdb.New(t)
	e := start(t, pool, ownerID)
	c := e.browser()
	e.login(c, "/admin")
	ck := e.sessionCookie(c)

	// Same DB, owner ID changed in configuration: the old owner's session no longer works.
	other := start(t, pool, 999)
	req, _ := http.NewRequest(http.MethodGet, other.origin+"/api/v1/auth/me", nil)
	req.AddCookie(ck)
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("session with changed owner = %d, want 401", r.StatusCode)
	}

	if _, err := pool.Exec(context.Background(), `UPDATE sessions SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if status, _, _ := e.me(c); status != http.StatusUnauthorized {
		t.Fatalf("expired session = %d, want 401", status)
	}
}

func TestAuthNotConfigured(t *testing.T) {
	e := start(t, testdb.New(t), 0)
	resp, err := http.Get(e.origin + "/api/v1/auth/github/start")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("start without owner configured = %d, want 503", resp.StatusCode)
	}
}

// pendingCallback starts a login and returns the state cookie and GitHub's callback URL
// without following it.
func (e *env) pendingCallback() (*http.Cookie, string) {
	e.t.Helper()
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(e.origin + "/api/v1/auth/github/start")
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	cookie := resp.Cookies()[0]
	resp, err = noRedirect.Get(resp.Header.Get("Location"))
	if err != nil {
		e.t.Fatal(err)
	}
	resp.Body.Close()
	return cookie, resp.Header.Get("Location")
}

func (e *env) callback(rawURL string, cookie *http.Cookie) string {
	e.t.Helper()
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	r, err := noRedirect.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	r.Body.Close()
	return r.Header.Get("Location")
}

func TestExpiredStateIsRejected(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	cookie, callback := e.pendingCallback()
	if _, err := e.pool.Exec(context.Background(), `UPDATE oauth_states SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if loc := e.callback(callback, cookie); loc != e.origin+"/admin/login?error=failed" {
		t.Fatalf("expired state → %q", loc)
	}
	if n := e.count(`SELECT count(*) FROM sessions`); n != 0 {
		t.Fatalf("sessions after expired state = %d", n)
	}
}

func TestForgedStateIsRejectedEvenWithMatchingCookie(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	_, callback := e.pendingCallback()
	u, _ := url.Parse(callback)
	q := u.Query()
	q.Set("state", "forged-state")
	u.RawQuery = q.Encode()
	// The attacker controls both the parameter and a matching cookie, but the state was never issued.
	if loc := e.callback(u.String(), &http.Cookie{Name: "bl_oauth_state", Value: "forged-state"}); loc != e.origin+"/admin/login?error=failed" {
		t.Fatalf("forged state → %q", loc)
	}
}

func TestTamperedSessionTokenIsRejected(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	c := e.browser()
	e.login(c, "/admin")
	ck := e.sessionCookie(c)
	last := ck.Value[len(ck.Value)-1]
	flipped := byte('A')
	if last == 'A' {
		flipped = 'B'
	}
	for _, value := range []string{ck.Value[:len(ck.Value)-1] + string(flipped), ck.Value + "x", "", "not-a-token"} {
		req, _ := http.NewRequest(http.MethodGet, e.origin+"/api/v1/auth/me", nil)
		req.AddCookie(&http.Cookie{Name: ck.Name, Value: value})
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusUnauthorized {
			t.Fatalf("tampered token %q → %d, want 401", value, r.StatusCode)
		}
	}
	// The genuine session keeps working.
	if status, _, _ := e.me(c); status != http.StatusOK {
		t.Fatalf("genuine session → %d", status)
	}
}

func TestPKCEVerifierMustMatchChallenge(t *testing.T) {
	e := start(t, testdb.New(t), ownerID)
	cookie, callback := e.pendingCallback()
	// Swap the stored verifier: GitHub (the fake) must refuse the exchange, so no session.
	if _, err := e.pool.Exec(context.Background(), `UPDATE oauth_states SET code_verifier = 'not-the-original-verifier'`); err != nil {
		t.Fatal(err)
	}
	if loc := e.callback(callback, cookie); loc != e.origin+"/admin/login?error=failed" {
		t.Fatalf("mismatched PKCE verifier → %q", loc)
	}
	if n := e.count(`SELECT count(*) FROM sessions`); n != 0 {
		t.Fatalf("sessions after PKCE failure = %d", n)
	}
}

func TestStatusReportsOnlyWhetherLoginIsEnabled(t *testing.T) {
	for _, tc := range []struct {
		adminID int64
		want    string
	}{{ownerID, `{"login_enabled":true}`}, {0, `{"login_enabled":false}`}} {
		// The status endpoint never touches the database.
		e := start(t, nil, tc.adminID)
		resp, err := http.Get(e.origin + "/api/v1/auth/status")
		if err != nil {
			t.Fatal(err)
		}
		var body bytes.Buffer
		_, _ = body.ReadFrom(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || strings.TrimSpace(body.String()) != tc.want {
			t.Fatalf("admin %d: status = %d %s, want 200 %s", tc.adminID, resp.StatusCode, body.String(), tc.want)
		}
		if !strings.Contains(resp.Header.Get("Cache-Control"), "no-store") {
			t.Fatalf("status Cache-Control = %q", resp.Header.Get("Cache-Control"))
		}
	}
}
