// Package preflight validates a production configuration before a release (web-v1.md §15.1,
// WEB-008). It reports each check by variable name and never prints a value.
package preflight

import (
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/auth"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/config"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/contact"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/ops"
)

type Level string

const (
	OK   Level = "ok"
	Warn Level = "warn"
	Fail Level = "FAIL"
)

type Result struct {
	Level   Level
	Name    string
	Message string
}

// demo values from .env.example, compose.e2e.yaml and docs: never valid in production.
var demo = []string{"change-me", "e2e-secret", "e2e-client", "re_e2e_fake", "re_test", "example.com", "example.org", "example.test", "localhost"}

func hasDemo(v string) bool {
	l := strings.ToLower(v)
	for _, d := range demo {
		if strings.Contains(l, d) {
			return true
		}
	}
	return false
}

// testOnly variables point integrations at doubles; they must be unset in production.
var testOnly = []string{"GITHUB_AUTHORIZE_URL", "GITHUB_TOKEN_URL", "GITHUB_USER_URL", "RESEND_API_URL", "FAKE_RESEND_API_KEY", "FAKE_USER_ID", "FAKE_USER_LOGIN", "TEST_DATABASE_URL", "AUTHTEST_LOGS"}

// Check runs every production check. writable tests the media root (injectable for tests).
func Check(getenv func(string) string, writable func(string) error) []Result {
	var out []Result
	add := func(l Level, name, format string, args ...any) {
		out = append(out, Result{l, name, fmt.Sprintf(format, args...)})
	}

	origin := getenv("PUBLIC_ORIGIN")
	switch o, err := auth.ParsePublicOrigin(origin); {
	case err != nil:
		add(Fail, "PUBLIC_ORIGIN", "%v", err)
	case !strings.HasPrefix(o, "https://") || hasDemo(o):
		add(Fail, "PUBLIC_ORIGIN", "must be the public https origin (not localhost or an example domain)")
	default:
		add(OK, "PUBLIC_ORIGIN", "https origin")
	}

	for _, v := range []string{"PGHOST", "PGUSER", "PGDATABASE"} {
		if getenv(v) == "" {
			add(Fail, v, "is required")
		} else {
			add(OK, v, "set")
		}
	}
	switch pw := getenv("PGPASSWORD"); {
	case pw == "":
		add(Fail, "PGPASSWORD", "is required")
	case len(pw) < 16 || hasDemo(pw):
		add(Fail, "PGPASSWORD", "must be a strong secret (16+ characters, not the example value)")
	default:
		add(OK, "PGPASSWORD", "set")
	}

	a, err := auth.LoadConfig(getenv)
	switch {
	case err != nil:
		add(Fail, "auth", "%v", err)
	case !a.Enabled():
		add(Fail, "ADMIN_GITHUB_USER_ID/GITHUB_CLIENT_ID/GITHUB_CLIENT_SECRET", "owner login must be configured")
	case hasDemo(a.ClientID) || hasDemo(a.ClientSecret):
		add(Fail, "GITHUB_CLIENT_ID/GITHUB_CLIENT_SECRET", "look like test values")
	default:
		add(OK, "owner login", "configured; callback must be registered as %s", a.CallbackURL())
	}

	for _, v := range testOnly {
		if getenv(v) != "" {
			add(Fail, v, "is test-only and must be unset in production")
		}
	}

	c := contact.LoadConfig(getenv)
	enabledFlag := strings.EqualFold(strings.TrimSpace(getenv("CONTACT_ENABLED")), "true")
	switch {
	case !enabledFlag:
		add(OK, "contact", "disabled (CONTACT_ENABLED is not true)")
	case !c.Enabled:
		add(Fail, "contact", "CONTACT_ENABLED=true but: %s", strings.Join(c.Problems, "; "))
	case hasDemo(getenv("CONTACT_FROM")) || hasDemo(getenv("CONTACT_TO")) || hasDemo(getenv("RESEND_API_KEY")):
		add(Fail, "contact", "CONTACT_FROM/CONTACT_TO/RESEND_API_KEY look like test values")
	default:
		from, _ := mail.ParseAddress(getenv("CONTACT_FROM"))
		add(OK, "contact", "enabled; sender domain %s must be verified in Resend", domainOf(from))
	}
	if getenv("TRUSTED_PROXIES") == "" {
		add(Warn, "TRUSTED_PROXIES", "unset: every private range is trusted; set the Docker network of Coolify's proxy and Caddy")
	}
	if _, err := ops.WebhookURL(getenv("ALERT_WEBHOOK_URL")); err != nil {
		add(Fail, "ALERT_WEBHOOK_URL", "%v", err)
	} else if getenv("ALERT_WEBHOOK_URL") == "" {
		add(Warn, "ALERT_WEBHOOK_URL", "unset: problems are only logged and shown in the panel (Operación)")
	} else {
		add(OK, "ALERT_WEBHOOK_URL", "set")
	}
	if strings.EqualFold(getenv("BACKGROUND_JOBS"), "off") {
		add(Fail, "BACKGROUND_JOBS", "is off: publications and contact would never run (restore environments only)")
	}

	cfg, err := config.Load()
	if err != nil {
		add(Fail, "config", "%v", err)
	} else if err := writable(cfg.MediaRoot); err != nil {
		add(Fail, "STORAGE_LOCAL_ROOT", "media root is not writable: %v", err)
	} else {
		add(OK, "STORAGE_LOCAL_ROOT", "writable")
	}
	return out
}

func domainOf(a *mail.Address) string {
	if a == nil {
		return "?"
	}
	if i := strings.LastIndex(a.Address, "@"); i >= 0 {
		return a.Address[i+1:]
	}
	return "?"
}

// Writable creates and removes a probe file.
func Writable(dir string) error {
	f, err := os.CreateTemp(dir, ".preflight-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(filepath.Clean(name))
}

// Failed reports whether any check failed.
func Failed(rs []Result) bool {
	for _, r := range rs {
		if r.Level == Fail {
			return true
		}
	}
	return false
}
