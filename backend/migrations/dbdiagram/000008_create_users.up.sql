CREATE TABLE users (
    guid          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL,
    name          text        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);

COMMENT ON TABLE users IS 'Accounts. email is stored trimmed and lower-cased.';
COMMENT ON COLUMN users.password_hash IS 'bcrypt. Never leaves the storage layer.';
