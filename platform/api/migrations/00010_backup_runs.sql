-- Backup runs (web-v1.md §15.2, WEB-008): mutual exclusion and the facts alerts need (last
-- success, duration, size, failure). No personal data or secrets.

-- +goose Up
CREATE TABLE ops_backup_runs (
    id           bigserial   PRIMARY KEY,
    started_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz,
    status       text        NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'succeeded', 'failed')),
    name         text,
    bytes        bigint,
    commit_sha   text,
    schema_version bigint,
    assets       integer,
    step         text,
    error        text
);
-- One backup at a time.
CREATE UNIQUE INDEX ops_backup_runs_one_running ON ops_backup_runs ((true)) WHERE status = 'running';

-- +goose Down
DROP TABLE ops_backup_runs;
