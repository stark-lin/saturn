// This file tests append-only audit validation and superuser query access.
package audit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/auth"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
	"github.com/stark-lin/go-proj/internal/platform/httpx"
)

func TestServiceRejectsUserReadAuditAndInvalidActorIdentity(t *testing.T) {
	service := NewService(&fakeRepository{}, platformdb.NoopTransactionRunner{})
	for _, event := range []Event{
		{ActorType: ActorTypeUser, ActorUserID: 7, Action: ActionRead, TargetRefCode: "NTE-00000001", Result: ResultSuccess},
		{ActorType: ActorTypeUser, Action: ActionUpdate, TargetRefCode: "NTE-00000001", Result: ResultSuccess},
		{ActorType: ActorTypeLLM, ActorUserID: 7, Action: ActionRead, TargetRefCode: "NTE-00000001", Result: ResultSuccess},
	} {
		if _, err := service.Record(context.Background(), event); !errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("event %#v error = %v, want invalid event", event, err)
		}
	}
}

func TestServiceAcceptsLLMReadAudit(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, platformdb.NoopTransactionRunner{})
	event := Event{ActorType: ActorTypeLLM, Action: ActionRead, TargetRefCode: "NTE-00000001", Result: ResultSuccess}

	if _, err := service.Record(context.Background(), event); err != nil {
		t.Fatalf("record LLM read: %v", err)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].SourceIP != "127.0.0.1" {
		t.Fatalf("inserted audit = %#v", repo.inserted)
	}
}

func TestServiceRecordEnrichesSourceAndNormalizesStableFields(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, platformdb.NoopTransactionRunner{})
	var ctx context.Context
	httpx.CaptureRequestSource(nil, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		ctx = request.Context()
	})).ServeHTTP(httptest.NewRecorder(), requestWithSource("203.0.113.10:1234", "saturn-test"))

	createdAt := time.Unix(100, 0).UTC()
	event, err := service.Record(ctx, Event{
		ActorType: ActorTypeUser, ActorUserID: 7, Action: ActionUpdate,
		TargetRefCode: " nte-00000001 ", Result: ResultFailed, Reason: " validation_failed ",
		CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatalf("Record error = %v", err)
	}
	if event.TargetRefCode != "NTE-00000001" || event.Reason != "validation_failed" {
		t.Fatalf("normalized event = %#v", event)
	}
	if event.SourceIP != "203.0.113.10" || event.UserAgent != "saturn-test" {
		t.Fatalf("source fields = %#v", event)
	}
	if !event.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at = %v, want %v", event.CreatedAt, createdAt)
	}
}

func TestServiceRecordRejectsInvalidActionResultTargetAndSource(t *testing.T) {
	service := NewService(&fakeRepository{}, platformdb.NoopTransactionRunner{})
	tests := []Event{
		{ActorType: ActorTypeUser, ActorUserID: 7, Action: Action("BAD"), TargetRefCode: "NTE-00000001", Result: ResultSuccess},
		{ActorType: ActorTypeUser, ActorUserID: 7, Action: ActionUpdate, TargetRefCode: "NTE-00000001", Result: Result("BAD")},
		{ActorType: ActorTypeUser, ActorUserID: 7, Action: ActionUpdate, TargetRefCode: "bad", Result: ResultSuccess},
		{ActorType: ActorTypeAnonymous, Action: ActionLogin, TargetRefCode: "NTE-00000001", Result: ResultSuccess},
		{ActorType: ActorTypeSystem, Action: ActionExport, TargetRefCode: SystemTargetRefCode, Result: ResultSuccess, SourceIP: " "},
	}

	for _, event := range tests {
		if _, err := service.Record(context.Background(), event); !errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("event %#v error = %v, want invalid event", event, err)
		}
	}
}

func TestServiceRecordStandaloneUsesAuditTransaction(t *testing.T) {
	repo := &fakeRepository{}
	runner := &fakeTransactionRunner{}
	service := NewService(repo, runner)

	err := service.RecordStandalone(context.Background(), Event{
		ActorType: ActorTypeSystem, Action: ActionExport, TargetRefCode: SystemTargetRefCode, Result: ResultSuccess,
	})
	if err != nil {
		t.Fatalf("RecordStandalone error = %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("transaction calls = %d, want 1", runner.calls)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("inserted events = %#v", repo.inserted)
	}
}

func TestServiceRecordStandaloneRequiresTransactionRunner(t *testing.T) {
	service := NewService(&fakeRepository{}, nil)

	err := service.RecordStandalone(context.Background(), Event{})
	if err == nil {
		t.Fatal("expected transaction runner error")
	}
}

func TestServiceRecordAuthenticationBuildsAuthenticationEvents(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, platformdb.NoopTransactionRunner{})

	if err := service.RecordAuthentication(context.Background(), 0, string(ActionLogin), string(ResultDenied), "bad_token"); err != nil {
		t.Fatalf("anonymous authentication audit: %v", err)
	}
	if err := service.RecordAuthentication(context.Background(), 7, string(ActionLogout), string(ResultSuccess), ""); err != nil {
		t.Fatalf("user authentication audit: %v", err)
	}
	if repo.inserted[0].ActorType != ActorTypeAnonymous || repo.inserted[0].TargetRefCode != SystemTargetRefCode {
		t.Fatalf("anonymous auth event = %#v", repo.inserted[0])
	}
	if repo.inserted[1].ActorType != ActorTypeUser || repo.inserted[1].ActorUserID != 7 {
		t.Fatalf("user auth event = %#v", repo.inserted[1])
	}
}

func TestServiceListsOnlyForSuperuser(t *testing.T) {
	service := NewService(&fakeRepository{}, platformdb.NoopTransactionRunner{})
	if _, err := service.List(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, Query{}); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("ordinary user list error = %v, want forbidden", err)
	}
	if _, err := service.List(context.Background(), auth.Principal{ID: 1, Role: auth.RoleSuperuser}, Query{}); err != nil {
		t.Fatalf("superuser list: %v", err)
	}
}

func TestServiceListNormalizesAndValidatesQuery(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, platformdb.NoopTransactionRunner{})
	actor := auth.Principal{ID: 1, Role: auth.RoleSuperuser}

	if _, err := service.List(context.Background(), actor, Query{
		TargetRefCode: " nte-00000001 ",
		Action:        ActionLogin,
		Result:        ResultSuccess,
		Offset:        5,
	}); err != nil {
		t.Fatalf("List error = %v", err)
	}
	if repo.query.TargetRefCode != "NTE-00000001" || repo.query.Limit != DefaultLimit || repo.query.Offset != 5 {
		t.Fatalf("query = %#v", repo.query)
	}

	invalidQueries := []Query{
		{Limit: -1},
		{Limit: MaxLimit + 1},
		{Offset: -1},
		{TargetRefCode: "bad"},
		{Action: Action("BAD")},
		{Result: Result("BAD")},
	}
	for _, query := range invalidQueries {
		if _, err := service.List(context.Background(), actor, query); !errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("query %#v error = %v, want invalid event", query, err)
		}
	}
}

type fakeRepository struct {
	inserted []Event
	query    Query
}

func (r *fakeRepository) Insert(_ context.Context, event Event) (Event, error) {
	r.inserted = append(r.inserted, event)
	return event, nil
}

func (r *fakeRepository) List(_ context.Context, query Query) ([]Event, error) {
	r.query = query
	return []Event{}, nil
}

type fakeTransactionRunner struct {
	calls int
}

func (r *fakeTransactionRunner) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	r.calls++
	return fn(ctx)
}

func requestWithSource(remoteAddr string, userAgent string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/audit-source", nil)
	request.RemoteAddr = remoteAddr
	request.Header.Set("User-Agent", userAgent)
	return request
}
