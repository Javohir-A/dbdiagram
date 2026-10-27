ALTER TABLE access_tokens ADD CONSTRAINT access_tokens_user_guid_fkey FOREIGN KEY (user_guid) REFERENCES users (guid);

CREATE UNIQUE INDEX access_tokens_token_hash_key ON access_tokens (token_hash);
CREATE INDEX access_tokens_user_idx ON access_tokens (user_guid, created_at DESC) WHERE revoked_at IS NULL;
