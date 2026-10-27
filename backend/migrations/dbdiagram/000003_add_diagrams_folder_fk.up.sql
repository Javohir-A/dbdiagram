ALTER TABLE diagrams
    ADD CONSTRAINT diagrams_folder_guid_fkey FOREIGN KEY (folder_guid) REFERENCES folders (guid);
