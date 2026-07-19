-- This script grants the Saturn runtime role only the database privileges used by current application queries.
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON TYPE audit_action, audit_result FROM PUBLIC;

ALTER DEFAULT PRIVILEGES FOR ROLE saturn_owner IN SCHEMA public
    REVOKE ALL ON TABLES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE saturn_owner IN SCHEMA public
    REVOKE ALL ON SEQUENCES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE saturn_owner IN SCHEMA public
    REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE saturn_owner IN SCHEMA public
    REVOKE USAGE ON TYPES FROM PUBLIC;

GRANT USAGE ON SCHEMA public TO saturn;
GRANT USAGE ON TYPE audit_action, audit_result TO saturn;

GRANT SELECT ON TABLE
    users,
    api_keys,
    object_refs,
    audit_logs,
    file_collections,
    files,
    notes,
    note_versions,
    accounts,
    transactions,
    event_aggregates,
    events,
    storage_objects,
    llm_sessions,
    llm_requests,
    llm_request_references
TO saturn;

GRANT INSERT (ref_code, password_hash) ON TABLE users TO saturn;
GRANT UPDATE (email, password_hash) ON TABLE users TO saturn;

GRANT INSERT (ref_code, name, key_prefix, key_hash, scopes, expires_at) ON TABLE api_keys TO saturn;
GRANT UPDATE (last_used_at, revoked_at) ON TABLE api_keys TO saturn;

GRANT INSERT (owner_id, ref_code, object_type, object_id, title, tags, status) ON TABLE object_refs TO saturn;
GRANT UPDATE (title, tags, status, updated_at) ON TABLE object_refs TO saturn;
GRANT DELETE ON TABLE object_refs TO saturn;

GRANT INSERT (actor_ref_code, action, target_ref_code, result, reason, source_ip, user_agent)
    ON TABLE audit_logs TO saturn;

GRANT INSERT (owner_id, name, description) ON TABLE file_collections TO saturn;
GRANT DELETE ON TABLE file_collections TO saturn;

GRANT INSERT (owner_id, collection_id, object_key, original_name, mime_type, size_bytes, sha256, blake3, metadata)
    ON TABLE files TO saturn;
GRANT DELETE ON TABLE files TO saturn;

GRANT INSERT (owner_id) ON TABLE notes TO saturn;
GRANT UPDATE (current_version_id, updated_at) ON TABLE notes TO saturn;
GRANT DELETE ON TABLE notes TO saturn;

GRANT INSERT (note_id, parent_version_id, version_number, title, content, content_type, operation)
    ON TABLE note_versions TO saturn;

GRANT INSERT (owner_id, name, type, currency, opening_balance_cents, balance_cents)
    ON TABLE accounts TO saturn;
GRANT UPDATE (balance_cents, updated_at) ON TABLE accounts TO saturn;
GRANT DELETE ON TABLE accounts TO saturn;

GRANT INSERT (owner_id, account_id, occurred_on, kind, amount_cents, title, note)
    ON TABLE transactions TO saturn;
GRANT UPDATE (status, updated_at) ON TABLE transactions TO saturn;

GRANT INSERT (owner_id, metadata) ON TABLE event_aggregates TO saturn;
GRANT DELETE ON TABLE event_aggregates TO saturn;

GRANT INSERT (owner_id, aggregate_id, starts_at, ends_at, metadata) ON TABLE events TO saturn;
GRANT UPDATE (status, updated_at) ON TABLE events TO saturn;

GRANT INSERT (object_key, path, size_bytes, sha256, blake3) ON TABLE storage_objects TO saturn;
GRANT UPDATE (path, size_bytes, sha256, blake3) ON TABLE storage_objects TO saturn;
GRANT DELETE ON TABLE storage_objects TO saturn;

GRANT INSERT (owner_id, title) ON TABLE llm_sessions TO saturn;
GRANT DELETE ON TABLE llm_sessions TO saturn;

GRANT INSERT (owner_id, session_id, actor_ref_code, prompt, model, max_tokens, context_json, request_json)
    ON TABLE llm_requests TO saturn;
GRANT UPDATE (response_status, content, error_code, error_message, response_json, updated_at, completed_at)
    ON TABLE llm_requests TO saturn;

GRANT INSERT (request_id, object_ref_id, ref_code, module, object_type, title, status, payload_json)
    ON TABLE llm_request_references TO saturn;

GRANT USAGE ON SEQUENCE
    object_ref_code_sequence,
    api_keys_id_seq,
    object_refs_id_seq,
    audit_logs_id_seq,
    file_collections_id_seq,
    files_id_seq,
    notes_id_seq,
    note_versions_id_seq,
    accounts_id_seq,
    transactions_id_seq,
    event_aggregates_id_seq,
    events_id_seq,
    storage_objects_id_seq,
    llm_sessions_id_seq,
    llm_requests_id_seq,
    llm_request_references_id_seq
TO saturn;
