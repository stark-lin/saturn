-- This file defines typed administrator and API key queries for sqlc generation.

-- name: FindAdministratorByUsername :one
SELECT id, ref_code, username, COALESCE(email, '') AS email, password_hash
FROM users
WHERE username = $1
  AND ref_code = 'USR-00000001';

-- name: FindAdministratorByRefCode :one
SELECT id, ref_code, username, COALESCE(email, '') AS email, password_hash
FROM users
WHERE ref_code = $1;

-- name: FindAdministrator :one
SELECT id, ref_code, username, COALESCE(email, '') AS email, password_hash
FROM users
WHERE ref_code = 'USR-00000001';

-- name: CreateAdministratorIfMissing :exec
INSERT INTO users (ref_code, username, password_hash)
VALUES ('USR-00000001', $1, $2)
ON CONFLICT (ref_code) DO NOTHING;

-- name: UpdateAdministratorProfile :one
UPDATE users
SET username = sqlc.arg(username),
    email = sqlc.narg(email)
WHERE ref_code = 'USR-00000001'
RETURNING id, ref_code, username, COALESCE(email, '') AS email, password_hash;

-- name: UpdateAdministratorPassword :one
UPDATE users
SET password_hash = sqlc.arg(password_hash)
WHERE ref_code = 'USR-00000001'
RETURNING id, ref_code, username, COALESCE(email, '') AS email, password_hash;

-- name: CreateAPIKey :one
INSERT INTO api_keys (ref_code, name, key_prefix, key_hash, scopes, expires_at)
VALUES (sqlc.arg(ref_code), sqlc.arg(name), sqlc.arg(key_prefix), sqlc.arg(key_hash), sqlc.arg(scopes), sqlc.narg(expires_at))
RETURNING ref_code, name, key_prefix, key_hash, scopes, created_at, last_used_at, expires_at, revoked_at;

-- name: UseAPIKey :one
UPDATE api_keys
SET last_used_at = GREATEST(COALESCE(last_used_at, '-infinity'::timestamptz), clock_timestamp())
WHERE key_hash = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > NOW())
RETURNING ref_code, name, key_prefix, key_hash, scopes, created_at, last_used_at, expires_at, revoked_at;

-- name: FindAPIKeyByRefCode :one
SELECT ref_code, name, key_prefix, key_hash, scopes, created_at, last_used_at, expires_at, revoked_at
FROM api_keys
WHERE ref_code = $1;

-- name: ListAPIKeys :many
SELECT ref_code, name, key_prefix, key_hash, scopes, created_at, last_used_at, expires_at, revoked_at
FROM api_keys
ORDER BY created_at DESC, ref_code DESC;

-- name: RevokeAPIKey :one
UPDATE api_keys
SET revoked_at = COALESCE(revoked_at, NOW())
WHERE ref_code = $1
RETURNING ref_code, name, key_prefix, key_hash, scopes, created_at, last_used_at, expires_at, revoked_at;
