// This file records append-only audit logs and enforces audit query access.
package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/auth"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
	"github.com/stark-lin/go-proj/internal/platform/httpx"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

var ErrInvalidEvent = errors.New("invalid audit event")

type Service struct {
	repo         Repository
	transactions platformdb.TransactionRunner
}

func NewService(repo Repository, transactions platformdb.TransactionRunner) *Service {
	return &Service{repo: repo, transactions: transactions}
}

// Record inserts an audit row into the caller's transaction.
func (s *Service) Record(ctx context.Context, event Event) (Event, error) {
	if s == nil || s.repo == nil {
		return Event{}, fmt.Errorf("audit repository is required")
	}
	event = enrichEvent(ctx, event)
	if err := validateEvent(event); err != nil {
		return Event{}, err
	}
	return s.repo.Insert(ctx, event)
}

// RecordStandalone writes an outcome after it is known, in an audit-only transaction.
func (s *Service) RecordStandalone(ctx context.Context, event Event) error {
	if s == nil || s.transactions == nil {
		return fmt.Errorf("audit transaction runner is required")
	}
	if _, ok := platformdb.TransactionExecutorFromContext(ctx); ok {
		return fmt.Errorf("standalone audit insert cannot reuse a business transaction")
	}
	return s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		_, err := s.Record(txCtx, event)
		return err
	})
}

// RecordAuthentication implements the auth package's minimal audit dependency.
func (s *Service) RecordAuthentication(ctx context.Context, actorUserID int64, action string, result string, reason string) error {
	actorType := ActorTypeAnonymous
	if actorUserID > 0 {
		actorType = ActorTypeUser
	}
	return s.RecordStandalone(ctx, Event{
		ActorType:     actorType,
		ActorUserID:   actorUserID,
		Action:        Action(action),
		TargetRefCode: SystemTargetRefCode,
		Result:        Result(result),
		Reason:        reason,
	})
}

func (s *Service) List(ctx context.Context, actor auth.Principal, query Query) ([]Event, error) {
	if actor.IsZero() {
		return nil, auth.ErrUnauthenticated
	}
	if !actor.IsSuperuser() {
		return nil, auth.ErrForbidden
	}
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("audit repository is required")
	}
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	query.TargetRefCode = ref.NormalizeCode(query.TargetRefCode)
	if query.Limit < 1 || query.Limit > MaxLimit || query.Offset < 0 ||
		(query.TargetRefCode != "" && !ref.ValidCode(query.TargetRefCode)) ||
		(query.Action != "" && !validQueryAction(query.Action)) ||
		(query.Result != "" && !validQueryResult(query.Result)) {
		return nil, ErrInvalidEvent
	}
	return s.repo.List(ctx, query)
}

func enrichEvent(ctx context.Context, event Event) Event {
	source := httpx.RequestSourceFromContext(ctx)
	event.TargetRefCode = ref.NormalizeCode(event.TargetRefCode)
	event.Reason = strings.TrimSpace(event.Reason)
	if event.SourceIP == "" {
		event.SourceIP = source.IP
	}
	if event.UserAgent == "" {
		event.UserAgent = source.UserAgent
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	return event
}

func validateEvent(event Event) error {
	switch event.ActorType {
	case ActorTypeUser:
		if event.ActorUserID < 1 {
			return ErrInvalidEvent
		}
	case ActorTypeSystem, ActorTypeLLM, ActorTypeAnonymous:
		if event.ActorUserID != 0 {
			return ErrInvalidEvent
		}
	default:
		return ErrInvalidEvent
	}
	switch event.Action {
	case ActionCreate, ActionUpdate, ActionDelete, ActionExport, ActionLogin, ActionLogout:
	case ActionRead:
		if event.ActorType != ActorTypeLLM {
			return ErrInvalidEvent
		}
	default:
		return ErrInvalidEvent
	}
	switch event.Result {
	case ResultSuccess, ResultFailed, ResultDenied:
	default:
		return ErrInvalidEvent
	}
	if event.TargetRefCode == "" || (!ref.ValidCode(event.TargetRefCode) && event.TargetRefCode != SystemTargetRefCode) {
		return ErrInvalidEvent
	}
	if (event.Action == ActionLogin || event.Action == ActionLogout) && event.TargetRefCode != SystemTargetRefCode {
		return ErrInvalidEvent
	}
	if strings.TrimSpace(event.SourceIP) == "" {
		return ErrInvalidEvent
	}
	return nil
}
