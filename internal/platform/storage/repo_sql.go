// This file persists local storage metadata through PostgreSQL.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
)

type SQLRepository struct {
	database *sql.DB
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{database: database}
}

func (r *SQLRepository) Save(ctx context.Context, object Object) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `
INSERT INTO storage_objects (object_key, path, size_bytes, sha256, blake3)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (object_key) DO UPDATE
SET path = EXCLUDED.path,
    size_bytes = EXCLUDED.size_bytes,
    sha256 = EXCLUDED.sha256,
    blake3 = EXCLUDED.blake3`,
		object.Key, object.Path, object.Size, object.SHA256, object.BLAKE3)
	return err
}

func (r *SQLRepository) Find(ctx context.Context, key string) (Object, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Object{}, err
	}
	var object Object
	err = executor.QueryRowContext(ctx, `
SELECT object_key, path, size_bytes, sha256, blake3, created_at
FROM storage_objects
WHERE object_key = $1`, key).Scan(
		&object.Key, &object.Path, &object.Size, &object.SHA256, &object.BLAKE3, &object.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Object{}, osErrNotExist(key)
	}
	return object, err
}

func (r *SQLRepository) Delete(ctx context.Context, key string) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `DELETE FROM storage_objects WHERE object_key = $1`, key)
	return err
}

func (r *SQLRepository) executor(ctx context.Context) (platformdb.Executor, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("storage database is required")
	}
	return platformdb.ExecutorFromContext(ctx, r.database), nil
}

func osErrNotExist(name string) error {
	return fmt.Errorf("%s: %w", name, sql.ErrNoRows)
}
