ALTER TABLE diagram_versions
    ADD CONSTRAINT diagram_versions_diagram_guid_fkey FOREIGN KEY (diagram_guid) REFERENCES diagrams (guid);

CREATE INDEX diagram_versions_diagram_idx ON diagram_versions (diagram_guid, created_at DESC, guid DESC) WHERE deleted_at IS NULL;
