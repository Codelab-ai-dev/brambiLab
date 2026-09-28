-- Publication, scheduling and withdrawal per translation (web-v1.md §8.1, WEB-005).

-- +goose Up
ALTER TABLE translations
    ADD COLUMN editorial_version  integer     NOT NULL DEFAULT 0 CHECK (editorial_version >= 0),
    ADD COLUMN published_at       timestamptz,
    ADD COLUMN first_published_at timestamptz,
    ADD COLUMN withdrawn_at       timestamptz;
-- Pointers set before WEB-005 get a date so the invariant below holds.
UPDATE translations SET published_at = updated_at, first_published_at = updated_at WHERE published_revision_id IS NOT NULL;
ALTER TABLE translations
    ADD CONSTRAINT translations_published_has_date CHECK (published_revision_id IS NULL OR published_at IS NOT NULL);

-- Scheduled publications. The revision is frozen; (revision_id, translation_id) must match.
CREATE TABLE publication_jobs (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    translation_id  uuid        NOT NULL REFERENCES translations (id),
    revision_id     uuid        NOT NULL,
    run_at          timestamptz NOT NULL,
    status          text        NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'succeeded', 'failed', 'cancelled')),
    attempts        integer     NOT NULL DEFAULT 0,
    max_attempts    integer     NOT NULL DEFAULT 5 CHECK (max_attempts >= 1),
    next_attempt_at timestamptz NOT NULL,
    last_error      text,
    error_kind      text        CHECK (error_kind IN ('transient', 'terminal')),
    cancel_reason   text,
    created_by      text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    finished_at     timestamptz,
    FOREIGN KEY (revision_id, translation_id) REFERENCES revisions (id, translation_id)
);
-- One active schedule per translation.
CREATE UNIQUE INDEX publication_jobs_one_active ON publication_jobs (translation_id) WHERE status = 'scheduled';
CREATE INDEX publication_jobs_due ON publication_jobs (next_attempt_at) WHERE status = 'scheduled';

CREATE TABLE publication_job_attempts (
    job_id   uuid        NOT NULL REFERENCES publication_jobs (id),
    attempt  integer     NOT NULL,
    at       timestamptz NOT NULL DEFAULT now(),
    outcome  text        NOT NULL CHECK (outcome IN ('succeeded', 'transient_error', 'terminal_error')),
    error    text,
    PRIMARY KEY (job_id, attempt)
);

-- Public routes (also the slug history). scope: 'project', 'article' or 'log:<project content id>'.
-- Unique per locale and scope: two publications can never win the same path. Old slugs stay as
-- aliases of the same content (reserved for it), each resolving to that content's current route.
CREATE TABLE public_routes (
    locale     text        NOT NULL CHECK (locale IN ('es', 'en')),
    scope      text        NOT NULL CHECK (scope IN ('project', 'article') OR scope ~ '^log:[0-9a-f-]{36}$'),
    slug       text        NOT NULL,
    content_id uuid        NOT NULL REFERENCES contents (id),
    is_current boolean     NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (locale, scope, slug)
);
-- At most one current route per content and locale.
CREATE UNIQUE INDEX public_routes_one_current ON public_routes (content_id, locale) WHERE is_current;
-- Routes for pointers set before WEB-005 (a collision keeps the first and leaves the other unrouted).
INSERT INTO public_routes (locale, scope, slug, content_id, is_current)
SELECT t.locale, CASE WHEN c.kind = 'log' THEN 'log:' || c.project_id::text ELSE c.kind END, r.slug, c.id, true
FROM translations t JOIN contents c ON c.id = t.content_id JOIN revisions r ON r.id = t.published_revision_id
ORDER BY t.published_at
ON CONFLICT DO NOTHING;

-- Idempotent editorial mutations: same key + same body → the original answer.
CREATE TABLE editorial_requests (
    translation_id  uuid        NOT NULL REFERENCES translations (id),
    idempotency_key text        NOT NULL,
    action          text        NOT NULL,
    payload_hash    bytea       NOT NULL,
    response_status integer     NOT NULL,
    response_body   jsonb       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (translation_id, idempotency_key)
);

-- The single visibility rule shared by media delivery, the public reader and future listings,
-- search and sitemap: published, content not archived, and a log only with its project
-- published (and not archived) in the same locale.
CREATE VIEW visible_translations AS
SELECT t.id AS translation_id, t.content_id, t.locale, t.published_revision_id AS revision_id, c.kind, c.project_id, t.published_at
FROM translations t
JOIN contents c ON c.id = t.content_id
WHERE t.published_revision_id IS NOT NULL
  AND c.archived_at IS NULL
  AND (c.kind <> 'log' OR EXISTS (
        SELECT 1 FROM translations pt JOIN contents pc ON pc.id = pt.content_id
        WHERE pt.content_id = c.project_id AND pt.locale = t.locale
          AND pt.published_revision_id IS NOT NULL AND pc.archived_at IS NULL));

-- +goose Down
DROP VIEW visible_translations;
DROP TABLE editorial_requests;
DROP TABLE public_routes;
DROP TABLE publication_job_attempts;
DROP TABLE publication_jobs;
ALTER TABLE translations
    DROP CONSTRAINT translations_published_has_date,
    DROP COLUMN withdrawn_at,
    DROP COLUMN first_published_at,
    DROP COLUMN published_at,
    DROP COLUMN editorial_version;
