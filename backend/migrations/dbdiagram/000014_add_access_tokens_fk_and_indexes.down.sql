DROP INDEX IF EXISTS access_tokens_user_idx;
DROP INDEX IF EXISTS access_tokens_token_hash_key;
ALTER TABLE access_tokens DROP CONSTRAINT IF EXISTS access_tokens_user_guid_fkey;
