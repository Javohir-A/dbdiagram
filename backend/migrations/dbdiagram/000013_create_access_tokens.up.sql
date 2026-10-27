CREATE TABLE access_tokens (
    guid         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_guid    uuid        NOT NULL,
    name         text        NOT NULL,
    prefix       text        NOT NULL,
    token_hash   text        NOT NULL,
    scope        text        NOT NULL,
    expires_at   timestamptz NOT NULL,
    last_used_at timestamptz,
    revoked_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT access_tokens_scope_check CHECK (scope IN ('read', 'write'))
);

COMMENT ON TABLE access_tokens IS 'Personal access tokens for scripts and AI agents. Only the sha256 of the secret is stored; the secret is shown once at creation.';
COMMENT ON COLUMN access_tokens.prefix IS 'First characters of the secret, so a person can recognise which token is which.';
