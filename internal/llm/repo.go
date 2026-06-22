// This file defines LLM data access boundaries.
package llm

import (
	"context"
	"errors"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

var (
	ErrRepositoryUnavailable = errors.New("llm repository is not wired")
	ErrSessionNotFound       = errors.New("llm session not found")
	ErrRequestNotFound       = errors.New("llm request not found")
	ErrRequestAlreadyFinal   = errors.New("llm request is already final")
	ErrNoQueuedRequest       = errors.New("no queued llm request")
)

type Repository interface {
	ListSessions(ctx context.Context, scope auth.Scope, limit int, offset int) (SessionPage, error)
	CreateSession(ctx context.Context, ownerID int64, input CreateSessionInput) (Session, error)
	FindSessionByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Session, error)
	LockSessionByRefCode(ctx context.Context, refCode string) (Session, error)
	DeleteSession(ctx context.Context, ownerID int64, sessionID int64) error

	CreateRequest(ctx context.Context, ownerID int64, sessionID int64, input PersistedRequestInput) (Request, error)
	FindRequestByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Request, error)
	InsertRequestReference(ctx context.Context, requestID int64, reference ResolvedReference) (RequestReference, error)
	ListRequests(ctx context.Context, ownerID int64, sessionID int64, limit int, offset int) ([]Request, error)
	ListRequestDeletionTargets(ctx context.Context, ownerID int64, sessionID int64) ([]RequestDeletionTarget, error)
	ClaimNextQueuedRequest(ctx context.Context) (Request, error)
	CompleteRequestResponse(ctx context.Context, ownerID int64, requestID int64, input CompleteResponseInput) (Response, error)
}
