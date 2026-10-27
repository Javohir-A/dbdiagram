CREATE TABLE sessions (
    guid        uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_guid   uuid        NOT NULL,
    family_guid uuid        NOT NULL,
    token_hash  text        NOT NULL,
    user_agent  text        NOT NULL DEFAULT '',
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE sessions IS 'Refresh tokens, stored as sha256 hashes. One login = one family; every refresh rotates the token inside its family.';
COMMENT ON COLUMN sessions.revoked_at IS 'Set when the token was used (rotated), logged out, or its family was revoked. Using a revoked token again revokes the whole family.';
