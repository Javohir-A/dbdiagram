-- Rows created before accounts existed have no owner (NULL): nobody can reach
-- them until `app claim -email=...` assigns them.
ALTER TABLE folders  ADD COLUMN owner_guid uuid;
ALTER TABLE folders  ADD COLUMN parent_guid uuid;
ALTER TABLE diagrams ADD COLUMN owner_guid uuid;
ALTER TABLE diagrams ADD COLUMN updated_by_guid uuid;
ALTER TABLE diagrams ADD COLUMN revision bigint NOT NULL DEFAULT 1;

COMMENT ON COLUMN folders.parent_guid IS 'NULL = top level. Nesting depth is bounded by the service (config.MaxFolderDepth).';
COMMENT ON COLUMN diagrams.revision IS 'Bumps whenever dbml changes; the optimistic-concurrency token.';
