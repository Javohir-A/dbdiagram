-- One-off: copies the rows of the pre-refactor prototype tables (public.diagrams,
-- public.folders: bigint ids) into the dbdiagram schema (uuid guids).
-- Run once, after `make migrate-up`:  psql "$DB" -f scripts/import_legacy.sql
-- The legacy tables are renamed, not dropped, so a mistake costs nothing.
BEGIN;

CREATE TEMP TABLE folder_map ON COMMIT DROP AS
    SELECT id AS old_id, gen_random_uuid() AS guid, name, created_at FROM public.folders;

INSERT INTO dbdiagram.folders (guid, name, created_at)
    SELECT guid, name, created_at FROM folder_map;

INSERT INTO dbdiagram.diagrams (guid, name, dbml, positions, folder_guid, share_token, created_at, updated_at)
    SELECT gen_random_uuid(), d.name, d.dbml,
           CASE WHEN jsonb_typeof(d.positions) = 'object' THEN d.positions ELSE '{}'::jsonb END,
           m.guid, d.share_token, d.created_at, d.updated_at
    FROM public.diagrams d
    LEFT JOIN folder_map m ON m.old_id = d.folder_id;

ALTER TABLE public.diagrams RENAME TO legacy_diagrams;
ALTER TABLE public.folders RENAME TO legacy_folders;

COMMIT;
