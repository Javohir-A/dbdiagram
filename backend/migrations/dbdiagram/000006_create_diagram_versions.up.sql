CREATE TABLE diagram_versions (
    guid         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    diagram_guid uuid        NOT NULL,
    label        text        NOT NULL DEFAULT '',
    dbml         text        NOT NULL DEFAULT '',
    positions    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,
    CONSTRAINT diagram_versions_positions_is_object CHECK (jsonb_typeof(positions) = 'object')
);

COMMENT ON TABLE diagram_versions IS 'Immutable snapshots of a diagram (dbml + layout). Soft deleted; hidden with their diagram.';
COMMENT ON COLUMN diagram_versions.label IS 'Name given by the user; empty for automatic snapshots.';
