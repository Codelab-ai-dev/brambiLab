package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errNotFound = errors.New("not found")

// Session is the authenticated owner session attached to a request context.
type Session struct {
	ID           string
	GitHubUserID int64
	GitHubLogin  string
	CSRFToken    string
	ExpiresAt    time.Time
}

type store struct{ pool *pgxpool.Pool }

func (s store) saveState(ctx context.Context, state, verifier, returnTo string, expires time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO oauth_states (state_hash, code_verifier, return_to, expires_at) VALUES ($1, $2, $3, $4)`,
		hashToken(state), verifier, returnTo, expires)
	return err
}

// consumeState deletes and returns a state atomically, so each one works exactly once.
func (s store) consumeState(ctx context.Context, state string) (verifier, returnTo string, err error) {
	err = s.pool.QueryRow(ctx,
		`DELETE FROM oauth_states WHERE state_hash = $1 AND expires_at > now() RETURNING code_verifier, return_to`,
		hashToken(state)).Scan(&verifier, &returnTo)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", errNotFound
	}
	return verifier, returnTo, err
}

// createSession stores the session and its audit event in one transaction.
func (s store) createSession(ctx context.Context, token string, u githubUser, ttl time.Duration) (Session, error) {
	sess := Session{GitHubUserID: u.ID, GitHubLogin: u.Login, CSRFToken: randomToken()}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO sessions (token_hash, csrf_token, github_user_id, github_login, expires_at)
			 VALUES ($1, $2, $3, $4, now() + make_interval(secs => $5)) RETURNING id, expires_at`,
			hashToken(token), sess.CSRFToken, u.ID, u.Login, ttl.Seconds()).Scan(&sess.ID, &sess.ExpiresAt); err != nil {
			return err
		}
		return audit(ctx, tx, u.ID, "auth.login", "session", &sess.ID, nil)
	})
	return sess, err
}

// activeSession returns a live, unrevoked session that still belongs to the configured owner.
func (s store) activeSession(ctx context.Context, token string, ownerID int64) (Session, error) {
	var sess Session
	err := s.pool.QueryRow(ctx,
		`SELECT id, github_user_id, github_login, csrf_token, expires_at FROM sessions
		 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now() AND github_user_id = $2`,
		hashToken(token), ownerID).Scan(&sess.ID, &sess.GitHubUserID, &sess.GitHubLogin, &sess.CSRFToken, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, errNotFound
	}
	return sess, err
}

func (s store) revokeSession(ctx context.Context, sess Session) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, sess.ID); err != nil {
			return err
		}
		return audit(ctx, tx, sess.GitHubUserID, "auth.logout", "session", &sess.ID, nil)
	})
}

func (s store) recordRejected(ctx context.Context, u githubUser) error {
	return audit(ctx, s.pool, u.ID, "auth.login_rejected", "auth", nil, map[string]string{"github_login": u.Login})
}

// cleanup removes expired OAuth states and sessions expired for more than 30 days.
func (s store) cleanup(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM oauth_states WHERE expires_at <= now()`); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now() - interval '30 days'`)
	return err
}

// execer is satisfied by both *pgxpool.Pool and pgx.Tx.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// audit records who did what. Metadata must never contain tokens or secrets.
func audit(ctx context.Context, db execer, actorID int64, action, entityType string, entityID *string, metadata map[string]string) error {
	meta, err := json.Marshal(metadata)
	if err != nil || metadata == nil {
		meta = []byte("{}")
	}
	_, err = db.Exec(ctx,
		`INSERT INTO audit_events (actor, action, entity_type, entity_id, metadata) VALUES ($1, $2, $3, $4, $5)`,
		"github:"+strconv.FormatInt(actorID, 10), action, entityType, entityID, meta)
	return err
}
