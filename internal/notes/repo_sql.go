// This file persists logical Notes and immutable versions through PostgreSQL.
package notes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
)

type SQLRepository struct {
	database *sql.DB
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{database: database}
}

func (r *SQLRepository) ListNotes(ctx context.Context, ownerID int64, query Query) (Page, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Page{}, err
	}
	rows, err := executor.QueryContext(ctx, noteSelectSQL+`
WHERE n.owner_id = $1
  AND n.deleted_at IS NULL
  AND ($2::text = '' OR version.title ILIKE '%' || $2 || '%' OR version.content ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR note_ref.tags @> ARRAY[$3]::text[])
ORDER BY n.updated_at DESC, note_ref.ref_code DESC
LIMIT $4 OFFSET $5`, ownerID, query.Text, query.Tag, query.Limit+1, query.Offset)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()

	notes := make([]Note, 0, query.Limit+1)
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return Page{}, err
		}
		notes = append(notes, note)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	hasMore := len(notes) > query.Limit
	if hasMore {
		notes = notes[:query.Limit]
	}
	return Page{Notes: notes, Limit: query.Limit, Offset: query.Offset, HasMore: hasMore}, nil
}

func (r *SQLRepository) CreateNote(ctx context.Context, ownerID int64) (Note, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Note{}, err
	}
	var note Note
	err = executor.QueryRowContext(ctx, `
INSERT INTO notes (owner_id)
VALUES ($1)
RETURNING id, owner_id, created_at, updated_at`, ownerID).Scan(
		&note.ID, &note.OwnerID, &note.CreatedAt, &note.UpdatedAt,
	)
	return note, err
}

func (r *SQLRepository) FindNoteByRefCode(ctx context.Context, ownerID int64, refCode string, includeDeleted bool) (Note, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Note{}, err
	}
	note, err := scanNote(executor.QueryRowContext(ctx, noteSelectSQL+`
WHERE n.owner_id = $1
  AND note_ref.ref_code = $2
  AND ($3::boolean OR n.deleted_at IS NULL)`, ownerID, refCode, includeDeleted))
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNoteNotFound
	}
	return note, err
}

func (r *SQLRepository) LockNoteByRefCode(ctx context.Context, ownerID int64, refCode string) (Note, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Note{}, err
	}
	note, err := scanNote(executor.QueryRowContext(ctx, noteSelectSQL+`
WHERE n.owner_id = $1
  AND note_ref.ref_code = $2
FOR UPDATE OF n`, ownerID, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNoteNotFound
	}
	return note, err
}

func (r *SQLRepository) CreateVersion(ctx context.Context, input CreateVersionInput) (Version, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Version{}, err
	}
	var version Version
	err = executor.QueryRowContext(ctx, `
INSERT INTO note_versions (note_id, parent_version_id, version_number, title, content, content_type, operation)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, note_id, parent_version_id, version_number, title, content, content_type, operation, created_at`,
		input.NoteID, input.ParentVersionID, input.VersionNumber, input.Title, input.Content, input.ContentType, input.Operation,
	).Scan(
		&version.ID, &version.NoteID, &version.ParentVersionID, &version.VersionNumber,
		&version.Title, &version.Content, &version.ContentType, &version.Operation, &version.CreatedAt,
	)
	return version, err
}

func (r *SQLRepository) FindVersionByRefCode(ctx context.Context, ownerID int64, refCode string) (Version, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Version{}, err
	}
	version, err := scanVersion(executor.QueryRowContext(ctx, versionSelectSQL+`
WHERE n.owner_id = $1
  AND version_ref.ref_code = $2`, ownerID, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return Version{}, ErrVersionNotFound
	}
	return version, err
}

func (r *SQLRepository) ListVersions(ctx context.Context, ownerID int64, noteRefCode string) ([]Version, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := executor.QueryContext(ctx, versionSelectSQL+`
WHERE n.owner_id = $1
  AND note_ref.ref_code = $2
ORDER BY version.version_number DESC`, ownerID, noteRefCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make([]Version, 0)
	for rows.Next() {
		version, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, ErrNoteNotFound
	}
	return versions, nil
}

func (r *SQLRepository) SetCurrentVersion(ctx context.Context, ownerID int64, noteID int64, versionID int64) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `
UPDATE notes
SET current_version_id = $3, updated_at = NOW()
WHERE owner_id = $1 AND id = $2`, ownerID, noteID, versionID)
	return noteMutationResult(result, err)
}

func (r *SQLRepository) SetNoteDeleted(ctx context.Context, ownerID int64, noteID int64, deleted bool) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	statement := `UPDATE notes SET deleted_at = NOW(), updated_at = NOW() WHERE owner_id = $1 AND id = $2 AND deleted_at IS NULL`
	if !deleted {
		statement = `UPDATE notes SET deleted_at = NULL, updated_at = NOW() WHERE owner_id = $1 AND id = $2 AND deleted_at IS NOT NULL`
	}
	result, err := executor.ExecContext(ctx, statement, ownerID, noteID)
	return noteMutationResult(result, err)
}

func noteMutationResult(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrNoteNotFound
	}
	return nil
}

func (r *SQLRepository) executor(ctx context.Context) (platformdb.Executor, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("notes database is required")
	}
	return platformdb.ExecutorFromContext(ctx, r.database), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNote(row rowScanner) (Note, error) {
	var note Note
	var parentVersionRef sql.NullString
	err := row.Scan(
		&note.ID, &note.OwnerID, &note.CurrentVersionID, &note.ObjectRefID, &note.RefCode,
		&note.CurrentVersionRef, &note.CurrentParentVersionID, &parentVersionRef,
		&note.CurrentVersionNumber, &note.Title, &note.Markdown, &note.ContentType,
		&note.CurrentVersionOperation, pq.Array(&note.Tags), &note.Status,
		&note.CreatedAt, &note.UpdatedAt, &note.DeletedAt,
	)
	if parentVersionRef.Valid {
		note.CurrentParentVersionRef = parentVersionRef.String
	}
	return note, err
}

func scanVersion(row rowScanner) (Version, error) {
	var version Version
	var parentVersionRef sql.NullString
	err := row.Scan(
		&version.ID, &version.NoteID, &version.ParentVersionID, &version.OwnerID,
		&version.ObjectRefID, &version.RefCode, &version.NoteRefCode, &parentVersionRef,
		&version.VersionNumber, &version.Title, &version.Content, &version.ContentType,
		&version.Operation, pq.Array(&version.Tags), &version.CreatedAt,
	)
	if parentVersionRef.Valid {
		version.ParentVersionRef = parentVersionRef.String
	}
	return version, err
}

const noteSelectSQL = `
SELECT n.id, n.owner_id, n.current_version_id,
       note_ref.id, note_ref.ref_code,
       version_ref.ref_code,
       version.parent_version_id, parent_ref.ref_code,
       version.version_number, version.title, version.content, version.content_type, version.operation,
       note_ref.tags, note_ref.status,
       n.created_at, n.updated_at, n.deleted_at
FROM notes AS n
JOIN object_refs AS note_ref
  ON note_ref.owner_id = n.owner_id
 AND note_ref.object_type = 'nte-obj'
 AND note_ref.object_id = n.id
JOIN note_versions AS version ON version.id = n.current_version_id
JOIN object_refs AS version_ref
  ON version_ref.owner_id = n.owner_id
 AND version_ref.object_type = 'version-obj'
 AND version_ref.object_id = version.id
LEFT JOIN note_versions AS parent_version ON parent_version.id = version.parent_version_id
LEFT JOIN object_refs AS parent_ref
  ON parent_ref.owner_id = n.owner_id
 AND parent_ref.object_type = 'version-obj'
 AND parent_ref.object_id = parent_version.id`

const versionSelectSQL = `
SELECT version.id, version.note_id, version.parent_version_id, n.owner_id,
       version_ref.id, version_ref.ref_code, note_ref.ref_code, parent_ref.ref_code,
       version.version_number, version.title, version.content, version.content_type,
       version.operation, version_ref.tags, version.created_at
FROM note_versions AS version
JOIN notes AS n ON n.id = version.note_id
JOIN object_refs AS version_ref
  ON version_ref.owner_id = n.owner_id
 AND version_ref.object_type = 'version-obj'
 AND version_ref.object_id = version.id
JOIN object_refs AS note_ref
  ON note_ref.owner_id = n.owner_id
 AND note_ref.object_type = 'nte-obj'
 AND note_ref.object_id = n.id
LEFT JOIN note_versions AS parent_version ON parent_version.id = version.parent_version_id
LEFT JOIN object_refs AS parent_ref
  ON parent_ref.owner_id = n.owner_id
 AND parent_ref.object_type = 'version-obj'
 AND parent_ref.object_id = parent_version.id`
