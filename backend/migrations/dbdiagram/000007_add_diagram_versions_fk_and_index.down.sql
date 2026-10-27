DROP INDEX IF EXISTS diagram_versions_diagram_idx;
ALTER TABLE diagram_versions DROP CONSTRAINT IF EXISTS diagram_versions_diagram_guid_fkey;
