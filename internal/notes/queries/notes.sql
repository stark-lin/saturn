-- This file defines typed Notes query templates for sqlc generation.

-- name: ListNotesForOwner :many
SELECT n.id, n.owner_id, object_ref.ref_code, n.title, object_ref.status
FROM notes AS n
JOIN object_refs AS object_ref
  ON object_ref.owner_id = n.owner_id
 AND object_ref.object_type = 'note'
 AND object_ref.object_id = n.id
WHERE n.owner_id = $1
ORDER BY n.id DESC
LIMIT $2 OFFSET $3;
