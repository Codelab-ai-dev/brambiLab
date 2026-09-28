-- Accents also ignored in tokens with digits or hyphens ("telemetría-2", "ESP32-C3"), then reindex.
-- (web-v1.md §9.1, WEB-006.)

-- +goose Up
ALTER TEXT SEARCH CONFIGURATION bl_es ALTER MAPPING FOR numword, numhword, hword_numpart WITH unaccent, simple;
ALTER TEXT SEARCH CONFIGURATION bl_en ALTER MAPPING FOR numword, numhword, hword_numpart WITH unaccent, simple;
UPDATE search_documents SET search = search_vector(revision_id, locale), updated_at = now();

-- +goose Down
ALTER TEXT SEARCH CONFIGURATION bl_es ALTER MAPPING FOR numword, numhword, hword_numpart WITH simple;
ALTER TEXT SEARCH CONFIGURATION bl_en ALTER MAPPING FOR numword, numhword, hword_numpart WITH simple;
UPDATE search_documents SET search = search_vector(revision_id, locale), updated_at = now();
