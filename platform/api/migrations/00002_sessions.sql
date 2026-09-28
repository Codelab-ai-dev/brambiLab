-- Owner sessions and single-use OAuth states (web-v1.md §11).
-- Only SHA-256 hashes of bearer values are stored; GitHub tokens are never stored.

-- +goose Up
CREATE TABLE oauth_states (
    state_hash    bytea       PRIMARY KEY,
    code_verifier text        NOT NULL,
    return_to     text        NOT NULL,
    expires_at    timestamptz NOT NULL
);

CREATE TABLE sessions (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash     bytea       NOT NULL UNIQUE,
    csrf_token     text        NOT NULL,
    github_user_id bigint      NOT NULL,
    github_login   text        NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    expires_at     timestamptz NOT NULL,
    revoked_at     timestamptz
);

CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
DROP TABLE oauth_states;
