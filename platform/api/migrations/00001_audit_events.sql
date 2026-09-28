-- Baseline: audit trail shared by every module (web-v1.md §6).
-- Never store tokens or message bodies in metadata.

-- +goose Up
CREATE TABLE audit_events (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    actor       text        NOT NULL,
    action      text        NOT NULL,
    entity_type text        NOT NULL,
    entity_id   uuid,
    metadata    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_entity_idx ON audit_events (entity_type, entity_id, created_at DESC);

-- +goose Down
DROP TABLE audit_events;
