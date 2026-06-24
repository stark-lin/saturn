// This file enforces LLM session, request, result, reference, and audit boundaries.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stark-lin/saturn/internal/platform/audit"
	"github.com/stark-lin/saturn/internal/platform/auth"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

var (
	ErrDependencyUnavailable = errors.New("llm dependency is not wired")
	ErrInvalidSession        = errors.New("invalid llm session")
	ErrInvalidRequest        = errors.New("invalid llm request")
	ErrInvalidQuery          = errors.New("invalid llm query")
	ErrReferenceNotFound     = errors.New("llm reference not found")
	ErrClientUnavailable     = errors.New("llm client is not configured")
)

const (
	systemPrompt = "You answer using only the user request and the authorized Saturn context JSON supplied in this request."
)

type ObjectReferenceService interface {
	ClaimCode(ctx context.Context, objectType ref.ObjectType) (string, error)
	Register(ctx context.Context, registration ref.Registration) (ref.ObjectRef, error)
	UpdateProjection(ctx context.Context, update ref.ProjectionUpdate) (ref.ObjectRef, error)
	Delete(ctx context.Context, ownerID int64, objectType ref.ObjectType, objectID int64) error
}

type AuditService interface {
	Record(ctx context.Context, event audit.Event) (audit.Event, error)
	RecordStandalone(ctx context.Context, event audit.Event) error
}

type Client interface {
	Complete(ctx context.Context, requestJSON json.RawMessage) (ClientResult, error)
}

type ReferenceResolver interface {
	Resolve(ctx context.Context, actor auth.Principal, refCode string) (ResolvedReference, error)
}

type RuntimeConfig struct {
	Enabled   bool
	Model     string
	MaxTokens int
}

type Service struct {
	repo         Repository
	transactions platformdb.TransactionRunner
	references   ObjectReferenceService
	audit        AuditService
	client       Client
	resolver     ReferenceResolver
	config       RuntimeConfig
	authorizer   *auth.Authorizer
}

type ServiceDependencies struct {
	Repository   Repository
	Transactions platformdb.TransactionRunner
	References   ObjectReferenceService
	Audit        AuditService
	Client       Client
	Resolver     ReferenceResolver
	Config       RuntimeConfig
}

func NewService(deps ServiceDependencies) *Service {
	if deps.Transactions == nil {
		deps.Transactions = platformdb.NoopTransactionRunner{}
	}
	if deps.Config.Model == "" {
		deps.Config.Model = "gpt-4o-mini"
	}
	if deps.Config.MaxTokens == 0 {
		deps.Config.MaxTokens = DefaultMaxTokens
	}
	return &Service{
		repo: deps.Repository, transactions: deps.Transactions, references: deps.References, audit: deps.Audit,
		client: deps.Client, resolver: deps.Resolver, config: deps.Config, authorizer: auth.NewAuthorizer(),
	}
}

func (s *Service) ListSessions(ctx context.Context, actor auth.Principal, limit int, offset int) (SessionPage, error) {
	if actor.IsZero() {
		return SessionPage{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return SessionPage{}, ErrRepositoryUnavailable
	}
	limit, offset, err := normalizePagination(limit, offset)
	if err != nil {
		return SessionPage{}, err
	}
	return s.repo.ListSessions(ctx, auth.ScopeForPrincipal(actor), limit, offset)
}

func (s *Service) CreateSession(ctx context.Context, actor auth.Principal, input CreateSessionInput) (Session, error) {
	if actor.IsZero() {
		return Session{}, auth.ErrUnauthenticated
	}
	input, err := normalizeSessionInput(input)
	if err != nil {
		return Session{}, err
	}
	if err := s.requireCreateDependencies(); err != nil {
		return Session{}, err
	}
	refCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeLLMSession)
	if err != nil {
		return Session{}, err
	}
	var created Session
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, err := s.repo.CreateSession(txCtx, actor.ID, input)
		if err != nil {
			return err
		}
		object, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: actor.ID, RefCode: refCode, ObjectType: ref.ObjectTypeLLMSession,
			ObjectID: session.ID, Title: session.Title, Tags: input.Tags, Status: string(SessionStatusActive),
		})
		if err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
			TargetRefCode: object.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		session.ObjectRefID = object.ID
		session.RefCode = object.RefCode
		session.Tags = input.Tags
		created = session
		return nil
	})
	if err != nil {
		return Session{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, refCode, err)
	}
	return created, nil
}

func (s *Service) GetSession(ctx context.Context, actor auth.Principal, refCode string, limit int, offset int) (SessionDetail, error) {
	if actor.IsZero() {
		return SessionDetail{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return SessionDetail{}, ErrRepositoryUnavailable
	}
	refCode = ref.NormalizeCode(refCode)
	limit, offset, err := normalizePagination(limit, offset)
	if err != nil {
		return SessionDetail{}, err
	}
	session, err := s.repo.FindSessionByRefCode(ctx, auth.ScopeForPrincipal(actor), refCode)
	if err != nil {
		return SessionDetail{}, err
	}
	requests, err := s.repo.ListRequests(ctx, session.OwnerID, session.ID, limit, offset)
	if err != nil {
		return SessionDetail{}, err
	}
	return SessionDetail{Session: session, Requests: requests}, nil
}

func (s *Service) GetRequest(ctx context.Context, actor auth.Principal, refCode string) (Request, error) {
	if actor.IsZero() {
		return Request{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return Request{}, ErrRepositoryUnavailable
	}
	refCode = ref.NormalizeCode(refCode)
	return s.repo.FindRequestByRefCode(ctx, auth.ScopeForPrincipal(actor), refCode)
}

func (s *Service) CreateRequest(ctx context.Context, actor auth.Principal, sessionRefCode string, input CreateRequestInput) (Request, error) {
	if actor.IsZero() {
		return Request{}, auth.ErrUnauthenticated
	}
	if err := s.requireRequestDependencies(); err != nil {
		return Request{}, err
	}
	sessionRefCode = ref.NormalizeCode(sessionRefCode)
	input, err := s.normalizeRequestInput(input)
	if err != nil {
		return Request{}, err
	}
	session, err := s.repo.FindSessionByRefCode(ctx, auth.ScopeForPrincipal(actor), sessionRefCode)
	if err != nil {
		return Request{}, err
	}
	if err := s.authorizer.Can(actor, auth.ActionCreate, auth.Resource{Type: "llm_session", ID: session.ID, OwnerID: session.OwnerID}); err != nil {
		return Request{}, err
	}
	resolved, err := s.resolveReferences(ctx, actor, input.References)
	if err != nil {
		return Request{}, err
	}
	contextJSON, err := marshalContextJSON(resolved)
	if err != nil {
		return Request{}, err
	}
	requestJSON, err := marshalOpenAIRequest(input.Model, input.MaxTokens, input.Prompt, contextJSON)
	if err != nil {
		return Request{}, err
	}

	requestRefCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeLLMRequest)
	if err != nil {
		return Request{}, err
	}

	var request Request
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		lockedSession, err := s.repo.LockSessionByRefCode(txCtx, sessionRefCode)
		if err != nil {
			return err
		}
		if err := s.authorizer.Can(actor, auth.ActionCreate, auth.Resource{Type: "llm_session", ID: lockedSession.ID, OwnerID: lockedSession.OwnerID}); err != nil {
			return err
		}
		createdRequest, err := s.repo.CreateRequest(txCtx, lockedSession.OwnerID, lockedSession.ID, PersistedRequestInput{
			ActorUserID: actor.ID, Prompt: input.Prompt, Model: input.Model, MaxTokens: input.MaxTokens,
			ContextJSON: contextJSON, RequestJSON: requestJSON,
		})
		if err != nil {
			return err
		}
		requestObject, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: lockedSession.OwnerID, RefCode: requestRefCode, ObjectType: ref.ObjectTypeLLMRequest,
			ObjectID: createdRequest.ID, Title: requestProjectionTitle(createdRequest.Prompt), Tags: input.Tags, Status: string(ResponseStatusQueued),
		})
		if err != nil {
			return err
		}
		createdRequest.ObjectRefID = requestObject.ID
		createdRequest.RefCode = requestObject.RefCode
		createdRequest.Tags = input.Tags
		createdRequest.References = make([]RequestReference, 0, len(resolved))
		for _, reference := range resolved {
			storedReference, err := s.repo.InsertRequestReference(txCtx, createdRequest.ID, reference)
			if err != nil {
				return err
			}
			createdRequest.References = append(createdRequest.References, storedReference)
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
			TargetRefCode: createdRequest.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		request = createdRequest
		return nil
	})
	if err != nil {
		return Request{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, requestRefCode, err)
	}

	return request, nil
}

func (s *Service) ProcessNextQueuedRequest(ctx context.Context, requestTimeout time.Duration) (bool, error) {
	if err := s.requireWorkerDependencies(); err != nil {
		return false, err
	}
	var request Request
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		request, err = s.repo.ClaimNextQueuedRequest(txCtx)
		if err != nil {
			return err
		}
		_, err = s.references.UpdateProjection(txCtx, ref.ProjectionUpdate{
			OwnerID: request.OwnerID, ObjectType: ref.ObjectTypeLLMRequest, ObjectID: request.ID,
			Title: requestProjectionTitle(request.Prompt), Tags: request.Tags, Status: string(ResponseStatusRunning),
		})
		return err
	})
	if errors.Is(err, ErrNoQueuedRequest) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = s.completeRequestResponse(ctx, request.ActorUserID, request, requestTimeout)
	return true, err
}

func (s *Service) completeRequestResponse(ctx context.Context, actorUserID int64, request Request, requestTimeout time.Duration) (Response, error) {
	input := CompleteResponseInput{Status: ResponseStatusError, ErrorCode: "llm_unavailable", ErrorMessage: ErrClientUnavailable.Error(), ResponseJSON: json.RawMessage(`{}`)}
	if s.config.Enabled && s.client != nil {
		providerCtx := ctx
		var cancel context.CancelFunc
		if requestTimeout > 0 {
			providerCtx, cancel = context.WithTimeout(ctx, requestTimeout)
			defer cancel()
		}
		result, err := s.client.Complete(providerCtx, request.RequestJSON)
		if err != nil {
			input.ErrorCode = "llm_request_failed"
			input.ErrorMessage = err.Error()
			if providerTimedOut(providerCtx, err) {
				input.ErrorCode = "llm_request_timeout"
				input.ErrorMessage = "llm request timed out"
			}
			input.ResponseJSON = errorResponseJSON(input.ErrorCode, input.ErrorMessage)
		} else {
			input.Status = ResponseStatusSuccess
			input.Content = result.Content
			input.ResponseJSON = result.RawJSON
			if len(input.ResponseJSON) == 0 {
				input.ResponseJSON = json.RawMessage(`{}`)
			}
			input.ErrorCode = ""
			input.ErrorMessage = ""
		}
	}

	var completed Response
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		completed, err = s.repo.CompleteRequestResponse(txCtx, request.OwnerID, request.ID, input)
		if err != nil {
			return err
		}
		if _, err := s.references.UpdateProjection(txCtx, ref.ProjectionUpdate{
			OwnerID: completed.OwnerID, ObjectType: ref.ObjectTypeLLMRequest, ObjectID: request.ID,
			Title: requestProjectionTitle(request.Prompt), Tags: request.Tags, Status: string(completed.Status),
		}); err != nil {
			return err
		}
		reason := ""
		result := audit.ResultSuccess
		if completed.Status == ResponseStatusError {
			result = audit.ResultFailed
			reason = completed.ErrorCode
		}
		_, err = s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actorUserID, Action: audit.ActionUpdate,
			TargetRefCode: request.RefCode, Result: result, Reason: reason,
			SourceIP: "127.0.0.1", UserAgent: "saturn-llm-worker",
		})
		return err
	})
	return completed, err
}

func providerTimedOut(ctx context.Context, err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func (s *Service) DeleteSession(ctx context.Context, actor auth.Principal, refCode string) error {
	if actor.IsZero() {
		return auth.ErrUnauthenticated
	}
	if err := s.requireWriteDependencies(); err != nil {
		return err
	}
	refCode = ref.NormalizeCode(refCode)
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		session, err := s.repo.LockSessionByRefCode(txCtx, refCode)
		if err != nil {
			return err
		}
		if err := s.authorizer.Can(actor, auth.ActionDelete, auth.Resource{Type: "llm_session", ID: session.ID, OwnerID: session.OwnerID}); err != nil {
			return err
		}
		requests, err := s.repo.ListRequestDeletionTargets(txCtx, session.OwnerID, session.ID)
		if err != nil {
			return err
		}
		for _, request := range requests {
			if _, err := s.audit.Record(txCtx, audit.Event{
				ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
				TargetRefCode: request.RefCode, Result: audit.ResultSuccess, Reason: "cascade_llm_session",
			}); err != nil {
				return err
			}
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
			TargetRefCode: session.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		for _, request := range requests {
			if err := s.references.Delete(txCtx, session.OwnerID, ref.ObjectTypeLLMRequest, request.ID); err != nil {
				return err
			}
		}
		if err := s.references.Delete(txCtx, session.OwnerID, ref.ObjectTypeLLMSession, session.ID); err != nil {
			return err
		}
		return s.repo.DeleteSession(txCtx, session.OwnerID, session.ID)
	})
	if err != nil {
		return s.recordWriteFailure(ctx, actor, audit.ActionDelete, refCode, err)
	}
	return nil
}

func (s *Service) resolveReferences(ctx context.Context, actor auth.Principal, refCodes []string) ([]ResolvedReference, error) {
	resolved := make([]ResolvedReference, 0, len(refCodes))
	for _, rawCode := range refCodes {
		code := ref.NormalizeCode(rawCode)
		if !ref.ValidCode(code) {
			return nil, ErrInvalidRequest
		}
		reference, err := s.resolver.Resolve(ctx, actor, code)
		if err != nil {
			_ = s.recordLLMRead(ctx, code, audit.ResultDenied, "not_found")
			return nil, errors.Join(ErrReferenceNotFound, err)
		}
		if err := s.recordLLMRead(ctx, code, audit.ResultSuccess, ""); err != nil {
			return nil, err
		}
		resolved = append(resolved, reference)
	}
	return resolved, nil
}

func (s *Service) recordLLMRead(ctx context.Context, refCode string, result audit.Result, reason string) error {
	if s.audit == nil {
		return ErrDependencyUnavailable
	}
	return s.audit.RecordStandalone(ctx, audit.Event{
		ActorType: audit.ActorTypeLLM, Action: audit.ActionRead,
		TargetRefCode: refCode, Result: result, Reason: reason,
	})
}

func (s *Service) recordWriteFailure(ctx context.Context, actor auth.Principal, action audit.Action, refCode string, operationErr error) error {
	if s.audit == nil {
		return operationErr
	}
	result := audit.ResultFailed
	reason := "operation_failed"
	if errors.Is(operationErr, ErrSessionNotFound) || errors.Is(operationErr, ErrRequestNotFound) ||
		errors.Is(operationErr, auth.ErrForbidden) || errors.Is(operationErr, ref.ErrNotFound) {
		result = audit.ResultDenied
		reason = "not_found"
	}
	auditErr := s.audit.RecordStandalone(ctx, audit.Event{
		ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: action,
		TargetRefCode: refCode, Result: result, Reason: reason,
	})
	if auditErr != nil {
		return errors.Join(operationErr, auditErr)
	}
	return operationErr
}

func (s *Service) requireWriteDependencies() error {
	if s.repo == nil {
		return ErrRepositoryUnavailable
	}
	if s.references == nil || s.audit == nil {
		return ErrDependencyUnavailable
	}
	return nil
}

func (s *Service) requireCreateDependencies() error {
	return s.requireWriteDependencies()
}

func (s *Service) requireRequestDependencies() error {
	if err := s.requireCreateDependencies(); err != nil {
		return err
	}
	if s.resolver == nil {
		return ErrDependencyUnavailable
	}
	return nil
}

func (s *Service) requireWorkerDependencies() error {
	return s.requireWriteDependencies()
}

func normalizeSessionInput(input CreateSessionInput) (CreateSessionInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Tags = normalizedTags(input.Tags)
	if input.Title == "" {
		return CreateSessionInput{}, ErrInvalidSession
	}
	return input, nil
}

func (s *Service) normalizeRequestInput(input CreateRequestInput) (CreateRequestInput, error) {
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.Model = strings.TrimSpace(input.Model)
	if input.Model == "" {
		input.Model = s.config.Model
	}
	if input.MaxTokens == 0 {
		input.MaxTokens = s.config.MaxTokens
	}
	input.References = normalizedRefCodes(input.References)
	input.Tags = normalizedTags(input.Tags)
	if input.Prompt == "" || input.Model == "" || input.MaxTokens < 1 || input.MaxTokens > s.config.MaxTokens {
		return CreateRequestInput{}, ErrInvalidRequest
	}
	return input, nil
}

func normalizePagination(limit int, offset int) (int, int, error) {
	if limit == 0 {
		limit = DefaultLimit
	}
	if limit < 1 || limit > MaxLimit || offset < 0 {
		return 0, 0, ErrInvalidQuery
	}
	return limit, offset, nil
}

func normalizedRefCodes(values []string) []string {
	seen := make(map[string]struct{})
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		code := ref.NormalizeCode(value)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}
	return normalized
}

func normalizedTags(names []string) []string {
	tags := make([]string, 0, len(names))
	seen := make(map[string]struct{})
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}
	return tags
}

func marshalContextJSON(references []ResolvedReference) (json.RawMessage, error) {
	payload := struct {
		References []ResolvedReference `json:"references"`
	}{References: references}
	content, err := json.Marshal(payload)
	return json.RawMessage(content), err
}

func marshalOpenAIRequest(model string, maxTokens int, prompt string, contextJSON json.RawMessage) (json.RawMessage, error) {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": fmt.Sprintf("Context JSON:\n%s", string(contextJSON))},
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
	}
	content, err := json.Marshal(payload)
	return json.RawMessage(content), err
}

func errorResponseJSON(code string, message string) json.RawMessage {
	content, err := json.Marshal(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(content)
}

func requestProjectionTitle(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	runes := []rune(prompt)
	if len(runes) <= 80 {
		return prompt
	}
	return strings.TrimSpace(string(runes[:80]))
}
