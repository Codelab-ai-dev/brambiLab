package auth

import (
	"net/http"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestParsePublicOrigin(t *testing.T) {
	ok := map[string]string{
		"https://brambilab.example":  "https://brambilab.example",
		"https://brambilab.example/": "https://brambilab.example",
		"http://localhost:8000":      "http://localhost:8000",
		"http://127.0.0.1:8000":      "http://127.0.0.1:8000",
	}
	for in, want := range ok {
		got, err := ParsePublicOrigin(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "http://brambilab.example", "ftp://x", "https://x/path", "https://u:p@x", "https://x?q=1", "localhost:8000"} {
		if _, err := ParsePublicOrigin(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{"PUBLIC_ORIGIN": "http://localhost:8000"}))
	if err != nil || cfg.Enabled() || cfg.SessionTTL != 12*time.Hour || cfg.AuthorizeURL != defaultAuthorizeURL {
		t.Fatalf("defaults: %+v, %v", cfg, err)
	}
	cfg, err = LoadConfig(env(map[string]string{
		"PUBLIC_ORIGIN": "https://b.example", "ADMIN_GITHUB_USER_ID": "201345228",
		"GITHUB_CLIENT_ID": "id", "GITHUB_CLIENT_SECRET": "secret", "SESSION_TTL": "2h",
	}))
	if err != nil || !cfg.Enabled() || cfg.AdminUserID != 201345228 || cfg.SessionTTL != 2*time.Hour ||
		cfg.CallbackURL() != "https://b.example/api/v1/auth/github/callback" {
		t.Fatalf("configured: %+v, %v", cfg, err)
	}
	for _, bad := range []map[string]string{
		{"PUBLIC_ORIGIN": "http://localhost:8000", "ADMIN_GITHUB_USER_ID": "octocat"},
		{"PUBLIC_ORIGIN": "http://localhost:8000", "ADMIN_GITHUB_USER_ID": "-1"},
		{"PUBLIC_ORIGIN": "http://localhost:8000", "SESSION_TTL": "1m"},
	} {
		if _, err := LoadConfig(env(bad)); err == nil {
			t.Errorf("%v: expected error", bad)
		}
	}
}

func TestSafeReturnTo(t *testing.T) {
	cases := map[string]string{
		"": "/admin", "/admin": "/admin", "/admin/posts?x=1": "/admin/posts?x=1",
		"/es": "/admin", "https://evil.example/admin": "/admin", "//evil.example": "/admin",
		"/admin//evil.example": "/admin", "/admin/\\evil": "/admin", "/administrator": "/admin",
	}
	for in, want := range cases {
		if got := safeReturnTo(in); got != want {
			t.Errorf("safeReturnTo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPKCEChallengeRFC7636Vector(t *testing.T) {
	// RFC 7636 Appendix B.
	if got := pkceChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"); got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("challenge = %q", got)
	}
}

func TestCookieAttributes(t *testing.T) {
	secure := &Handler{cfg: Config{PublicOrigin: "https://b.example"}}
	c := secure.cookie(sessionCookieName, "v", time.Hour)
	if c.Name != "__Host-bl_session" || !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" ||
		c.SameSite != http.SameSiteLaxMode || c.MaxAge != 3600 {
		t.Fatalf("https cookie: %+v", c)
	}
	local := &Handler{cfg: Config{PublicOrigin: "http://localhost:8000"}}
	c = local.cookie(sessionCookieName, "", -1)
	if c.Name != "bl_session" || c.Secure || !c.HttpOnly || c.MaxAge != -1 {
		t.Fatalf("localhost deletion cookie: %+v", c)
	}
}
