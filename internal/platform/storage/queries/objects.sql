-- This file defines typed storage metadata query templates for sqlc generation.

-- name: SaveStorageObject :exec
INSERT INTO storage_objects (object_key, path, size_bytes, sha256, blake3)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (object_key) DO UPDATE
SET path = EXCLUDED.path,
    size_bytes = EXCLUDED.size_bytes,
    sha256 = EXCLUDED.sha256,
    blake3 = EXCLUDED.blake3;

-- name: FindStorageObjectByKey :one
SELECT id, object_key, path, size_bytes, sha256, blake3, created_at
FROM storage_objects
WHERE object_key = $1;

-- name: DeleteStorageObjectByKey :exec
DELETE FROM storage_objects
WHERE object_key = $1;
