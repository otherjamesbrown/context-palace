-- +goose Up
-- Trigger to broadcast a schedules_changed notification whenever a schedule row is
-- created, updated, or deleted.  The daemon listens on this channel and reloads its
-- cron table within ~5 seconds of any change — no restart required.

CREATE OR REPLACE FUNCTION notify_schedules_changed()
RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    PERFORM pg_notify('schedules_changed', OLD.id::text);
  ELSE
    PERFORM pg_notify('schedules_changed', NEW.id::text);
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER schedules_changed_trigger
AFTER INSERT OR UPDATE OR DELETE ON schedules
FOR EACH ROW EXECUTE FUNCTION notify_schedules_changed();

-- +goose Down
DROP TRIGGER IF EXISTS schedules_changed_trigger ON schedules;
DROP FUNCTION IF EXISTS notify_schedules_changed();
