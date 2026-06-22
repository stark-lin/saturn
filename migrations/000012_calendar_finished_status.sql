-- This migration adds Calendar event finish status transitions.
ALTER TABLE events
DROP CONSTRAINT IF EXISTS events_status_check;

ALTER TABLE events
ADD CONSTRAINT events_status_check
CHECK (status IN ('scheduled', 'finished', 'voided'));

CREATE OR REPLACE FUNCTION reject_calendar_event_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.owner_id IS DISTINCT FROM OLD.owner_id
       OR NEW.aggregate_id IS DISTINCT FROM OLD.aggregate_id
       OR NEW.starts_at IS DISTINCT FROM OLD.starts_at
       OR NEW.duration_minutes IS DISTINCT FROM OLD.duration_minutes
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
