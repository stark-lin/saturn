-- This migration moves LLM request queue claiming into PostgreSQL.
ALTER TABLE llm_requests
ADD COLUMN IF NOT EXISTS actor_user_id BIGINT REFERENCES users(id);

ALTER TABLE llm_requests DISABLE TRIGGER llm_requests_single_response_write;

UPDATE llm_requests
SET actor_user_id = owner_id
WHERE actor_user_id IS NULL;

ALTER TABLE llm_requests
ALTER COLUMN actor_user_id SET NOT NULL;

ALTER TABLE llm_requests ENABLE TRIGGER llm_requests_single_response_write;

CREATE OR REPLACE FUNCTION reject_llm_request_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF COALESCE(current_setting('saturn.deleting_llm_session', true), '') <> 'on' THEN
            RAISE EXCEPTION 'llm requests can only be deleted by deleting their session';
        END IF;
        RETURN OLD;
    END IF;

    IF OLD.response_status IN ('success', 'error') THEN
        RAISE EXCEPTION 'llm request response is terminal after first write';
    END IF;

    IF NEW.owner_id IS DISTINCT FROM OLD.owner_id
       OR NEW.session_id IS DISTINCT FROM OLD.session_id
       OR NEW.actor_user_id IS DISTINCT FROM OLD.actor_user_id
       OR NEW.prompt IS DISTINCT FROM OLD.prompt
       OR NEW.model IS DISTINCT FROM OLD.model
       OR NEW.max_tokens IS DISTINCT FROM OLD.max_tokens
       OR NEW.context_json IS DISTINCT FROM OLD.context_json
       OR NEW.request_json IS DISTINCT FROM OLD.request_json
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'llm request identity is immutable';
    END IF;

    IF NEW.response_status NOT IN ('running', 'success', 'error') THEN
        RAISE EXCEPTION 'llm request response status may only transition to running, success or error';
    END IF;

    IF OLD.response_status = 'running' AND NEW.response_status = 'running' THEN
        RAISE EXCEPTION 'llm request response is already running';
    END IF;

    IF OLD.response_status = 'queued' AND NEW.response_status = 'running' THEN
        IF NEW.content IS DISTINCT FROM OLD.content
           OR NEW.error_code IS DISTINCT FROM OLD.error_code
           OR NEW.error_message IS DISTINCT FROM OLD.error_message
           OR NEW.response_json IS DISTINCT FROM OLD.response_json
           OR NEW.completed_at IS DISTINCT FROM OLD.completed_at THEN
            RAISE EXCEPTION 'llm request running transition cannot write response result';
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.response_status <> 'running' THEN
        RAISE EXCEPTION 'llm request response must transition through running';
    END IF;

    IF NEW.response_status NOT IN ('success', 'error') THEN
        RAISE EXCEPTION 'llm request response status may only finish as success or error';
    END IF;

    IF NEW.response_status = 'error' AND BTRIM(NEW.error_message) = '' THEN
        RAISE EXCEPTION 'llm request error response requires an error message';
    END IF;

    IF NEW.completed_at IS NULL THEN
        RAISE EXCEPTION 'llm request response completion time is required';
    END IF;

    RETURN NEW;
END;
$$;

CREATE INDEX IF NOT EXISTS idx_llm_requests_queue_claim
ON llm_requests (created_at ASC, id ASC)
WHERE response_status = 'queued';

DROP TABLE IF EXISTS queue_job_events;
DROP TABLE IF EXISTS queue_jobs;
