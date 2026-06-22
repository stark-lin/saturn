// This file tests LLM service request and response orchestration.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/audit"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

func TestNewModuleBuildsLLMDependencies(t *testing.T) {
	module := NewModule(ServiceDependencies{})
	if module.Service == nil {
		t.Fatal("expected service")
	}
	if module.Handler == nil {
		t.Fatal("expected handler")
	}
}

func TestCreateRequestPersistsQueuedRequest(t *testing.T) {
	repo := newFakeRepository()
	client := &fakeClient{result: ClientResult{
		Content: "answer",
		RawJSON: json.RawMessage(`{"choices":[{"message":{"content":"answer"}}]}`),
	}}
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: newFakeReferences("LLM-00000002"),
		Audit:      &fakeAudit{},
		Client:     client,
		Resolver: &fakeResolver{reference: ResolvedReference{
			ObjectRefID: 10,
			RefCode:     "ACC-00000001",
			Module:      "accounting",
			ObjectType:  "account",
			Title:       "Cash",
			Status:      "active",
			Tags:        []string{"finance"},
			PayloadJSON: json.RawMessage(`{"ref_code":"ACC-00000001","name":"Cash"}`),
		}},
		Config: RuntimeConfig{Enabled: true, Model: "test-model", MaxTokens: 100},
	})

	request, err := service.CreateRequest(context.Background(), testActor(), "LLM-00000001", CreateRequestInput{
		Prompt:     "Summarize this account",
		References: []string{"ACC-00000001"},
		Tags:       []string{" review ", "monthly", "review"},
	})
	if err != nil {
		t.Fatalf("CreateRequest error = %v", err)
	}
	if request.RefCode != "LLM-00000002" {
		t.Fatalf("request ref_code = %q", request.RefCode)
	}
	if request.ResponseStatus != ResponseStatusQueued || request.ResponseContent != "" {
		t.Fatalf("request response = %#v, want queued without content", request)
	}
	if repo.createdRequest.ResponseStatus != ResponseStatusQueued {
		t.Fatalf("initial response status = %q, want queued", repo.createdRequest.ResponseStatus)
	}
	if repo.createdRequest.ActorUserID != testActor().ID {
		t.Fatalf("actor user id = %d, want %d", repo.createdRequest.ActorUserID, testActor().ID)
	}
	if client.calls != 0 {
		t.Fatalf("provider calls = %d, want 0 before worker", client.calls)
	}
	if len(request.References) != 1 || request.References[0].RefCode != "ACC-00000001" {
		t.Fatalf("request references = %#v", request.References)
	}
	if len(request.Tags) != 2 || request.Tags[0] != "review" || request.Tags[1] != "monthly" {
		t.Fatalf("request tags = %#v", request.Tags)
	}
	if !json.Valid(request.ContextJSON) || !json.Valid(request.RequestJSON) {
		t.Fatalf("stored request JSON is invalid")
	}
	var contextPayload struct {
		References []ResolvedReference `json:"references"`
	}
	if err := json.Unmarshal(request.ContextJSON, &contextPayload); err != nil {
		t.Fatalf("decode context json: %v", err)
	}
	if len(contextPayload.References) != 1 || contextPayload.References[0].RefCode != "ACC-00000001" {
		t.Fatalf("context references = %#v", contextPayload.References)
	}
	var providerPayload struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(request.RequestJSON, &providerPayload); err != nil {
		t.Fatalf("decode request json: %v", err)
	}
	if providerPayload.Model != "test-model" || providerPayload.MaxTokens != 100 || len(providerPayload.Messages) != 3 {
		t.Fatalf("provider request = %#v", providerPayload)
	}
	if providerPayload.Messages[0].Role != "system" || providerPayload.Messages[2].Content != "Summarize this account" {
		t.Fatalf("provider messages = %#v", providerPayload.Messages)
	}
}

func TestCreateSessionAssociatesTags(t *testing.T) {
	references := newFakeReferences("LLM-00000001")
	service := NewService(ServiceDependencies{
		Repository: newFakeRepository(),
		References: references,
		Audit:      &fakeAudit{},
	})

	session, err := service.CreateSession(context.Background(), testActor(), CreateSessionInput{
		Title: "Planning",
		Tags:  []string{" planning ", "", "llm", "planning"},
	})
	if err != nil {
		t.Fatalf("CreateSession error = %v", err)
	}
	if session.RefCode != "LLM-00000001" {
		t.Fatalf("session ref_code = %q", session.RefCode)
	}
	if len(session.Tags) != 2 || session.Tags[0] != "planning" || session.Tags[1] != "llm" {
		t.Fatalf("session tags = %#v", session.Tags)
	}
	if len(references.registrations) != 1 ||
		len(references.registrations[0].Tags) != 2 ||
		references.registrations[0].Tags[0] != "planning" ||
		references.registrations[0].Tags[1] != "llm" {
		t.Fatalf("registration tags = %#v", references.registrations)
	}
}

func TestListSessionsNormalizesPaginationAndScopesByActor(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(ServiceDependencies{Repository: repo})

	page, err := service.ListSessions(context.Background(), testActor(), 0, 3)
	if err != nil {
		t.Fatalf("ListSessions error = %v", err)
	}
	if page.Limit != DefaultLimit || page.Offset != 3 {
		t.Fatalf("page = %#v", page)
	}
	if repo.listLimit != DefaultLimit || repo.listOffset != 3 {
		t.Fatalf("repo pagination = %d/%d", repo.listLimit, repo.listOffset)
	}
	if repo.listScope.All || repo.listScope.OwnerID != testActor().ID {
		t.Fatalf("scope = %#v", repo.listScope)
	}

	superuser := auth.Principal{ID: 9, Role: auth.RoleSuperuser}
	if _, err := service.ListSessions(context.Background(), superuser, 10, 0); err != nil {
		t.Fatalf("superuser ListSessions error = %v", err)
	}
	if !repo.listScope.All {
		t.Fatalf("superuser scope = %#v", repo.listScope)
	}
}

func TestListSessionsRejectsInvalidInputs(t *testing.T) {
	service := NewService(ServiceDependencies{Repository: newFakeRepository()})
	if _, err := service.ListSessions(context.Background(), auth.Principal{}, 10, 0); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("unauthenticated error = %v", err)
	}
	if _, err := service.ListSessions(context.Background(), testActor(), MaxLimit+1, 0); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("invalid pagination error = %v", err)
	}
	if _, err := NewService(ServiceDependencies{}).ListSessions(context.Background(), testActor(), 10, 0); !errors.Is(err, ErrRepositoryUnavailable) {
		t.Fatalf("missing repo error = %v", err)
	}
}

func TestGetSessionReturnsRequestsWithPagination(t *testing.T) {
	repo := newFakeRepository()
	repo.createdRequest = Request{ID: 2, OwnerID: 1, SessionID: 1, RefCode: "LLM-00000002", ResponseStatus: ResponseStatusSuccess}
	service := NewService(ServiceDependencies{Repository: repo})

	detail, err := service.GetSession(context.Background(), testActor(), " llm-00000001 ", 5, 2)
	if err != nil {
		t.Fatalf("GetSession error = %v", err)
	}
	if detail.Session.RefCode != "LLM-00000001" || len(detail.Requests) != 1 {
		t.Fatalf("detail = %#v", detail)
	}
	if repo.listRequestsLimit != 5 || repo.listRequestsOffset != 2 {
		t.Fatalf("request pagination = %d/%d", repo.listRequestsLimit, repo.listRequestsOffset)
	}
}

func TestGetRequestUsesActorScope(t *testing.T) {
	repo := newFakeRepository()
	repo.createdRequest = Request{ID: 2, OwnerID: 1, SessionID: 1, RefCode: "LLM-00000002", ResponseStatus: ResponseStatusSuccess}
	service := NewService(ServiceDependencies{Repository: repo})

	request, err := service.GetRequest(context.Background(), testActor(), " llm-00000002 ")
	if err != nil {
		t.Fatalf("GetRequest error = %v", err)
	}
	if request.RefCode != "LLM-00000002" {
		t.Fatalf("request = %#v", request)
	}
	if repo.findRequestScope.All || repo.findRequestScope.OwnerID != testActor().ID {
		t.Fatalf("scope = %#v", repo.findRequestScope)
	}
}

func TestProcessNextQueuedRequestCompletesSuccess(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: newFakeReferences("LLM-00000002"),
		Audit:      &fakeAudit{},
		Client: &fakeClient{result: ClientResult{
			Content: "answer",
			RawJSON: json.RawMessage(`{"choices":[{"message":{"content":"answer"}}]}`),
		}},
		Resolver: &fakeResolver{},
		Config:   RuntimeConfig{Enabled: true, Model: "test-model", MaxTokens: 100},
	})

	_, err := service.CreateRequest(context.Background(), testActor(), "LLM-00000001", CreateRequestInput{
		Prompt: "Answer without references",
	})
	if err != nil {
		t.Fatalf("CreateRequest error = %v", err)
	}

	processed, err := service.ProcessNextQueuedRequest(context.Background(), 0)
	if err != nil {
		t.Fatalf("ProcessNextQueuedRequest error = %v", err)
	}
	if !processed {
		t.Fatal("expected queued request to be processed")
	}
	if !repo.startedRequest {
		t.Fatal("request was not moved to running before provider call")
	}
	if repo.completedInput.Status != ResponseStatusSuccess || repo.createdRequest.ResponseContent != "answer" {
		t.Fatalf("completed request = %#v; input = %#v", repo.createdRequest, repo.completedInput)
	}
}

func TestProcessNextQueuedRequestFinalizesErrorWhenProviderFails(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: newFakeReferences("LLM-00000002"),
		Audit:      &fakeAudit{},
		Client:     &fakeClient{err: errors.New("provider unavailable")},
		Resolver:   &fakeResolver{},
		Config:     RuntimeConfig{Enabled: true, Model: "test-model", MaxTokens: 100},
	})

	_, err := service.CreateRequest(context.Background(), testActor(), "LLM-00000001", CreateRequestInput{
		Prompt: "Answer without references",
	})
	if err != nil {
		t.Fatalf("CreateRequest error = %v", err)
	}

	processed, err := service.ProcessNextQueuedRequest(context.Background(), 0)
	if err != nil {
		t.Fatalf("ProcessNextQueuedRequest error = %v", err)
	}
	if !processed {
		t.Fatal("expected queued request to be processed")
	}
	if repo.createdRequest.ResponseStatus != ResponseStatusError {
		t.Fatalf("response status = %q, want error", repo.createdRequest.ResponseStatus)
	}
	if repo.createdRequest.ResponseErrorCode != "llm_request_failed" || repo.createdRequest.ResponseErrorMessage == "" {
		t.Fatalf("response error = %#v", repo.createdRequest)
	}
}

func TestProcessNextQueuedRequestFinalizesTimeout(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: newFakeReferences("LLM-00000002"),
		Audit:      &fakeAudit{},
		Client:     &fakeClient{waitForContext: true},
		Resolver:   &fakeResolver{},
		Config:     RuntimeConfig{Enabled: true, Model: "test-model", MaxTokens: 100},
	})

	_, err := service.CreateRequest(context.Background(), testActor(), "LLM-00000001", CreateRequestInput{
		Prompt: "Answer without references",
	})
	if err != nil {
		t.Fatalf("CreateRequest error = %v", err)
	}

	processed, err := service.ProcessNextQueuedRequest(context.Background(), time.Millisecond)
	if err != nil {
		t.Fatalf("ProcessNextQueuedRequest error = %v", err)
	}
	if !processed {
		t.Fatal("expected queued request to be processed")
	}
	if repo.createdRequest.ResponseStatus != ResponseStatusError {
		t.Fatalf("response status = %q, want error", repo.createdRequest.ResponseStatus)
	}
	if repo.createdRequest.ResponseErrorCode != "llm_request_timeout" {
		t.Fatalf("response error code = %q, want timeout", repo.createdRequest.ResponseErrorCode)
	}
}

func TestProcessNextQueuedRequestReturnsFalseWhenQueueIsEmpty(t *testing.T) {
	service := NewService(ServiceDependencies{
		Repository: newFakeRepository(),
		References: newFakeReferences("LLM-00000002"),
		Audit:      &fakeAudit{},
	})

	processed, err := service.ProcessNextQueuedRequest(context.Background(), 0)
	if err != nil {
		t.Fatalf("ProcessNextQueuedRequest error = %v", err)
	}
	if processed {
		t.Fatal("expected empty queue to report no work")
	}
}

func TestProcessNextQueuedRequestFinalizesUnavailableWhenClientDisabled(t *testing.T) {
	repo := newFakeRepository()
	audits := &fakeAudit{}
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: newFakeReferences("LLM-00000002"),
		Audit:      audits,
		Resolver:   &fakeResolver{},
		Config:     RuntimeConfig{Enabled: false, Model: "test-model", MaxTokens: 100},
	})

	_, err := service.CreateRequest(context.Background(), testActor(), "LLM-00000001", CreateRequestInput{
		Prompt: "Answer without provider",
	})
	if err != nil {
		t.Fatalf("CreateRequest error = %v", err)
	}
	processed, err := service.ProcessNextQueuedRequest(context.Background(), 0)
	if err != nil {
		t.Fatalf("ProcessNextQueuedRequest error = %v", err)
	}
	if !processed {
		t.Fatal("expected queued request to be processed")
	}
	if repo.createdRequest.ResponseStatus != ResponseStatusError || repo.createdRequest.ResponseErrorCode != "llm_unavailable" {
		t.Fatalf("completed request = %#v", repo.createdRequest)
	}
	if len(audits.events) == 0 || audits.events[len(audits.events)-1].Result != audit.ResultFailed {
		t.Fatalf("audit events = %#v", audits.events)
	}
}

func TestCreateRequestStopsBeforePersistenceWhenReferenceDenied(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: newFakeReferences("LLM-00000002"),
		Audit:      &fakeAudit{},
		Client:     &fakeClient{},
		Resolver:   &fakeResolver{err: auth.ErrForbidden},
		Config:     RuntimeConfig{Enabled: true, Model: "test-model", MaxTokens: 100},
	})

	_, err := service.CreateRequest(context.Background(), testActor(), "LLM-00000001", CreateRequestInput{
		Prompt:     "Summarize",
		References: []string{"ACC-00000001"},
	})
	if !errors.Is(err, ErrReferenceNotFound) {
		t.Fatalf("CreateRequest error = %v, want ErrReferenceNotFound", err)
	}
	if repo.createdRequest.ID != 0 {
		t.Fatalf("request was persisted despite denied reference: %#v", repo.createdRequest)
	}
}

func TestCreateRequestRecordsDeniedAuditWhenLockedSessionOwnerChanges(t *testing.T) {
	repo := newFakeRepository()
	repo.lockedSession = repo.session
	repo.lockedSession.OwnerID = 2
	audits := &fakeAudit{}
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: newFakeReferences("LLM-00000002"),
		Audit:      audits,
		Resolver:   &fakeResolver{},
		Config:     RuntimeConfig{Enabled: true, Model: "test-model", MaxTokens: 100},
	})

	_, err := service.CreateRequest(context.Background(), testActor(), "LLM-00000001", CreateRequestInput{
		Prompt: "Summarize",
	})
	if !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("CreateRequest error = %v, want forbidden", err)
	}
	if len(audits.standalone) != 1 {
		t.Fatalf("standalone audits = %#v", audits.standalone)
	}
	if audits.standalone[0].Result != audit.ResultDenied || audits.standalone[0].Reason != "not_found" {
		t.Fatalf("standalone audit = %#v", audits.standalone[0])
	}
}

func TestDeleteSessionCascadesRequestsWithAudit(t *testing.T) {
	repo := newFakeRepository()
	repo.createdRequest = Request{ID: 2, OwnerID: 1, SessionID: 1, RefCode: "LLM-00000002", ResponseStatus: ResponseStatusSuccess}
	references := newFakeReferences()
	audits := &fakeAudit{}
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: references,
		Audit:      audits,
	})

	err := service.DeleteSession(context.Background(), testActor(), "LLM-00000001")
	if err != nil {
		t.Fatalf("DeleteSession error = %v", err)
	}
	if !repo.deletedSession {
		t.Fatal("session was not deleted")
	}
	if len(references.deletions) != 2 {
		t.Fatalf("object ref deletions = %#v, want request and session", references.deletions)
	}
	if len(audits.events) != 2 {
		t.Fatalf("audit events = %#v, want request delete and session delete", audits.events)
	}
	if audits.events[0].TargetRefCode != "LLM-00000002" || audits.events[0].Reason != "cascade_llm_session" {
		t.Fatalf("request delete audit = %#v", audits.events[0])
	}
	if audits.events[1].TargetRefCode != "LLM-00000001" {
		t.Fatalf("session delete audit = %#v", audits.events[1])
	}
}

func TestDeleteSessionRecordsFailedStandaloneAudit(t *testing.T) {
	repo := newFakeRepository()
	repo.deleteSessionErr = errors.New("delete failed")
	audits := &fakeAudit{}
	service := NewService(ServiceDependencies{
		Repository: repo,
		References: newFakeReferences(),
		Audit:      audits,
	})

	err := service.DeleteSession(context.Background(), testActor(), "LLM-00000001")
	if err == nil {
		t.Fatal("expected delete error")
	}
	if len(audits.standalone) != 1 {
		t.Fatalf("standalone audits = %#v", audits.standalone)
	}
	if audits.standalone[0].Result != audit.ResultFailed || audits.standalone[0].Reason != "operation_failed" {
		t.Fatalf("standalone audit = %#v", audits.standalone[0])
	}
}

func testActor() auth.Principal {
	return auth.Principal{ID: 1, Username: "alice", Role: auth.RoleUser}
}

type fakeRepository struct {
	session            Session
	lockedSession      Session
	createdRequest     Request
	completedInput     CompleteResponseInput
	listScope          auth.Scope
	findRequestScope   auth.Scope
	listLimit          int
	listOffset         int
	listRequestsLimit  int
	listRequestsOffset int
	startedRequest     bool
	deletedSession     bool
	deleteSessionErr   error
}

func newFakeRepository() *fakeRepository {
	now := time.Unix(100, 0).UTC()
	return &fakeRepository{
		session: Session{
			ID: 1, OwnerID: 1, ObjectRefID: 1, RefCode: "LLM-00000001",
			Title: "Session", Status: SessionStatusActive, CreatedAt: now, UpdatedAt: now,
		},
	}
}

func (r *fakeRepository) ListSessions(_ context.Context, scope auth.Scope, limit int, offset int) (SessionPage, error) {
	r.listScope = scope
	r.listLimit = limit
	r.listOffset = offset
	return SessionPage{Sessions: []Session{r.session}, Limit: limit, Offset: offset}, nil
}

func (r *fakeRepository) CreateSession(_ context.Context, ownerID int64, input CreateSessionInput) (Session, error) {
	r.session = Session{ID: 1, OwnerID: ownerID, Title: input.Title, Status: SessionStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return r.session, nil
}

func (r *fakeRepository) FindSessionByRefCode(_ context.Context, scope auth.Scope, refCode string) (Session, error) {
	if refCode != r.session.RefCode || (!scope.All && scope.OwnerID != r.session.OwnerID) {
		return Session{}, ErrSessionNotFound
	}
	return r.session, nil
}

func (r *fakeRepository) LockSessionByRefCode(ctx context.Context, refCode string) (Session, error) {
	if r.lockedSession.ID != 0 {
		if refCode != r.lockedSession.RefCode {
			return Session{}, ErrSessionNotFound
		}
		return r.lockedSession, nil
	}
	return r.FindSessionByRefCode(ctx, auth.Scope{All: true}, refCode)
}

func (r *fakeRepository) DeleteSession(_ context.Context, ownerID int64, sessionID int64) error {
	if r.deleteSessionErr != nil {
		return r.deleteSessionErr
	}
	if ownerID != r.session.OwnerID || sessionID != r.session.ID {
		return ErrSessionNotFound
	}
	r.deletedSession = true
	return nil
}

func (r *fakeRepository) CreateRequest(_ context.Context, ownerID int64, sessionID int64, input PersistedRequestInput) (Request, error) {
	now := time.Now()
	r.createdRequest = Request{
		ID: 2, OwnerID: ownerID, SessionID: sessionID, ActorUserID: input.ActorUserID,
		Prompt: input.Prompt, Model: input.Model, MaxTokens: input.MaxTokens,
		ContextJSON: input.ContextJSON, RequestJSON: input.RequestJSON, ResponseStatus: ResponseStatusQueued,
		ResponseJSON: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	return r.createdRequest, nil
}

func (r *fakeRepository) FindRequestByRefCode(_ context.Context, scope auth.Scope, refCode string) (Request, error) {
	r.findRequestScope = scope
	request := r.createdRequest
	if request.RefCode == "" {
		request.RefCode = "LLM-00000002"
	}
	if refCode != request.RefCode || (!scope.All && scope.OwnerID != request.OwnerID) {
		return Request{}, ErrRequestNotFound
	}
	return request, nil
}

func (r *fakeRepository) InsertRequestReference(_ context.Context, requestID int64, reference ResolvedReference) (RequestReference, error) {
	return RequestReference{
		ID: 4, RequestID: requestID, ObjectRefID: reference.ObjectRefID, RefCode: reference.RefCode,
		Module: reference.Module, ObjectType: reference.ObjectType, Title: reference.Title,
		Status: reference.Status, Tags: reference.Tags, PayloadJSON: reference.PayloadJSON, CreatedAt: time.Now(),
	}, nil
}

func (r *fakeRepository) ListRequests(_ context.Context, _ int64, _ int64, limit int, offset int) ([]Request, error) {
	r.listRequestsLimit = limit
	r.listRequestsOffset = offset
	if r.createdRequest.ID == 0 {
		return nil, nil
	}
	return []Request{r.createdRequest}, nil
}

func (r *fakeRepository) ListRequestDeletionTargets(_ context.Context, ownerID int64, sessionID int64) ([]RequestDeletionTarget, error) {
	if r.createdRequest.ID == 0 || r.createdRequest.OwnerID != ownerID || r.createdRequest.SessionID != sessionID {
		return nil, nil
	}
	return []RequestDeletionTarget{{ID: r.createdRequest.ID, RefCode: r.createdRequest.RefCode}}, nil
}

func (r *fakeRepository) ClaimNextQueuedRequest(_ context.Context) (Request, error) {
	if r.createdRequest.ID == 0 || r.createdRequest.ResponseStatus != ResponseStatusQueued {
		return Request{}, ErrNoQueuedRequest
	}
	r.startedRequest = true
	r.createdRequest.ResponseStatus = ResponseStatusRunning
	r.createdRequest.RefCode = "LLM-00000002"
	r.createdRequest.UpdatedAt = time.Now()
	return r.createdRequest, nil
}

func (r *fakeRepository) CompleteRequestResponse(_ context.Context, ownerID int64, requestID int64, input CompleteResponseInput) (Response, error) {
	r.completedInput = input
	if requestID != r.createdRequest.ID || ownerID != r.createdRequest.OwnerID {
		return Response{}, ErrRequestNotFound
	}
	if r.createdRequest.ResponseStatus != ResponseStatusRunning {
		return Response{}, ErrRequestAlreadyFinal
	}
	r.createdRequest.ResponseStatus = input.Status
	r.createdRequest.ResponseContent = input.Content
	r.createdRequest.ResponseErrorCode = input.ErrorCode
	r.createdRequest.ResponseErrorMessage = input.ErrorMessage
	r.createdRequest.ResponseJSON = input.ResponseJSON
	now := time.Now()
	r.createdRequest.UpdatedAt = now
	r.createdRequest.CompletedAt = &now
	return Response{
		OwnerID:      r.createdRequest.OwnerID,
		SessionID:    r.createdRequest.SessionID,
		RequestID:    r.createdRequest.ID,
		Status:       r.createdRequest.ResponseStatus,
		Content:      r.createdRequest.ResponseContent,
		ErrorCode:    r.createdRequest.ResponseErrorCode,
		ErrorMessage: r.createdRequest.ResponseErrorMessage,
		ResponseJSON: r.createdRequest.ResponseJSON,
		CreatedAt:    r.createdRequest.CreatedAt,
		UpdatedAt:    r.createdRequest.UpdatedAt,
		CompletedAt:  r.createdRequest.CompletedAt,
	}, nil
}

type fakeReferences struct {
	next          []string
	id            int64
	registrations []ref.Registration
	deletions     []ref.ProjectionUpdate
}

func newFakeReferences(codes ...string) *fakeReferences {
	return &fakeReferences{next: codes, id: 20}
}

func (r *fakeReferences) ClaimCode(_ context.Context, _ ref.ObjectType) (string, error) {
	code := r.next[0]
	r.next = r.next[1:]
	return code, nil
}

func (r *fakeReferences) Register(_ context.Context, registration ref.Registration) (ref.ObjectRef, error) {
	r.id++
	r.registrations = append(r.registrations, registration)
	return ref.ObjectRef{
		ID: r.id, OwnerID: registration.OwnerID, RefCode: registration.RefCode,
		ObjectType: registration.ObjectType, ObjectID: registration.ObjectID,
		Title: registration.Title, Tags: registration.Tags, Status: registration.Status,
	}, nil
}

func (r *fakeReferences) UpdateProjection(_ context.Context, update ref.ProjectionUpdate) (ref.ObjectRef, error) {
	return ref.ObjectRef{
		ID: r.id, OwnerID: update.OwnerID, ObjectType: update.ObjectType,
		ObjectID: update.ObjectID, Title: update.Title, Tags: update.Tags, Status: update.Status,
	}, nil
}

func (r *fakeReferences) Delete(_ context.Context, ownerID int64, objectType ref.ObjectType, objectID int64) error {
	r.deletions = append(r.deletions, ref.ProjectionUpdate{
		OwnerID: ownerID, ObjectType: objectType, ObjectID: objectID,
		Title: "deleted", Status: "deleted",
	})
	return nil
}

type fakeAudit struct {
	events     []audit.Event
	standalone []audit.Event
}

func (a *fakeAudit) Record(_ context.Context, event audit.Event) (audit.Event, error) {
	a.events = append(a.events, event)
	return event, nil
}

func (a *fakeAudit) RecordStandalone(_ context.Context, event audit.Event) error {
	a.standalone = append(a.standalone, event)
	return nil
}

type fakeClient struct {
	result         ClientResult
	err            error
	calls          int
	waitForContext bool
}

func (c *fakeClient) Complete(ctx context.Context, _ json.RawMessage) (ClientResult, error) {
	c.calls++
	if c.waitForContext {
		<-ctx.Done()
		return ClientResult{}, ctx.Err()
	}
	return c.result, c.err
}

type fakeResolver struct {
	reference ResolvedReference
	err       error
}

func (r *fakeResolver) Resolve(_ context.Context, _ auth.Principal, refCode string) (ResolvedReference, error) {
	if r.err != nil {
		return ResolvedReference{}, r.err
	}
	if r.reference.RefCode == "" {
		return ResolvedReference{
			RefCode: refCode, Module: "accounting", ObjectType: "account", Title: refCode, Status: "active",
			Tags: []string{}, PayloadJSON: json.RawMessage(`{}`),
		}, nil
	}
	return r.reference, nil
}
