ALTER TABLE sessions        ADD CONSTRAINT sessions_user_guid_fkey FOREIGN KEY (user_guid) REFERENCES users (guid);
ALTER TABLE folders         ADD CONSTRAINT folders_owner_guid_fkey FOREIGN KEY (owner_guid) REFERENCES users (guid);
ALTER TABLE folders         ADD CONSTRAINT folders_parent_guid_fkey FOREIGN KEY (parent_guid) REFERENCES folders (guid);
ALTER TABLE diagrams        ADD CONSTRAINT diagrams_owner_guid_fkey FOREIGN KEY (owner_guid) REFERENCES users (guid);
ALTER TABLE diagrams        ADD CONSTRAINT diagrams_updated_by_guid_fkey FOREIGN KEY (updated_by_guid) REFERENCES users (guid);
ALTER TABLE folder_members  ADD CONSTRAINT folder_members_folder_guid_fkey FOREIGN KEY (folder_guid) REFERENCES folders (guid);
ALTER TABLE folder_members  ADD CONSTRAINT folder_members_user_guid_fkey FOREIGN KEY (user_guid) REFERENCES users (guid);
ALTER TABLE diagram_members ADD CONSTRAINT diagram_members_diagram_guid_fkey FOREIGN KEY (diagram_guid) REFERENCES diagrams (guid);
ALTER TABLE diagram_members ADD CONSTRAINT diagram_members_user_guid_fkey FOREIGN KEY (user_guid) REFERENCES users (guid);

CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX sessions_token_hash_key ON sessions (token_hash);
CREATE INDEX sessions_user_idx ON sessions (user_guid);
CREATE INDEX sessions_family_idx ON sessions (family_guid);
CREATE INDEX folders_owner_idx ON folders (owner_guid) WHERE deleted_at IS NULL;
CREATE INDEX folders_parent_idx ON folders (parent_guid) WHERE deleted_at IS NULL;
CREATE INDEX diagrams_owner_idx ON diagrams (owner_guid) WHERE deleted_at IS NULL;
CREATE INDEX folder_members_user_idx ON folder_members (user_guid);
CREATE INDEX diagram_members_user_idx ON diagram_members (user_guid);

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
