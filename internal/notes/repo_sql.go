// This file persists current Notes through PostgreSQL for the owner-only API.
package notes

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

func (r *SQLRepository) ListNotes(ctx context.Context, ownerID int64, query Query) (Page, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Page{}, err
	}
	rows, err := executor.QueryContext(ctx, `
	SELECT n.id, n.owner_id, object_ref.id, object_ref.ref_code, n.title, n.markdown, object_ref.status, n.created_at, n.updated_at
FROM notes AS n
JOIN object_refs AS object_ref
  ON object_ref.owner_id = n.owner_id
 AND object_ref.object_type = 'note'
 AND object_ref.object_id = n.id
WHERE n.owner_id = $1
  AND ($2::text = '' OR n.title ILIKE '%' || $2 || '%' OR n.markdown ILIKE '%' || $2 || '%')
  AND (
    $3::text = ''
    OR object_ref.tags @> ARRAY[$3]::text[]
  )
ORDER BY n.updated_at DESC, object_ref.ref_code DESC
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

func (r *SQLRepository) CreateNote(ctx context.Context, ownerID int64, title string, markdown string) (Note, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Note{}, err
	}
	var note Note
	err = executor.QueryRowContext(ctx, `
	INSERT INTO notes (owner_id, title, markdown)
	VALUES ($1, $2, $3)
RETURNING id, owner_id, title, markdown, created_at, updated_at`, ownerID, title, markdown).Scan(
		&note.ID,
		&note.OwnerID,
		&note.Title,
		&note.Markdown,
		&note.CreatedAt,
		&note.UpdatedAt,
	)
	return note, err
}

func (r *SQLRepository) FindNoteByRefCode(ctx context.Context, ownerID int64, refCode string) (Note, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Note{}, err
	}
	note, err := scanNote(executor.QueryRowContext(ctx, `
	SELECT n.id, n.owner_id, object_ref.id, object_ref.ref_code, n.title, n.markdown, object_ref.status, n.created_at, n.updated_at
FROM notes AS n
JOIN object_refs AS object_ref
  ON object_ref.owner_id = n.owner_id
 AND object_ref.object_type = 'note'
 AND object_ref.object_id = n.id
WHERE n.owner_id = $1
  AND object_ref.ref_code = $2`, ownerID, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNoteNotFound
	}
	return note, err
}

func (r *SQLRepository) UpdateNote(ctx context.Context, ownerID int64, refCode string, title string, markdown string) (Note, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Note{}, err
	}
	note, err := scanNote(executor.QueryRowContext(ctx, `
UPDATE notes AS n
SET title = $3,
    markdown = $4,
    updated_at = NOW()
FROM object_refs AS object_ref
WHERE n.owner_id = $1
  AND object_ref.owner_id = n.owner_id
  AND object_ref.object_type = 'note'
  AND object_ref.object_id = n.id
  AND object_ref.ref_code = $2
	RETURNING n.id, n.owner_id, object_ref.id, object_ref.ref_code, n.title, n.markdown, object_ref.status, n.created_at, n.updated_at`,
		ownerID, refCode, title, markdown))
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNoteNotFound
	}
	return note, err
}

func (r *SQLRepository) DeleteNote(ctx context.Context, ownerID int64, noteID int64) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `DELETE FROM notes WHERE owner_id = $1 AND id = $2`, ownerID, noteID)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
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
	err := row.Scan(
		&note.ID,
		&note.OwnerID,
		&note.ObjectRefID,
		&note.RefCode,
		&note.Title,
		&note.Markdown,
		&note.Status,
		&note.CreatedAt,
		&note.UpdatedAt,
	)
	return note, err
}
