// This file defines the LLM HTTP handler dependencies.
package llm

import (
	"context"

	"github.com/stark-lin/saturn/internal/platform/auth"
)

type SessionService interface {
	ListSessions(ctx context.Context, actor auth.Principal, limit int, offset int) (SessionPage, error)
	CreateSession(ctx context.Context, actor auth.Principal, input CreateSessionInput) (Session, error)
	GetSession(ctx context.Context, actor auth.Principal, refCode string, limit int, offset int) (SessionDetail, error)
	CreateRequest(ctx context.Context, actor auth.Principal, sessionRefCode string, input CreateRequestInput) (Request, error)
	GetRequest(ctx context.Context, actor auth.Principal, refCode string) (Request, error)
	DeleteSession(ctx context.Context, actor auth.Principal, refCode string) error
}

type Handler struct {
	service SessionService
}

func NewHandler(service SessionService) *Handler {
	return &Handler{service: service}
}
