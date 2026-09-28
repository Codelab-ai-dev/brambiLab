-- Public site: search projection and site settings (web-v1.md §9.1, WEB-006).

-- +goose Up
CREATE EXTENSION IF NOT EXISTS unaccent;

-- Spanish/English stemming that ignores accents ("energia" finds "energía").
CREATE TEXT SEARCH CONFIGURATION bl_es (COPY = pg_catalog.spanish);
ALTER TEXT SEARCH CONFIGURATION bl_es ALTER MAPPING FOR hword, hword_part, word WITH unaccent, spanish_stem;
CREATE TEXT SEARCH CONFIGURATION bl_en (COPY = pg_catalog.english);
ALTER TEXT SEARCH CONFIGURATION bl_en ALTER MAPPING FOR hword, hword_part, word WITH unaccent, english_stem;

-- One row per published translation, derived only from the published revision. Queries still
-- join visible_translations: this projection is an index, never the visibility rule.
CREATE TABLE search_documents (
    translation_id uuid        PRIMARY KEY REFERENCES translations (id),
    revision_id    uuid        NOT NULL REFERENCES revisions (id),
    locale         text        NOT NULL CHECK (locale IN ('es', 'en')),
    search         tsvector    NOT NULL,
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX search_documents_search_idx ON search_documents USING gin (search);

-- Weights: title A, summary B, body text C.
-- +goose StatementBegin
CREATE FUNCTION search_vector(rev uuid, loc text) RETURNS tsvector LANGUAGE sql STABLE AS $$
    SELECT setweight(to_tsvector(cfg, r.title), 'A')
        || setweight(to_tsvector(cfg, r.summary), 'B')
        || setweight(to_tsvector(cfg, r.plain_text), 'C')
    FROM revisions r, (SELECT (CASE loc WHEN 'en' THEN 'bl_en' ELSE 'bl_es' END)::regconfig AS cfg) c
    WHERE r.id = rev
$$;
-- +goose StatementEnd

-- Kept in the same transaction as every change of the public pointer (manual publish, scheduler,
-- withdraw), whoever makes it.
-- +goose StatementBegin
CREATE FUNCTION sync_search_document() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.published_revision_id IS NULL THEN
        DELETE FROM search_documents WHERE translation_id = NEW.id;
    ELSE
        INSERT INTO search_documents (translation_id, revision_id, locale, search, updated_at)
        VALUES (NEW.id, NEW.published_revision_id, NEW.locale, search_vector(NEW.published_revision_id, NEW.locale), now())
        ON CONFLICT (translation_id) DO UPDATE
            SET revision_id = EXCLUDED.revision_id, search = EXCLUDED.search, updated_at = now();
    END IF;
    RETURN NULL;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER translations_search_insert AFTER INSERT ON translations
    FOR EACH ROW WHEN (NEW.published_revision_id IS NOT NULL) EXECUTE FUNCTION sync_search_document();
CREATE TRIGGER translations_search_update AFTER UPDATE OF published_revision_id ON translations
    FOR EACH ROW WHEN (OLD.published_revision_id IS DISTINCT FROM NEW.published_revision_id) EXECUTE FUNCTION sync_search_document();

-- Backfill the publications made before WEB-006.
INSERT INTO search_documents (translation_id, revision_id, locale, search)
SELECT id, published_revision_id, locale, search_vector(published_revision_id, locale)
FROM translations WHERE published_revision_id IS NOT NULL;

-- Listings order by first publication.
CREATE INDEX translations_public_order_idx ON translations (locale, first_published_at DESC, content_id DESC)
    WHERE published_revision_id IS NOT NULL;

-- Site settings: a single row, saved explicitly with optimistic versioning (no revisions).
CREATE TABLE site_settings (
    id            boolean     PRIMARY KEY DEFAULT true CHECK (id),
    version       integer     NOT NULL DEFAULT 0 CHECK (version >= 0),
    intro_es      text        NOT NULL DEFAULT '',
    intro_en      text        NOT NULL DEFAULT '',
    bio_es        text        NOT NULL DEFAULT '',
    bio_en        text        NOT NULL DEFAULT '',
    contact_email text        NOT NULL DEFAULT '',
    links         jsonb       NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(links) = 'array'),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
INSERT INTO site_settings DEFAULT VALUES;

-- Featured projects, in order. Selecting one never publishes it: it shows only while visible.
CREATE TABLE site_featured_projects (
    content_id uuid    PRIMARY KEY REFERENCES contents (id),
    position   integer NOT NULL UNIQUE CHECK (position >= 0)
);

-- +goose Down
DROP TABLE site_featured_projects;
DROP TABLE site_settings;
DROP INDEX translations_public_order_idx;
DROP TRIGGER translations_search_update ON translations;
DROP TRIGGER translations_search_insert ON translations;
DROP FUNCTION sync_search_document();
DROP FUNCTION search_vector(uuid, text);
DROP TABLE search_documents;
DROP TEXT SEARCH CONFIGURATION bl_en;
DROP TEXT SEARCH CONFIGURATION bl_es;
DROP EXTENSION IF EXISTS unaccent;
