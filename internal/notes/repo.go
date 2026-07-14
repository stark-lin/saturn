// This file defines Notes data access boundaries.
package notes

import (
	"context"
	"errors"
)

var ErrRepositoryUnavailable = errors.New("notes repository is not wired")
var ErrNoteNotFound = errors.New("note not found")
var ErrVersionNotFound = errors.New("note version not found")

type Repository interface {
	ListNotes(ctx context.Context, ownerID int64, query Query) (Page, error)
	CreateNote(ctx context.Context, ownerID int64) (Note, error)
	FindNoteByRefCode(ctx context.Context, ownerID int64, refCode string) (Note, error)
	LockNoteByRefCode(ctx context.Context, ownerID int64, refCode string) (Note, error)
	CreateVersion(ctx context.Context, input CreateVersionInput) (Version, error)
	FindVersionByRefCode(ctx context.Context, ownerID int64, refCode string) (Version, error)
	ListVersions(ctx context.Context, ownerID int64, noteRefCode string) ([]Version, error)
	SetCurrentVersion(ctx context.Context, ownerID int64, noteID int64, versionID int64) error
	DeleteNote(ctx context.Context, ownerID int64, noteID int64) error
}

type CreateVersionInput struct {
	NoteID          int64
	ParentVersionID *int64
	VersionNumber   int64
	Title           string
	Content         string
	ContentType     string
	Operation       VersionOperation
}

type Page struct {
	Notes   []Note
	Limit   int
	Offset  int
	HasMore bool
}
