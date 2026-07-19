-- This file defines append-only audit log query templates for sqlc generation.

-- name: InsertAuditLog :one
INSERT INTO audit_logs (
    actor_ref_code,
    action,
    target_ref_code,
    result,
    reason,
    source_ip,
    user_agent
)
VALUES (
    sqlc.arg(actor_ref_code),
    sqlc.arg(action)::text::audit_action,
    sqlc.arg(target_ref_code),
    sqlc.arg(result)::text::audit_result,
    sqlc.narg(reason),
    sqlc.arg(source_ip)::text::inet,
    sqlc.narg(user_agent)
)
RETURNING id, actor_ref_code, action::text AS action, target_ref_code,
          result::text AS result, reason, source_ip::text AS source_ip,
          user_agent, created_at;

-- name: ListAuditLogs :many
SELECT id, actor_ref_code, action::text AS action, target_ref_code,
       result::text AS result, reason, source_ip::text AS source_ip,
       user_agent, created_at
FROM audit_logs
WHERE (sqlc.arg(target_ref_code)::text = '' OR target_ref_code = sqlc.arg(target_ref_code))
  AND (sqlc.arg(actor_ref_code)::text = '' OR actor_ref_code = sqlc.arg(actor_ref_code))
  AND (sqlc.arg(action)::text = '' OR action::text = sqlc.arg(action))
  AND (sqlc.arg(result)::text = '' OR result::text = sqlc.arg(result))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
