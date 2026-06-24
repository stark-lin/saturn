// This file defines the Notes HTTP handler dependencies.
package notes

import (
	"context"

	"github.com/stark-lin/saturn/internal/platform/auth"
)

type NoteService interface {
	ListNotes(ctx context.Context, actor auth.Principal, query Query) (Page, error)
	CreateNote(ctx context.Context, actor auth.Principal, markdown string) (Note, error)
	GetNote(ctx context.Context, actor auth.Principal, refCode string) (Note, error)
	UpdateNote(ctx context.Context, actor auth.Principal, refCode string, markdown string) (Note, error)
	DeleteNote(ctx context.Context, actor auth.Principal, refCode string) error
}

type Handler struct {
	service NoteService
}

func NewHandler(service NoteService) *Handler {
	return &Handler{service: service}
}
