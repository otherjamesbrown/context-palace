-- Add LISTEN/NOTIFY trigger on the schedules table.
-- The CP daemon LISTENs on 'schedule_changed' so that schedule creates,
-- enables, and disables are picked up within seconds — no restart required.

CREATE OR REPLACE FUNCTION notify_schedule_change()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('schedule_changed', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER schedules_change_trigger
AFTER INSERT OR UPDATE ON schedules
FOR EACH ROW EXECUTE FUNCTION notify_schedule_change();
