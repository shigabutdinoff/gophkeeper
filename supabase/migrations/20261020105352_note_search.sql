create extension if not exists pg_trgm with schema extensions;

create index records_note_trgm on public.records using gin (note extensions.gin_trgm_ops);
