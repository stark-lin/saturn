-- This file defines typed Notes query templates for sqlc generation.

-- name: ListNotesAll :many
SELECT n.id, n.owner_id, object_ref.ref_code, version_ref.ref_code AS current_version_ref,
       version.version_number, version.title, object_ref.status
FROM notes AS n
JOIN object_refs AS object_ref
  ON object_ref.owner_id = n.owner_id
 AND object_ref.object_type = 'nte-obj'
 AND object_ref.object_id = n.id
JOIN note_versions AS version ON version.id = n.current_version_id
JOIN object_refs AS version_ref
  ON version_ref.owner_id = n.owner_id
 AND version_ref.object_type = 'version-obj'
 AND version_ref.object_id = version.id
ORDER BY n.id DESC
LIMIT $1 OFFSET $2;
