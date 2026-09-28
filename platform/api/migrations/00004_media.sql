-- Media library and revision references (web-v1.md §12.1, WEB-004).

-- +goose Up
CREATE TABLE assets (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind            text        NOT NULL CHECK (kind IN ('image', 'video', 'resource')),
    storage_backend text        NOT NULL DEFAULT 'local',
    object_key      text        NOT NULL UNIQUE,
    original_name   text        NOT NULL,
    mime            text        NOT NULL DEFAULT '',
    bytes           bigint      NOT NULL DEFAULT 0,
    sha256          bytea,
    width           integer,
    height          integer,
    status          text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'failed')),
    failure         text,
    public_enabled  boolean     NOT NULL DEFAULT false,
    downloadable    boolean     NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    -- A ready asset always has verified bytes.
    CONSTRAINT assets_ready_complete CHECK (status <> 'ready' OR (sha256 IS NOT NULL AND bytes > 0 AND mime <> ''))
);
CREATE INDEX assets_created_idx ON assets (created_at DESC);
CREATE INDEX assets_pending_idx ON assets (created_at) WHERE status = 'pending';

-- Library defaults per locale. Documents keep their own alt/caption in each revision snapshot.
CREATE TABLE asset_translations (
    asset_id uuid NOT NULL REFERENCES assets (id) ON DELETE CASCADE,
    locale   text NOT NULL CHECK (locale IN ('es', 'en')),
    alt      text NOT NULL DEFAULT '',
    caption  text NOT NULL DEFAULT '',
    PRIMARY KEY (asset_id, locale)
);

-- Which retained revision uses which asset, and how. RESTRICT: referenced assets cannot be deleted.
CREATE TABLE revision_assets (
    revision_id uuid NOT NULL REFERENCES revisions (id),
    asset_id    uuid NOT NULL REFERENCES assets (id) ON DELETE RESTRICT,
    usage       text NOT NULL CHECK (usage IN ('image', 'video', 'poster', 'download', 'cover')),
    PRIMARY KEY (revision_id, asset_id, usage)
);
CREATE INDEX revision_assets_asset_idx ON revision_assets (asset_id);

-- References are part of the immutable snapshot.
CREATE TRIGGER revision_assets_immutable BEFORE UPDATE OR DELETE ON revision_assets
    FOR EACH ROW EXECUTE FUNCTION forbid_revision_change();

ALTER TABLE revisions ADD COLUMN cover_asset_id uuid REFERENCES assets (id);

-- +goose Down
ALTER TABLE revisions DROP COLUMN cover_asset_id;
DROP TABLE revision_assets;
DROP TABLE asset_translations;
DROP TABLE assets;
