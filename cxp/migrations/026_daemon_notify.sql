-- NOTIFY trigger on schedules table so the daemon picks up changes
-- within 5 seconds without a restart.

CREATE OR REPLACE FUNCTION notify_schedules_changed()
    RETURNS trigger
    LANGUAGE plpgsql
AS $$
BEGIN
    NOTIFY schedules_changed, '';
    RETURN NULL;
END;
$$;

CREATE TRIGGER trg_schedules_changed
    AFTER INSERT OR UPDATE OR DELETE ON schedules
    FOR EACH STATEMENT
    EXECUTE FUNCTION notify_schedules_changed();
