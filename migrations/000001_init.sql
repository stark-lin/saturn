-- This migration clears non-table schema objects before rebuilding the ER diagram schema.
DROP SEQUENCE IF EXISTS object_ref_code_sequence;
DROP FUNCTION IF EXISTS reject_accounting_transaction_mutation();
DROP FUNCTION IF EXISTS reject_calendar_event_aggregate_mutation();
DROP FUNCTION IF EXISTS reject_calendar_event_mutation();
DROP FUNCTION IF EXISTS reject_file_mutation();
DROP FUNCTION IF EXISTS reject_file_collection_mutation();
DROP FUNCTION IF EXISTS reject_llm_request_mutation();
DROP FUNCTION IF EXISTS reject_llm_response_delete();
DROP FUNCTION IF EXISTS reject_llm_response_update();
DROP FUNCTION IF EXISTS reject_audit_log_mutation();
DROP TYPE IF EXISTS audit_result;
DROP TYPE IF EXISTS audit_action;
DROP TYPE IF EXISTS audit_actor_type;
