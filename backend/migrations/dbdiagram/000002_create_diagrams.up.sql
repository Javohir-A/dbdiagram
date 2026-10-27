CREATE TABLE diagrams (
    guid        uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        NOT NULL,
    dbml        text        NOT NULL DEFAULT '',
    positions   jsonb       NOT NULL DEFAULT '{}'::jsonb,
    folder_guid uuid,
    share_token text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,
    CONSTRAINT diagrams_positions_is_object CHECK (jsonb_typeof(positions) = 'object')
);

COMMENT ON TABLE diagrams IS 'A DBML document plus where its tables sit on the canvas. Soft deleted.';
COMMENT ON COLUMN diagrams.positions IS 'Object of table id -> {x, y}.';
COMMENT ON COLUMN diagrams.share_token IS 'Random token of the public read-only link; NULL = not shared.';
