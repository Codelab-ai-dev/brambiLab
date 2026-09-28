-- Private editorial content, translations and immutable revisions (web-v1.md §6, §6.1).
-- Database constraints back the rules the API enforces, so a bug cannot silently break them.

-- +goose Up
CREATE TABLE contents (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        text        NOT NULL CHECK (kind IN ('project', 'article', 'log')),
    project_id  uuid,
    -- Constant 'project' whenever there is a parent, so the composite FK below only accepts projects.
    parent_kind text        GENERATED ALWAYS AS (CASE WHEN project_id IS NULL THEN NULL ELSE 'project' END) STORED,
    created_at  timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    UNIQUE (id, kind),
    CONSTRAINT contents_log_has_parent CHECK ((kind = 'log') = (project_id IS NOT NULL)),
    CONSTRAINT contents_parent_is_project FOREIGN KEY (project_id, parent_kind) REFERENCES contents (id, kind)
);
CREATE INDEX contents_kind_created_idx ON contents (kind, created_at DESC);
CREATE INDEX contents_project_idx ON contents (project_id) WHERE project_id IS NOT NULL;

CREATE TABLE translations (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    content_id            uuid        NOT NULL REFERENCES contents (id),
    locale                text        NOT NULL CHECK (locale IN ('es', 'en')),
    latest_version        integer     NOT NULL DEFAULT 0 CHECK (latest_version >= 0),
    published_revision_id uuid,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    UNIQUE (content_id, locale)
);

CREATE TABLE categories (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text        NOT NULL UNIQUE,
    label_es   text        NOT NULL,
    label_en   text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tags (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text        NOT NULL UNIQUE,
    label_es   text        NOT NULL,
    label_en   text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE revisions (
    id                    uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    translation_id        uuid        NOT NULL REFERENCES translations (id),
    version               integer     NOT NULL CHECK (version >= 1),
    kind                  text        NOT NULL CHECK (kind IN ('manual', 'auto', 'restore', 'copy')),
    restored_from_version integer,
    copied_from_locale    text        CHECK (copied_from_locale IN ('es', 'en')),
    title                 text        NOT NULL,
    slug                  text        NOT NULL,
    summary               text        NOT NULL DEFAULT '',
    body_json             jsonb       NOT NULL,
    body_schema_version   integer     NOT NULL,
    plain_text            text        NOT NULL,
    seo                   jsonb       NOT NULL DEFAULT '{}'::jsonb,
    project_fields        jsonb,
    category_id           uuid        REFERENCES categories (id),
    snapshot_hash         bytea       NOT NULL,
    idempotency_key       text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    UNIQUE (translation_id, version),
    UNIQUE (id, translation_id),
    UNIQUE (translation_id, idempotency_key),
    CONSTRAINT revisions_restore_source CHECK ((kind = 'restore') = (restored_from_version IS NOT NULL)),
    CONSTRAINT revisions_copy_source CHECK ((kind = 'copy') = (copied_from_locale IS NOT NULL))
);

CREATE TABLE revision_tags (
    revision_id uuid NOT NULL REFERENCES revisions (id),
    tag_id      uuid NOT NULL REFERENCES tags (id),
    PRIMARY KEY (revision_id, tag_id)
);

-- A translation can only point at one of its own revisions as published (WEB-005 sets it).
ALTER TABLE translations
    ADD CONSTRAINT translations_published_revision_fk
    FOREIGN KEY (published_revision_id, id) REFERENCES revisions (id, translation_id);

-- +goose StatementBegin
CREATE FUNCTION forbid_revision_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'revisions are immutable (% on %)', TG_OP, TG_TABLE_NAME
        USING ERRCODE = 'restrict_violation';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER revisions_immutable BEFORE UPDATE OR DELETE ON revisions
    FOR EACH ROW EXECUTE FUNCTION forbid_revision_change();
CREATE TRIGGER revision_tags_immutable BEFORE UPDATE OR DELETE ON revision_tags
    FOR EACH ROW EXECUTE FUNCTION forbid_revision_change();

-- +goose StatementBegin
CREATE FUNCTION forbid_content_identity_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.kind IS DISTINCT FROM OLD.kind OR NEW.project_id IS DISTINCT FROM OLD.project_id THEN
        RAISE EXCEPTION 'content kind and parent cannot change' USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER contents_identity_immutable BEFORE UPDATE ON contents
    FOR EACH ROW EXECUTE FUNCTION forbid_content_identity_change();

-- +goose Down
DROP TABLE revision_tags;
ALTER TABLE translations DROP CONSTRAINT translations_published_revision_fk;
DROP TABLE revisions;
DROP TABLE tags;
DROP TABLE categories;
DROP TABLE translations;
DROP TABLE contents;
DROP FUNCTION forbid_revision_change();
DROP FUNCTION forbid_content_identity_change();
