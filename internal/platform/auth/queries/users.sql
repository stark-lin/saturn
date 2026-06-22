-- This file defines typed authentication user queries for sqlc generation.

-- name: FindUserByUsername :one
SELECT id, username, COALESCE(email, '') AS email, role, password_hash
FROM users
WHERE username = $1;

-- name: FindUserByID :one
SELECT id, username, COALESCE(email, '') AS email, role, password_hash
FROM users
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (username, email, role, password_hash)
VALUES (sqlc.arg(username), sqlc.narg(email), sqlc.arg(role), sqlc.arg(password_hash))
RETURNING id, username, COALESCE(email, '') AS email, role, password_hash;

-- name: CreateUserIfMissing :exec
INSERT INTO users (username, role, password_hash)
VALUES ($1, $2, $3)
ON CONFLICT (username) DO NOTHING;

-- name: UpdateUserProfile :one
UPDATE users
SET username = sqlc.arg(username),
    email = sqlc.narg(email)
WHERE id = sqlc.arg(id)
RETURNING id, username, COALESCE(email, '') AS email, role, password_hash;

-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = sqlc.arg(password_hash)
WHERE id = sqlc.arg(id)
RETURNING id, username, COALESCE(email, '') AS email, role, password_hash;
