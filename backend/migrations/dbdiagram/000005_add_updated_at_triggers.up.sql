CREATE TRIGGER folders_set_updated_at BEFORE UPDATE ON folders
    FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
CREATE TRIGGER diagrams_set_updated_at BEFORE UPDATE ON diagrams
    FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
