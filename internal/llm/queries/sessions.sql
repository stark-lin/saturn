-- This file defines typed LLM query templates for sqlc generation.

-- name: ListSessionsByOwner :many
SELECT s.id, s.owner_id, session_ref.id AS object_ref_id, session_ref.ref_code,
       s.title, s.status, s.created_at, s.updated_at
FROM llm_sessions AS s
JOIN object_refs AS session_ref
  ON session_ref.owner_id = s.owner_id
 AND session_ref.object_type = 'llm_session'
 AND session_ref.object_id = s.id
WHERE s.owner_id = $1
ORDER BY s.created_at DESC, s.id DESC
LIMIT $2 OFFSET $3;

-- name: ListRequestsForSession :many
SELECT request.id, request.owner_id, request.session_id, request_ref.id AS object_ref_id,
       request_ref.ref_code, request.prompt, request.model, request.max_tokens,
       request.context_json, request.request_json, request.response_status,
       request.content, request.error_code, request.error_message, request.response_json,
       request.created_at, request.updated_at, request.completed_at
FROM llm_requests AS request
JOIN object_refs AS request_ref
  ON request_ref.owner_id = request.owner_id
 AND request_ref.object_type = 'llm_request'
 AND request_ref.object_id = request.id
WHERE request.owner_id = $1
  AND request.session_id = $2
ORDER BY request.created_at ASC, request.id ASC
LIMIT $3 OFFSET $4;
