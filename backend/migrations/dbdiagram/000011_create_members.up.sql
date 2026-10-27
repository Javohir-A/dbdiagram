CREATE TABLE folder_members (
    folder_guid uuid        NOT NULL,
    user_guid   uuid        NOT NULL,
    role        text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (folder_guid, user_guid),
    CONSTRAINT folder_members_role_check CHECK (role IN ('editor', 'viewer'))
);

CREATE TABLE diagram_members (
    diagram_guid uuid        NOT NULL,
    user_guid    uuid        NOT NULL,
    role         text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (diagram_guid, user_guid),
    CONSTRAINT diagram_members_role_check CHECK (role IN ('editor', 'viewer'))
);

COMMENT ON TABLE folder_members IS 'Direct shares of a folder; they apply to everything inside it. Hard deleted: a revoked share is not history. The owner is folders.owner_guid, never a row here.';
COMMENT ON TABLE diagram_members IS 'Direct shares of one diagram. Hard deleted.';
