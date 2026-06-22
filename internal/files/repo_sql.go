// This file persists immutable Files collections and files through PostgreSQL.
package files

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lib/pq"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
)

type SQLRepository struct {
	database *sql.DB
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{database: database}
}

func (r *SQLRepository) ListCollections(ctx context.Context, scope auth.Scope, query CollectionQuery) (CollectionPage, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return CollectionPage{}, err
	}
	statement := collectionBaseSQL + `
ORDER BY c.created_at DESC, collection_ref.ref_code DESC
LIMIT $1 OFFSET $2`
	arguments := []any{query.Limit + 1, query.Offset}
	if !scope.All {
		statement = collectionBaseSQL + `
WHERE c.owner_id = $1
ORDER BY c.created_at DESC, collection_ref.ref_code DESC
LIMIT $2 OFFSET $3`
		arguments = []any{scope.OwnerID, query.Limit + 1, query.Offset}
	}
	rows, err := executor.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return CollectionPage{}, err
	}
	defer rows.Close()

	collections := make([]Collection, 0, query.Limit+1)
	for rows.Next() {
		collection, err := scanCollection(rows)
		if err != nil {
			return CollectionPage{}, err
		}
		collections = append(collections, collection)
	}
	if err := rows.Err(); err != nil {
		return CollectionPage{}, err
	}
	hasMore := len(collections) > query.Limit
	if hasMore {
		collections = collections[:query.Limit]
	}
	return CollectionPage{Collections: collections, Limit: query.Limit, Offset: query.Offset, HasMore: hasMore}, nil
}

func (r *SQLRepository) CreateCollection(ctx context.Context, ownerID int64, input CreateCollectionInput) (Collection, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Collection{}, err
	}
	var collection Collection
	err = executor.QueryRowContext(ctx, `
INSERT INTO file_collections (owner_id, name, description)
VALUES ($1, $2, $3)
RETURNING id, owner_id, name, description, created_at, updated_at`,
		ownerID, input.Name, input.Description).Scan(
		&collection.ID, &collection.OwnerID, &collection.Name, &collection.Description,
		&collection.CreatedAt, &collection.UpdatedAt,
	)
	return collection, err
}

func (r *SQLRepository) FindCollectionByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Collection, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Collection{}, err
	}
	statement := collectionBaseSQL + `WHERE collection_ref.ref_code = $1`
	arguments := []any{refCode}
	if !scope.All {
		statement += ` AND c.owner_id = $2`
		arguments = append(arguments, scope.OwnerID)
	}
	collection, err := scanCollection(executor.QueryRowContext(ctx, statement, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return Collection{}, ErrCollectionNotFound
	}
	if err != nil {
		return Collection{}, err
	}
	return collection, nil
}

func (r *SQLRepository) LockCollectionByRefCode(ctx context.Context, refCode string) (Collection, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Collection{}, err
	}
	collection, err := scanCollection(executor.QueryRowContext(ctx, collectionBaseSQL+`WHERE collection_ref.ref_code = $1 FOR UPDATE OF c`, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return Collection{}, ErrCollectionNotFound
	}
	return collection, err
}

func (r *SQLRepository) DeleteCollection(ctx context.Context, ownerID int64, collectionID int64) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `DELETE FROM file_collections WHERE owner_id = $1 AND id = $2`, ownerID, collectionID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrCollectionNotFound
	}
	return nil
}

func (r *SQLRepository) ListFileIDsForCollection(ctx context.Context, ownerID int64, collectionID int64) ([]int64, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := executor.QueryContext(ctx, `
SELECT id
FROM files
WHERE owner_id = $1 AND collection_id = $2
ORDER BY id`, ownerID, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *SQLRepository) ListFiles(ctx context.Context, scope auth.Scope, query FileQuery) (FilePage, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return FilePage{}, err
	}
	statement := fileBaseSQL + `
WHERE ($1::text = '' OR collection_ref.ref_code = $1)
  AND ($2::text = '' OR file_ref.tags @> ARRAY[$2]::text[])
ORDER BY f.created_at DESC, file_ref.ref_code DESC
LIMIT $3 OFFSET $4`
	arguments := []any{query.CollectionRefCode, query.Tag, query.Limit + 1, query.Offset}
	if !scope.All {
		statement = fileBaseSQL + `
WHERE f.owner_id = $1
  AND ($2::text = '' OR collection_ref.ref_code = $2)
  AND ($3::text = '' OR file_ref.tags @> ARRAY[$3]::text[])
ORDER BY f.created_at DESC, file_ref.ref_code DESC
LIMIT $4 OFFSET $5`
		arguments = []any{scope.OwnerID, query.CollectionRefCode, query.Tag, query.Limit + 1, query.Offset}
	}
	rows, err := executor.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return FilePage{}, err
	}
	defer rows.Close()

	files := make([]File, 0, query.Limit+1)
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return FilePage{}, err
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return FilePage{}, err
	}
	hasMore := len(files) > query.Limit
	if hasMore {
		files = files[:query.Limit]
	}
	return FilePage{Files: files, Limit: query.Limit, Offset: query.Offset, HasMore: hasMore}, nil
}

func (r *SQLRepository) CreateFile(ctx context.Context, ownerID int64, collectionID int64, input CreateFileInput, stored StoredFile) (File, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return File{}, err
	}
	metadata, err := json.Marshal(FileMetadataDetail{
		OriginalName: input.OriginalName, MimeType: input.MimeType, SizeBytes: stored.SizeBytes,
		SHA256: stored.SHA256, BLAKE3: stored.BLAKE3,
	})
	if err != nil {
		return File{}, err
	}
	var file File
	err = executor.QueryRowContext(ctx, `
INSERT INTO files (owner_id, collection_id, object_key, original_name, mime_type, size_bytes, sha256, blake3, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, owner_id, collection_id, object_key, original_name, mime_type, size_bytes, sha256, blake3, created_at, updated_at`,
		ownerID, collectionID, stored.ObjectKey, input.OriginalName, input.MimeType, stored.SizeBytes,
		stored.SHA256, stored.BLAKE3, metadata).Scan(
		&file.ID, &file.OwnerID, &file.CollectionID, &file.ObjectKey, &file.OriginalName, &file.MimeType,
		&file.SizeBytes, &file.SHA256, &file.BLAKE3, &file.CreatedAt, &file.UpdatedAt,
	)
	return file, err
}

func (r *SQLRepository) FindFileByRefCode(ctx context.Context, scope auth.Scope, refCode string) (File, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return File{}, err
	}
	statement := fileBaseSQL + `WHERE file_ref.ref_code = $1`
	arguments := []any{refCode}
	if !scope.All {
		statement += ` AND f.owner_id = $2`
		arguments = append(arguments, scope.OwnerID)
	}
	file, err := scanFile(executor.QueryRowContext(ctx, statement, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrFileNotFound
	}
	if err != nil {
		return File{}, err
	}
	return file, nil
}

func (r *SQLRepository) LockFileByRefCode(ctx context.Context, refCode string) (File, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return File{}, err
	}
	file, err := scanFile(executor.QueryRowContext(ctx, fileBaseSQL+`WHERE file_ref.ref_code = $1 FOR UPDATE OF f`, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrFileNotFound
	}
	return file, err
}

func (r *SQLRepository) LockFileByID(ctx context.Context, ownerID int64, fileID int64) (File, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return File{}, err
	}
	file, err := scanFile(executor.QueryRowContext(ctx, fileBaseSQL+`WHERE f.owner_id = $1 AND f.id = $2 FOR UPDATE OF f`, ownerID, fileID))
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrFileNotFound
	}
	return file, err
}

func (r *SQLRepository) DeleteFile(ctx context.Context, ownerID int64, fileID int64) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `DELETE FROM files WHERE owner_id = $1 AND id = $2`, ownerID, fileID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrFileNotFound
	}
	return nil
}

func (r *SQLRepository) executor(ctx context.Context) (platformdb.Executor, error) {
	if r == nil || r.database == nil {
		return nil, ErrRepositoryUnavailable
	}
	return platformdb.ExecutorFromContext(ctx, r.database), nil
}

const collectionBaseSQL = `
SELECT c.id, c.owner_id, collection_ref.id, collection_ref.ref_code, collection_ref.tags, c.name, c.description,
       collection_ref.status, c.created_at, c.updated_at
FROM file_collections AS c
JOIN object_refs AS collection_ref
  ON collection_ref.owner_id = c.owner_id
 AND collection_ref.object_type = 'file_collection'
 AND collection_ref.object_id = c.id
`

const fileBaseSQL = `
SELECT f.id, f.owner_id, f.collection_id, file_ref.id, file_ref.ref_code, file_ref.tags, collection_ref.ref_code,
       f.object_key, f.original_name, f.mime_type, f.size_bytes, f.sha256, f.blake3,
       file_ref.status, f.created_at, f.updated_at
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
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCollection(row rowScanner) (Collection, error) {
	var collection Collection
	err := row.Scan(
		&collection.ID, &collection.OwnerID, &collection.ObjectRefID, &collection.RefCode,
		pq.Array(&collection.Tags), &collection.Name, &collection.Description, &collection.Status,
		&collection.CreatedAt, &collection.UpdatedAt,
	)
	collection.Tags = nonNilTags(collection.Tags)
	return collection, err
}

func scanFile(row rowScanner) (File, error) {
	var file File
	err := row.Scan(
		&file.ID, &file.OwnerID, &file.CollectionID, &file.ObjectRefID, &file.RefCode, pq.Array(&file.Tags), &file.CollectionRefCode,
		&file.ObjectKey, &file.OriginalName, &file.MimeType, &file.SizeBytes, &file.SHA256, &file.BLAKE3,
		&file.Status, &file.CreatedAt, &file.UpdatedAt,
	)
	file.Tags = nonNilTags(file.Tags)
	return file, err
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
