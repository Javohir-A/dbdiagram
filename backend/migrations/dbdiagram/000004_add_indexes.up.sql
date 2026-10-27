CREATE UNIQUE INDEX diagrams_share_token_key ON diagrams (share_token) WHERE share_token IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX diagrams_folder_guid_idx ON diagrams (folder_guid) WHERE deleted_at IS NULL;
CREATE INDEX diagrams_updated_at_idx ON diagrams (updated_at DESC, guid) WHERE deleted_at IS NULL;
CREATE INDEX folders_name_idx ON folders (lower(name), guid) WHERE deleted_at IS NULL;
