// This file records append-only audit logs and enforces audit query access.
package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stark-lin/saturn/internal/platform/auth"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
	"github.com/stark-lin/saturn/internal/platform/httpx"
	"github.com/stark-lin/saturn/internal/platform/ref"
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
func (s *Service) RecordAuthentication(ctx context.Context, actorRefCode string, action string, result string, reason string) error {
	return s.RecordStandalone(ctx, Event{
		ActorRefCode:  actorRefCode,
		Action:        Action(action),
		TargetRefCode: SystemTargetRefCode,
		Result:        Result(result),
		Reason:        reason,
	})
}

func (s *Service) RecordActorAction(ctx context.Context, actorRefCode string, action string, targetRefCode string, result string, reason string) error {
	event := Event{
		ActorRefCode: actorRefCode, Action: Action(action), TargetRefCode: targetRefCode,
		Result: Result(result), Reason: reason,
	}
	if _, ok := platformdb.TransactionExecutorFromContext(ctx); ok {
		_, err := s.Record(ctx, event)
		return err
	}
	return s.RecordStandalone(ctx, event)
}

func (s *Service) List(ctx context.Context, actor auth.Principal, query Query) ([]Event, error) {
	if actor.IsZero() {
		return nil, auth.ErrUnauthenticated
	}
	if !actor.IsAdministrator() {
		return nil, auth.ErrForbidden
	}
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("audit repository is required")
	}
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	query.TargetRefCode = ref.NormalizeCode(query.TargetRefCode)
	query.ActorRefCode = strings.ToUpper(strings.TrimSpace(query.ActorRefCode))
	if query.Limit < 1 || query.Limit > MaxLimit || query.Offset < 0 ||
		(query.TargetRefCode != "" && !validTargetRefCode(query.TargetRefCode)) ||
		(query.ActorRefCode != "" && !validActorRefCode(query.ActorRefCode)) ||
		(query.Action != "" && !validQueryAction(query.Action)) ||
		(query.Result != "" && !validQueryResult(query.Result)) {
		return nil, ErrInvalidEvent
	}
	return s.repo.List(ctx, query)
}

func enrichEvent(ctx context.Context, event Event) Event {
	source := httpx.RequestSourceFromContext(ctx)
	event.TargetRefCode = ref.NormalizeCode(event.TargetRefCode)
	event.ActorRefCode = strings.ToUpper(strings.TrimSpace(event.ActorRefCode))
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
	if !validActorRefCode(event.ActorRefCode) {
		return ErrInvalidEvent
	}
	switch event.Action {
	case ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionExport, ActionLogin, ActionLogout:
	default:
		return ErrInvalidEvent
	}
	switch event.Result {
	case ResultSuccess, ResultFailed, ResultDenied:
	default:
		return ErrInvalidEvent
	}
	if !validTargetRefCode(event.TargetRefCode) {
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

func validActorRefCode(value string) bool {
	return auth.ValidUserRefCode(value) || value == SystemTargetRefCode || auth.ValidAPIKeyRefCode(value)
}

func validTargetRefCode(value string) bool {
	if auth.ValidUserRefCode(value) || value == SystemTargetRefCode || auth.ValidAPIKeyRefCode(value) {
		return true
	}
	return ref.CodeMatchesModule(value, ref.ModuleAccounting) ||
		ref.CodeMatchesModule(value, ref.ModuleCalendar) ||
		ref.CodeMatchesModule(value, ref.ModuleFiles) ||
		ref.CodeMatchesModule(value, ref.ModuleLLM) ||
		ref.CodeMatchesModule(value, ref.ModuleNotes)
}
