package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/httpapi"
	"github.com/Codelab-ai-dev/brambiLab/platform/api/internal/ops"
)

const stateTTL = 10 * time.Minute

// Handler serves /api/v1/auth/* and provides RequireOwner for protected routes.
type Handler struct {
	cfg    Config
	store  store
	github githubClient
	logger *slog.Logger
	// Gate pauses the session cleanup (maintenance, BACKGROUND_JOBS=off).
	Gate ops.Gate
}

func NewHandler(cfg Config, pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{
		cfg:    cfg,
		store:  store{pool: pool},
		github: githubClient{cfg: cfg, http: &http.Client{Timeout: 10 * time.Second}},
		logger: logger,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/status", h.status)
	mux.HandleFunc("GET /api/v1/auth/github/start", h.enabled(h.start))
	mux.HandleFunc("GET /api/v1/auth/github/callback", h.enabled(h.callback))
	mux.Handle("GET /api/v1/auth/me", h.RequireOwner(http.HandlerFunc(h.me)))
	mux.Handle("POST /api/v1/auth/logout", h.RequireOwner(http.HandlerFunc(h.logout)))
}

func (h *Handler) enabled(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		private(w)
		if !h.cfg.Enabled() {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "auth_not_configured", "Owner login is not configured")
			return
		}
		next(w, r)
	}
}

// start begins Authorization Code + PKCE. The state is single-use, stored hashed in PostgreSQL
// and bound to this browser through a short-lived cookie (login CSRF protection).
func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	state, verifier := randomToken(), randomToken()
	returnTo := safeReturnTo(r.URL.Query().Get("return_to"))
	if err := h.store.saveState(r.Context(), state, verifier, returnTo, time.Now().Add(stateTTL)); err != nil {
		h.fail(w, r, "save oauth state", err)
		return
	}
	http.SetCookie(w, h.cookie(stateCookieName, state, stateTTL))
	http.Redirect(w, r, h.github.authorizeURL(state, verifier), http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	http.SetCookie(w, h.cookie(stateCookieName, "", -1))

	cookie, _ := r.Cookie(h.name(stateCookieName))
	state := q.Get("state")
	if cookie == nil || !equalTokens(cookie.Value, state) {
		h.toLogin(w, r, "failed", "oauth state mismatch", nil)
		return
	}
	verifier, returnTo, err := h.store.consumeState(r.Context(), state)
	if err != nil {
		h.toLogin(w, r, "failed", "oauth state not found or expired", err)
		return
	}
	if q.Get("error") != "" {
		// GitHub reports denial (e.g. access_denied) on the callback.
		h.toLogin(w, r, "denied", "github returned an error", nil)
		return
	}
	code := q.Get("code")
	if code == "" {
		h.toLogin(w, r, "failed", "callback without code", nil)
		return
	}

	token, err := h.github.exchange(r.Context(), code, verifier)
	if err != nil {
		h.toLogin(w, r, "failed", "github token exchange failed", err)
		return
	}
	user, err := h.github.user(r.Context(), token)
	if err != nil {
		h.toLogin(w, r, "failed", "github user lookup failed", err)
		return
	}

	// Authorization is the numeric ID only: never the login name or e-mail.
	if user.ID != h.cfg.AdminUserID {
		if err := h.store.recordRejected(r.Context(), user); err != nil {
			h.logger.Error("audit login rejection", "request_id", httpapi.RequestID(r.Context()), "error", err)
		}
		h.toLogin(w, r, "forbidden", "github account is not the owner", nil, "github_user_id", user.ID)
		return
	}

	sessionToken := randomToken()
	sess, err := h.store.createSession(r.Context(), sessionToken, user, h.cfg.SessionTTL)
	if err != nil {
		h.fail(w, r, "create session", err)
		return
	}
	http.SetCookie(w, h.cookie(sessionCookieName, sessionToken, h.cfg.SessionTTL))
	h.logger.Info("owner login", "request_id", httpapi.RequestID(r.Context()), "session_id", sess.ID)
	http.Redirect(w, r, h.cfg.PublicOrigin+returnTo, http.StatusFound)
}

type statusResponse struct {
	LoginEnabled bool `json:"login_enabled"`
}

// status tells the login page whether to offer the GitHub button. It exposes a single boolean,
// never which setting is missing.
func (h *Handler) status(w http.ResponseWriter, _ *http.Request) {
	private(w)
	httpapi.WriteJSON(w, http.StatusOK, statusResponse{LoginEnabled: h.cfg.Enabled()})
}

type meResponse struct {
	GitHubUserID int64     `json:"github_user_id"`
	GitHubLogin  string    `json:"github_login"`
	ExpiresAt    time.Time `json:"expires_at"`
	CSRFToken    string    `json:"csrf_token"`
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r.Context())
	httpapi.WriteJSON(w, http.StatusOK, meResponse{
		GitHubUserID: sess.GitHubUserID, GitHubLogin: sess.GitHubLogin,
		ExpiresAt: sess.ExpiresAt, CSRFToken: sess.CSRFToken,
	})
}

// logout revokes the session server-side. HTML form posts get a redirect; fetch calls get 204.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.store.revokeSession(r.Context(), SessionFrom(r.Context())); err != nil {
		h.fail(w, r, "revoke session", err)
		return
	}
	http.SetCookie(w, h.cookie(sessionCookieName, "", -1))
	if isForm(r) {
		http.Redirect(w, r, h.cfg.PublicOrigin+"/admin/login?logged_out=1", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// toLogin sends the browser back to the login page with a coarse reason; details stay in logs.
func (h *Handler) toLogin(w http.ResponseWriter, r *http.Request, reason, msg string, err error, attrs ...any) {
	args := append([]any{"request_id", httpapi.RequestID(r.Context()), "reason", reason}, attrs...)
	if err != nil {
		args = append(args, "error", err)
	}
	h.logger.Warn("owner login refused: "+msg, args...)
	http.Redirect(w, r, h.cfg.PublicOrigin+"/admin/login?error="+reason, http.StatusFound)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.Error(op, "request_id", httpapi.RequestID(r.Context()), "error", err)
	httpapi.WriteError(w, r, http.StatusInternalServerError, "internal", "Internal server error")
}

// RunCleanup deletes expired OAuth states and old sessions until ctx ends.
func (h *Handler) RunCleanup(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if done, ok := h.Gate.Try(); ok {
			if err := h.store.cleanup(ctx); err != nil && !errors.Is(err, context.Canceled) {
				h.logger.Warn("auth cleanup failed", "error", err)
			}
			done()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func private(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Vary", "Cookie")
}
