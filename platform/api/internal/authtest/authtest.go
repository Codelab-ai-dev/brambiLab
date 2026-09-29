// Package authtest runs the real router with owner auth against the fake GitHub, so integration
// tests of protected modules exercise genuine sessions, CSRF and same-origin checks. Test-only.
package authtest

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/fakegithub"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

const OwnerID = 201345228

// Server is a running API with an owner already logged in.
type Server struct {
	t      *testing.T
	Origin string
	Owner  *http.Client // cookie jar holding the owner session
	CSRF   string
}

// Start mounts auth plus the modules built by mount (which receives the auth handler).
func Start(t *testing.T, pool *pgxpool.Pool, mount func(a *auth.Handler, logger *slog.Logger) []httpapi.Module) *Server {
	return StartWrapped(t, pool, nil, mount)
}

// StartWrapped is Start with a router wrapper (e.g. the maintenance gate).
func StartWrapped(t *testing.T, pool *pgxpool.Pool, wrap func(http.Handler) http.Handler, mount func(a *auth.Handler, logger *slog.Logger) []httpapi.Module) *Server {
	t.Helper()
	fake := fakegithub.New("test-client", "test-secret", fakegithub.User{ID: OwnerID, Login: "owner"})
	gh := httptest.NewServer(fake.Handler())
	t.Cleanup(gh.Close)

	api := httptest.NewUnstartedServer(nil)
	origin := "http://" + api.Listener.Addr().String()
	logger := slog.New(slog.NewTextHandler(logSink(), nil))
	a := auth.NewHandler(auth.Config{
		PublicOrigin: origin, AdminUserID: OwnerID, ClientID: "test-client", ClientSecret: "test-secret",
		SessionTTL:   time.Hour,
		AuthorizeURL: gh.URL + "/login/oauth/authorize", TokenURL: gh.URL + "/login/oauth/access_token", UserURL: gh.URL + "/user",
	}, pool, logger)
	modules := append([]httpapi.Module{a}, mount(a, logger)...)
	api.Config.Handler = httpapi.NewRouter(logger, origin, wrap, modules...)
	api.Start()
	t.Cleanup(api.Close)

	jar, _ := cookiejar.New(nil)
	owner := &http.Client{Jar: jar, CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if strings.HasPrefix(origin, req.URL.Scheme+"://"+req.URL.Host) && !strings.HasPrefix(req.URL.Path, "/api/") {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	resp, err := owner.Get(origin + "/api/v1/auth/github/start")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp, err = owner.Get(origin + "/api/v1/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var me struct {
		CSRFToken string `json:"csrf_token"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&me) != nil {
		t.Fatalf("owner login failed: %d", resp.StatusCode)
	}
	return &Server{t: t, Origin: origin, Owner: owner, CSRF: me.CSRFToken}
}

// Response is a decoded API answer.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

func (r Response) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %s: %v", r.Body, err)
	}
}

// Do sends a request as the owner (session cookie, CSRF token and same Origin on mutations).
func (s *Server) Do(method, path string, body any, headers ...string) Response {
	return s.do(s.Owner, true, method, path, body, headers...)
}

// Anonymous sends the request without any session.
func (s *Server) Anonymous(method, path string, body any) Response {
	return s.do(http.DefaultClient, true, method, path, body)
}

// WithoutCSRF sends as the owner but omits the CSRF token.
func (s *Server) WithoutCSRF(method, path string, body any) Response {
	return s.do(s.Owner, false, method, path, body)
}

func (s *Server) do(c *http.Client, csrf bool, method, path string, body any, headers ...string) Response {
	s.t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = strings.NewReader(b)
	case []byte:
		reader = strings.NewReader(string(b))
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			s.t.Fatal(err)
		}
		reader = strings.NewReader(string(raw))
	}
	req, err := http.NewRequest(method, s.Origin+path, reader)
	if err != nil {
		s.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if method != http.MethodGet {
		req.Header.Set("Origin", s.Origin)
		if csrf {
			req.Header.Set("X-CSRF-Token", s.CSRF)
		}
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return Response{Status: resp.StatusCode, Header: resp.Header, Body: raw}
}

// logSink discards server logs unless AUTHTEST_LOGS=1 (debugging failing tests).
func logSink() io.Writer {
	if os.Getenv("AUTHTEST_LOGS") == "1" {
		return os.Stderr
	}
	return io.Discard
}
