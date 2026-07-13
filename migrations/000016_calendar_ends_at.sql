-- This migration replaces Calendar event durations with explicit end timestamps.
ALTER TABLE events
ADD COLUMN ends_at TIMESTAMPTZ;

ALTER TABLE events
DISABLE TRIGGER events_immutable_except_void;

UPDATE events
SET ends_at = starts_at + make_interval(mins => duration_minutes);

ALTER TABLE events
ENABLE TRIGGER events_immutable_except_void;

ALTER TABLE events
ALTER COLUMN ends_at SET NOT NULL;

ALTER TABLE events
ADD CONSTRAINT events_ends_after_start
CHECK (ends_at > starts_at);

CREATE OR REPLACE FUNCTION reject_calendar_event_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.owner_id IS DISTINCT FROM OLD.owner_id
       OR NEW.aggregate_id IS DISTINCT FROM OLD.aggregate_id
       OR NEW.starts_at IS DISTINCT FROM OLD.starts_at
       OR NEW.ends_at IS DISTINCT FROM OLD.ends_at
       OR NEW.metadata IS DISTINCT FROM OLD.metadata
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'calendar events are immutable except status';
    END IF;

    IF NOT (
        (OLD.status = 'scheduled' AND NEW.status IN ('finished', 'voided'))
        OR (OLD.status = 'finished' AND NEW.status = 'voided')
    ) THEN
        RAISE EXCEPTION 'calendar event status may only transition from scheduled to finished, scheduled to voided, or finished to voided';
    END IF;

    RETURN NEW;
END;
$$;

ALTER TABLE events
DROP COLUMN duration_minutes;
