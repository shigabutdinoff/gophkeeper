-- +goose Up
SELECT pg_advisory_xact_lock(hashtext('pg_trgm'));
CREATE EXTENSION IF NOT EXISTS pg_trgm SCHEMA public;
CREATE INDEX records_note_trgm ON records USING gin (note public.gin_trgm_ops);

-- +goose Down
DROP INDEX records_note_trgm;
