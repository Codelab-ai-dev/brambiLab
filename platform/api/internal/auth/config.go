// Package auth implements owner-only GitHub OAuth and opaque sessions (web-v1.md §11).
package auth

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAuthorizeURL = "https://github.com/login/oauth/authorize"
	defaultTokenURL     = "https://github.com/login/oauth/access_token"
	defaultUserURL      = "https://api.github.com/user"
	callbackPath        = "/api/v1/auth/github/callback"
)

type Config struct {
	// PublicOrigin is the single public origin (scheme://host[:port]) shared by web and API.
	PublicOrigin string
	AdminUserID  int64
	ClientID     string
	ClientSecret string
	SessionTTL   time.Duration

	// Overridable only for tests and the e2e fake; production uses GitHub's endpoints.
	AuthorizeURL string
	TokenURL     string
	UserURL      string
}

// Enabled reports whether OAuth credentials and the owner ID are configured.
// Without them the API still runs and auth endpoints answer 503.
func (c Config) Enabled() bool {
	return c.AdminUserID > 0 && c.ClientID != "" && c.ClientSecret != ""
}

// Secure is true for https origins: cookies get the Secure flag and the __Host- prefix.
func (c Config) Secure() bool { return strings.HasPrefix(c.PublicOrigin, "https://") }

func (c Config) CallbackURL() string { return c.PublicOrigin + callbackPath }

// LoadConfig reads auth settings through getenv (os.Getenv in production).
func LoadConfig(getenv func(string) string) (Config, error) {
	origin, err := ParsePublicOrigin(getenv("PUBLIC_ORIGIN"))
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		PublicOrigin: origin,
		ClientID:     getenv("GITHUB_CLIENT_ID"),
		ClientSecret: getenv("GITHUB_CLIENT_SECRET"),
		SessionTTL:   12 * time.Hour,
		AuthorizeURL: orDefault(getenv("GITHUB_AUTHORIZE_URL"), defaultAuthorizeURL),
		TokenURL:     orDefault(getenv("GITHUB_TOKEN_URL"), defaultTokenURL),
		UserURL:      orDefault(getenv("GITHUB_USER_URL"), defaultUserURL),
	}
	if v := getenv("ADMIN_GITHUB_USER_ID"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return Config{}, errors.New("ADMIN_GITHUB_USER_ID must be a positive numeric GitHub user ID")
		}
		cfg.AdminUserID = id
	}
	if v := getenv("SESSION_TTL"); v != "" {
		ttl, err := time.ParseDuration(v)
		if err != nil || ttl < 5*time.Minute || ttl > 7*24*time.Hour {
			return Config{}, errors.New("SESSION_TTL must be a duration between 5m and 168h")
		}
		cfg.SessionTTL = ttl
	}
	return cfg, nil
}

// ParsePublicOrigin requires https, except plain http on localhost for development.
func ParsePublicOrigin(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("PUBLIC_ORIGIN is required (e.g. https://brambilab.example or http://localhost:8000)")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", errors.New("PUBLIC_ORIGIN must be scheme://host[:port] without path")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if h := u.Hostname(); h != "localhost" && h != "127.0.0.1" {
			return "", errors.New("PUBLIC_ORIGIN must use https outside localhost")
		}
	default:
		return "", errors.New("PUBLIC_ORIGIN must use https (or http on localhost)")
	}
	return u.Scheme + "://" + u.Host, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
