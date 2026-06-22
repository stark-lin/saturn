// This file tests LLM SQL repository helper boundaries without a database.
package llm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

func TestSQLRepositoryRejectsMissingDatabase(t *testing.T) {
	repo := NewSQLRepository(nil)
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "ListSessions", call: func() error {
			_, err := repo.ListSessions(ctx, auth.Scope{All: true}, 10, 0)
			return err
		}},
		{name: "CreateSession", call: func() error {
			_, err := repo.CreateSession(ctx, 1, CreateSessionInput{Title: "Session"})
			return err
		}},
		{name: "FindSessionByRefCode", call: func() error {
			_, err := repo.FindSessionByRefCode(ctx, auth.Scope{All: true}, "LLM-00000001")
			return err
		}},
		{name: "LockSessionByRefCode", call: func() error {
			_, err := repo.LockSessionByRefCode(ctx, "LLM-00000001")
			return err
		}},
		{name: "DeleteSession", call: func() error {
			return repo.DeleteSession(ctx, 1, 1)
		}},
		{name: "CreateRequest", call: func() error {
			_, err := repo.CreateRequest(ctx, 1, 1, PersistedRequestInput{Prompt: "Prompt"})
			return err
		}},
		{name: "FindRequestByRefCode", call: func() error {
			_, err := repo.FindRequestByRefCode(ctx, auth.Scope{All: true}, "LLM-00000002")
			return err
		}},
		{name: "InsertRequestReference", call: func() error {
			_, err := repo.InsertRequestReference(ctx, 2, ResolvedReference{RefCode: "ACC-00000001"})
			return err
		}},
		{name: "ListRequests", call: func() error {
			_, err := repo.ListRequests(ctx, 1, 1, 10, 0)
			return err
		}},
		{name: "ListRequestDeletionTargets", call: func() error {
			_, err := repo.ListRequestDeletionTargets(ctx, 1, 1)
			return err
		}},
		{name: "ClaimNextQueuedRequest", call: func() error {
			_, err := repo.ClaimNextQueuedRequest(ctx)
			return err
		}},
		{name: "CompleteRequestResponse", call: func() error {
			_, err := repo.CompleteRequestResponse(ctx, 1, 2, CompleteResponseInput{Status: ResponseStatusSuccess})
			return err
		}},
		{name: "listRequestReferences", call: func() error {
			_, err := repo.listRequestReferences(ctx, 2)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("expected missing database error")
			}
		})
	}
}

func TestSQLRepositoryJSONAndTagHelpers(t *testing.T) {
	if got := jsonArgument(nil); got != "{}" {
		t.Fatalf("empty json argument = %q, want {}", got)
	}
	if got := jsonArgument(json.RawMessage(`{"ok":true}`)); got != `{"ok":true}` {
		t.Fatalf("json argument = %q", got)
	}
	if tags := tagsFromPayloadJSON(nil); len(tags) != 0 {
		t.Fatalf("empty tags = %#v", tags)
	}
	if tags := tagsFromPayloadJSON(json.RawMessage(`{"tags":["a","b"]}`)); len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Fatalf("tags = %#v", tags)
	}
	if tags := tagsFromPayloadJSON(json.RawMessage(`{`)); len(tags) != 0 {
		t.Fatalf("invalid tags = %#v", tags)
	}
	if tags := nonNilTags(nil); tags == nil || len(tags) != 0 {
		t.Fatalf("nonNilTags(nil) = %#v", tags)
	}
}

func TestSQLRepositoryScannersMapRows(t *testing.T) {
	createdAt := time.Unix(100, 0).UTC()
	updatedAt := time.Unix(101, 0).UTC()
	completedAt := time.Unix(102, 0).UTC()

	session, err := scanSession(fakeRow{values: []any{
		int64(1), int64(7), int64(20), "LLM-00000001", []byte(`{"planning","llm"}`),
		"Planning", string(SessionStatusActive), createdAt, updatedAt,
	}})
	if err != nil {
		t.Fatalf("scanSession error = %v", err)
	}
	if session.RefCode != "LLM-00000001" || len(session.Tags) != 2 || session.Tags[0] != "planning" {
		t.Fatalf("session = %#v", session)
	}

	response, err := scanResponseFromRequest(fakeRow{values: []any{
		int64(7), int64(1), int64(2), string(ResponseStatusSuccess), "answer", "", "",
		json.RawMessage(`{"ok":true}`), createdAt, updatedAt, completedAt,
	}})
	if err != nil {
		t.Fatalf("scanResponseFromRequest error = %v", err)
	}
	if response.RequestID != 2 || response.Status != ResponseStatusSuccess || response.CompletedAt == nil {
		t.Fatalf("response = %#v", response)
	}

	reference, err := scanRequestReference(fakeRow{values: []any{
		int64(4), int64(2), int64(10), "ACC-00000001",
		"accounting", "account", "Cash", "active", json.RawMessage(`{"tags":["finance"]}`), createdAt,
	}})
	if err != nil {
		t.Fatalf("scanRequestReference error = %v", err)
	}
	if reference.ObjectRefID != 10 || len(reference.Tags) != 1 || reference.Tags[0] != "finance" {
		t.Fatalf("reference = %#v", reference)
	}

	request, err := scanRequestWithRef(fakeRow{values: []any{
		int64(2), int64(7), int64(1), int64(7), int64(21), "LLM-00000002", []byte(`{"review"}`),
		"Prompt", "test-model", 100, json.RawMessage(`{"references":[]}`), json.RawMessage(`{"messages":[]}`),
		string(ResponseStatusSuccess), "answer", "", "", json.RawMessage(`{"ok":true}`),
		createdAt, updatedAt, completedAt,
	}})
	if err != nil {
		t.Fatalf("scanRequestWithRef error = %v", err)
	}
	if request.RefCode != "LLM-00000002" || request.ResponseContent != "answer" || request.CompletedAt == nil ||
		len(request.Tags) != 1 || request.Tags[0] != "review" {
		t.Fatalf("request = %#v", request)
	}
}

func TestSQLRepositoryScannersReturnRowErrors(t *testing.T) {
	rowErr := errors.New("scan failed")
	if _, err := scanSession(fakeRow{err: rowErr}); !errors.Is(err, rowErr) {
		t.Fatalf("scanSession error = %v, want %v", err, rowErr)
	}
	if _, err := scanResponseFromRequest(fakeRow{err: rowErr}); !errors.Is(err, rowErr) {
		t.Fatalf("scanResponseFromRequest error = %v, want %v", err, rowErr)
	}
	if _, err := scanRequestReference(fakeRow{err: rowErr}); !errors.Is(err, rowErr) {
		t.Fatalf("scanRequestReference error = %v, want %v", err, rowErr)
	}
	if _, err := scanRequestWithRef(fakeRow{err: rowErr}); !errors.Is(err, rowErr) {
		t.Fatalf("scanRequestWithRef error = %v, want %v", err, rowErr)
	}
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		assignScanValue(dest[i], r.values[i])
	}
	return nil
}

func assignScanValue(dest any, value any) {
	if scanner, ok := dest.(interface{ Scan(any) error }); ok {
		_ = scanner.Scan(value)
		return
	}
	switch target := dest.(type) {
	case *int:
		*target = value.(int)
	case *int64:
		*target = value.(int64)
	case *string:
		*target = value.(string)
	case *SessionStatus:
		*target = SessionStatus(value.(string))
	case *ResponseStatus:
		*target = ResponseStatus(value.(string))
	case *json.RawMessage:
		*target = value.(json.RawMessage)
	case *time.Time:
		*target = value.(time.Time)
	case *sql.NullInt64:
		*target = value.(sql.NullInt64)
	case *sql.NullTime:
		*target = value.(sql.NullTime)
	}
}
