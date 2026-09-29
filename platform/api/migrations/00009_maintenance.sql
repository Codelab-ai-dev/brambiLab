-- Maintenance switch for consistent backups (web-v1.md §15.1, WEB-008): while active the API
-- rejects writes and pauses background jobs; it acknowledges once nothing is in flight.

-- +goose Up
CREATE TABLE app_maintenance (
    id              boolean     PRIMARY KEY DEFAULT true CHECK (id),
    active          boolean     NOT NULL DEFAULT false,
    reason          text        NOT NULL DEFAULT '',
    activated_at    timestamptz,
    acknowledged_at timestamptz
);
INSERT INTO app_maintenance DEFAULT VALUES;

-- +goose Down
DROP TABLE app_maintenance;
