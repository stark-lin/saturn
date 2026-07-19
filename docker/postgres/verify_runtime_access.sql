-- This script exercises every current Saturn runtime table operation under the least-privilege role.
BEGIN;

SELECT nextval('object_ref_code_sequence');

INSERT INTO users (ref_code, password_hash)
VALUES ('USR-00000001', 'bootstrap-test-hash');
UPDATE users SET email = 'test@example.invalid', password_hash = 'updated-test-hash' WHERE id = 1;

INSERT INTO object_refs (owner_id, ref_code, object_type, object_id, title, tags, status)
VALUES (0, 'USR-00000001', 'user', 1, 'Administrator', '{}', 'active');

INSERT INTO api_keys (ref_code, name, key_prefix, key_hash, scopes)
VALUES ('KEY-00000001', 'runtime-access', 'sat_sk_12345678', repeat('a', 64), ARRAY['data:read']);
UPDATE api_keys SET last_used_at = clock_timestamp() WHERE ref_code = 'KEY-00000001';

INSERT INTO object_refs (owner_id, ref_code, object_type, object_id, title, tags, status)
SELECT 0, 'KEY-00000001', 'api_key', id, name, '{}', 'active'
FROM api_keys WHERE ref_code = 'KEY-00000001';

INSERT INTO audit_logs (actor_ref_code, action, target_ref_code, result, reason, source_ip)
VALUES ('USR-00000001', 'CREATE', 'KEY-00000001', 'SUCCESS', 'runtime_access_test', '127.0.0.1');

INSERT INTO file_collections (owner_id, name, description)
VALUES (1, 'Runtime collection', 'Permission verification')
RETURNING id AS file_collection_id \gset
INSERT INTO files (owner_id, collection_id, object_key, original_name, mime_type, size_bytes, sha256, blake3, metadata)
VALUES (
    1, :file_collection_id, 'runtime/file.txt', 'file.txt', 'text/plain', 4,
    repeat('b', 64), repeat('c', 64), '{}'::jsonb
)
RETURNING id AS file_id \gset

INSERT INTO object_refs (owner_id, ref_code, object_type, object_id, title, tags, status)
VALUES
    (1, 'FIL-00000001', 'file_collection', :file_collection_id, 'Runtime collection', '{}', 'active'),
    (1, 'FIL-00000002', 'file', :file_id, 'file.txt', '{}', 'active');

INSERT INTO notes (owner_id) VALUES (1)
RETURNING id AS note_id \gset
INSERT INTO note_versions (note_id, version_number, title, content, content_type, operation)
VALUES (:note_id, 1, 'Runtime note', 'Permission verification', 'text/markdown', 'create')
RETURNING id AS note_version_id \gset
UPDATE notes SET current_version_id = :note_version_id, updated_at = NOW() WHERE id = :note_id;

INSERT INTO object_refs (owner_id, ref_code, object_type, object_id, title, tags, status)
VALUES
    (1, 'NTE-00000001', 'nte-obj', :note_id, 'Runtime note', '{}', 'active'),
    (1, 'NTE-00000002', 'version-obj', :note_version_id, 'Runtime note', '{}', 'immutable');

INSERT INTO accounts (owner_id, name, type, currency, opening_balance_cents, balance_cents)
VALUES (1, 'Runtime account', 'bank', 'AUD', 100, 100)
RETURNING id AS account_id \gset
INSERT INTO transactions (owner_id, account_id, occurred_on, kind, amount_cents, title, note)
VALUES (1, :account_id, CURRENT_DATE, 'income', 50, 'Runtime transaction', '')
RETURNING id AS transaction_id \gset
UPDATE accounts SET balance_cents = 150, updated_at = NOW() WHERE id = :account_id;
UPDATE transactions SET status = 'voided', updated_at = NOW() WHERE id = :transaction_id;

INSERT INTO object_refs (owner_id, ref_code, object_type, object_id, title, tags, status)
VALUES
    (1, 'ACC-00000001', 'account', :account_id, 'Runtime account', '{}', 'active'),
    (1, 'ACC-00000002', 'transaction', :transaction_id, 'Runtime transaction', '{}', 'voided');

INSERT INTO event_aggregates (owner_id, metadata)
VALUES (1, '{"title":"Runtime aggregate"}'::jsonb)
RETURNING id AS event_aggregate_id \gset
INSERT INTO events (owner_id, aggregate_id, starts_at, ends_at, metadata)
VALUES (1, :event_aggregate_id, NOW(), NOW() + INTERVAL '1 hour', '{"title":"Runtime event"}'::jsonb)
RETURNING id AS event_id \gset
UPDATE events SET status = 'finished', updated_at = NOW() WHERE id = :event_id;

INSERT INTO object_refs (owner_id, ref_code, object_type, object_id, title, tags, status)
VALUES
    (1, 'CAL-00000001', 'event_aggregate', :event_aggregate_id, 'Runtime aggregate', '{}', 'active'),
    (1, 'CAL-00000002', 'event', :event_id, 'Runtime event', '{}', 'finished');

INSERT INTO storage_objects (object_key, path, size_bytes, sha256, blake3)
VALUES ('runtime/file.txt', '/tmp/runtime-file.txt', 4, repeat('b', 64), repeat('c', 64))
ON CONFLICT (object_key) DO UPDATE
SET path = EXCLUDED.path,
    size_bytes = EXCLUDED.size_bytes,
    sha256 = EXCLUDED.sha256,
    blake3 = EXCLUDED.blake3;

INSERT INTO llm_sessions (owner_id, title)
VALUES (1, 'Runtime session')
RETURNING id AS llm_session_id \gset
INSERT INTO llm_requests (owner_id, session_id, actor_ref_code, prompt, model, max_tokens, context_json, request_json)
VALUES (1, :llm_session_id, 'USR-00000001', 'Permission verification', 'test-model', 32, '{}'::jsonb, '{}'::jsonb)
RETURNING id AS llm_request_id \gset

INSERT INTO object_refs (owner_id, ref_code, object_type, object_id, title, tags, status)
VALUES
    (1, 'LLM-00000001', 'llm_session', :llm_session_id, 'Runtime session', '{}', 'active'),
    (1, 'LLM-00000002', 'llm_request', :llm_request_id, 'Permission verification', '{}', 'queued');

INSERT INTO llm_request_references (request_id, object_ref_id, ref_code, module, object_type, title, status, payload_json)
SELECT :llm_request_id, id, ref_code, 'notes', object_type, title, status, '{}'::jsonb
FROM object_refs WHERE ref_code = 'NTE-00000001';

UPDATE llm_requests SET response_status = 'running', updated_at = NOW() WHERE id = :llm_request_id;
UPDATE llm_requests
SET response_status = 'success', content = 'ok', response_json = '{"content":"ok"}'::jsonb,
    updated_at = NOW(), completed_at = NOW()
WHERE id = :llm_request_id;

UPDATE object_refs
SET title = 'Updated runtime session', tags = ARRAY['test'], status = 'active', updated_at = NOW()
WHERE ref_code = 'LLM-00000001';

DELETE FROM storage_objects WHERE object_key = 'runtime/file.txt';
DELETE FROM object_refs WHERE ref_code LIKE 'LLM-%';
SELECT set_config('saturn.deleting_llm_session', 'on', true);
DELETE FROM llm_sessions WHERE id = :llm_session_id;
DELETE FROM object_refs WHERE ref_code LIKE 'CAL-%';
DELETE FROM event_aggregates WHERE id = :event_aggregate_id;
DELETE FROM object_refs WHERE ref_code LIKE 'ACC-%';
DELETE FROM accounts WHERE id = :account_id;
DELETE FROM object_refs WHERE ref_code LIKE 'NTE-%';
DELETE FROM notes WHERE id = :note_id;
DELETE FROM object_refs WHERE ref_code LIKE 'FIL-%';
DELETE FROM files WHERE id = :file_id;
DELETE FROM file_collections WHERE id = :file_collection_id;

ROLLBACK;
