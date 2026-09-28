package auth

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
)

const (
	sessionCookieName = "bl_session"
	stateCookieName   = "bl_oauth_state"
	csrfHeader        = "X-CSRF-Token"
	csrfFormField     = "csrf_token"
	maxFormBytes      = 64 << 10
)

type sessionKey struct{}

// SessionFrom returns the session set by RequireOwner.
func SessionFrom(ctx context.Context) Session {
	s, _ := ctx.Value(sessionKey{}).(Session)
	return s
}

// Actor names the session owner for audit events ("github:<id>"). Only valid behind RequireOwner.
func Actor(r *http.Request) string {
	return "github:" + strconv.FormatInt(SessionFrom(r.Context()).GitHubUserID, 10)
}

// IsOwner reports whether the request carries a live owner session, without writing a response.
// Used where anonymous access is also possible (media delivery); authorization stays in Go.
func (h *Handler) IsOwner(r *http.Request) bool {
	if !h.cfg.Enabled() {
		return false
	}
	c, err := r.Cookie(h.name(sessionCookieName))
	if err != nil || c.Value == "" {
		return false
	}
	_, err = h.store.activeSession(r.Context(), c.Value, h.cfg.AdminUserID)
	return err == nil
}

// SessionCookieName is the cookie the web SSR must forward to the internal API.
func (h *Handler) SessionCookieName() string { return h.name(sessionCookieName) }

// RequireOwner rejects requests without a live owner session, and unsafe requests without the
// session's CSRF token. Hiding buttons in the UI is never the authorization (web-v1.md §10).
// Same-origin enforcement for unsafe methods happens earlier, in httpapi.
func (h *Handler) RequireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		private(w)
		if !h.cfg.Enabled() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "auth_not_configured", "Owner login is not configured")
			return
		}
		c, err := r.Cookie(h.name(sessionCookieName))
		if err != nil || c.Value == "" {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthenticated", "Owner session required")
			return
		}
		sess, err := h.store.activeSession(r.Context(), c.Value, h.cfg.AdminUserID)
		if errors.Is(err, errNotFound) {
			http.SetCookie(w, h.cookie(sessionCookieName, "", -1))
			httpapi.WriteError(w, r, http.StatusUnauthorized, "unauthenticated", "Owner session required")
			return
		}
		if err != nil {
			h.fail(w, r, "load session", err)
			return
		}
		if !httpapi.IsSafeMethod(r.Method) && !equalTokens(sess.CSRFToken, csrfToken(r)) {
			httpapi.WriteError(w, r, http.StatusForbidden, "csrf_failed", "Missing or invalid CSRF token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, sess)))
	})
}

// csrfToken accepts the header (fetch) or a hidden field (plain HTML form without JavaScript).
func csrfToken(r *http.Request) string {
	if v := r.Header.Get(csrfHeader); v != "" {
		return v
	}
	if isForm(r) {
		r.Body = http.MaxBytesReader(nil, r.Body, maxFormBytes)
		if err := r.ParseForm(); err == nil {
			return r.PostForm.Get(csrfFormField)
		}
	}
	return ""
}

func isForm(r *http.Request) bool {
	mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return mt == "application/x-www-form-urlencoded"
}

// name adds the __Host- prefix on https: Secure, Path=/ and no Domain are then enforced by browsers.
func (h *Handler) name(base string) string {
	if h.cfg.Secure() {
		return "__Host-" + base
	}
	return base
}

// cookie builds HttpOnly, SameSite=Lax, Path=/ cookies. A negative ttl deletes the cookie.
func (h *Handler) cookie(base, value string, ttl time.Duration) *http.Cookie {
	c := &http.Cookie{
		Name:     h.name(base),
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.Secure(),
		SameSite: http.SameSiteLaxMode,
	}
	if ttl < 0 {
		c.MaxAge = -1
	} else {
		c.MaxAge = int(ttl.Seconds())
	}
	return c
}
