// This file defines Notes data access boundaries.
package notes

import (
	"context"
	"errors"
)

var ErrRepositoryUnavailable = errors.New("notes repository is not wired")
var ErrNoteNotFound = errors.New("note not found")

type Repository interface {
	ListNotes(ctx context.Context, ownerID int64, query Query) (Page, error)
	CreateNote(ctx context.Context, ownerID int64, title string, markdown string) (Note, error)
	FindNoteByRefCode(ctx context.Context, ownerID int64, refCode string) (Note, error)
	UpdateNote(ctx context.Context, ownerID int64, refCode string, title string, markdown string) (Note, error)
	DeleteNote(ctx context.Context, ownerID int64, noteID int64) error
}

type Page struct {
	Notes   []Note
	Limit   int
	Offset  int
	HasMore bool
}
