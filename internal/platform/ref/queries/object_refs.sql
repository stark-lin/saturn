-- This file defines typed object reference query templates for sqlc generation.

-- name: NextObjectRefSequence :one
SELECT nextval('object_ref_code_sequence')::bigint;

-- name: RegisterObjectRef :one
INSERT INTO object_refs (
    owner_id,
    ref_code,
    object_type,
    object_id,
    title,
    tags,
    status
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, owner_id, ref_code, object_type, object_id, title, tags, status, created_at, updated_at;

-- name: FindObjectRefByCode :one
SELECT id, owner_id, ref_code, object_type, object_id, title, tags, status, created_at, updated_at
FROM object_refs
WHERE ref_code = $1;

-- name: ListRecentObjectRefsByOwner :many
SELECT id, owner_id, ref_code, object_type, object_id, title, tags, status, created_at, updated_at
FROM object_refs
WHERE owner_id = $1
ORDER BY updated_at DESC, ref_code DESC
LIMIT $2;

-- name: UpdateObjectRefProjection :one
UPDATE object_refs
SET title = $4,
    tags = $5,
    status = $6,
    updated_at = NOW()
WHERE owner_id = $1
  AND object_type = $2
  AND object_id = $3
RETURNING id, owner_id, ref_code, object_type, object_id, title, tags, status, created_at, updated_at;

-- name: DeleteObjectRef :exec
DELETE FROM object_refs
WHERE owner_id = $1
  AND object_type = $2
  AND object_id = $3;
