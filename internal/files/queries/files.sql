-- This file defines typed Files query templates for sqlc generation.

-- name: ListFileCollectionsAll :many
SELECT c.id, c.owner_id, collection_ref.ref_code, c.name, c.description, collection_ref.status
FROM file_collections AS c
JOIN object_refs AS collection_ref
  ON collection_ref.owner_id = c.owner_id
 AND collection_ref.object_type = 'file_collection'
 AND collection_ref.object_id = c.id
ORDER BY c.created_at DESC, collection_ref.ref_code DESC
LIMIT $1 OFFSET $2;

-- name: ListFilesAll :many
SELECT f.id, f.owner_id, f.collection_id, file_ref.ref_code, collection_ref.ref_code AS collection_ref_code,
       f.original_name, f.mime_type, f.size_bytes, f.sha256, f.blake3, file_ref.status
FROM files AS f
JOIN object_refs AS file_ref
  ON file_ref.owner_id = f.owner_id
 AND file_ref.object_type = 'file'
 AND file_ref.object_id = f.id
JOIN file_collections AS c
  ON c.owner_id = f.owner_id
 AND c.id = f.collection_id
JOIN object_refs AS collection_ref
  ON collection_ref.owner_id = c.owner_id
 AND collection_ref.object_type = 'file_collection'
 AND collection_ref.object_id = c.id
ORDER BY f.created_at DESC, file_ref.ref_code DESC
LIMIT $1 OFFSET $2;
