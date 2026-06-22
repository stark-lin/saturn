// This file tests LLM business reference resolution into authorized context payloads.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/accounting"
	"github.com/stark-lin/go-proj/internal/calendar"
	"github.com/stark-lin/go-proj/internal/files"
	"github.com/stark-lin/go-proj/internal/notes"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

func TestBusinessReferenceResolverResolvesSupportedObjectPayloads(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	tests := []struct {
		name       string
		objectType ref.ObjectType
		refCode    string
		wantModule string
		wantTags   []string
	}{
		{name: "account", objectType: ref.ObjectTypeAccount, refCode: "ACC-00000001", wantModule: "accounting", wantTags: []string{"cash"}},
		{name: "transaction", objectType: ref.ObjectTypeTransaction, refCode: "TRN-00000001", wantModule: "accounting", wantTags: []string{"tax"}},
		{name: "note", objectType: ref.ObjectTypeNote, refCode: "NTE-00000001", wantModule: "notes", wantTags: []string{"draft"}},
		{name: "file collection", objectType: ref.ObjectTypeFileCollection, refCode: "FCL-00000001", wantModule: "files", wantTags: []string{"docs"}},
		{name: "file", objectType: ref.ObjectTypeFile, refCode: "FIL-00000001", wantModule: "files", wantTags: []string{"receipt"}},
		{name: "event aggregate", objectType: ref.ObjectTypeEventAggregate, refCode: "EAG-00000001", wantModule: "calendar", wantTags: []string{"work"}},
		{name: "event", objectType: ref.ObjectTypeEvent, refCode: "EVT-00000001", wantModule: "calendar", wantTags: []string{"meeting"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := newReferenceResolverFixture(now, ref.ObjectRef{
				ID: 10, RefCode: tt.refCode, ObjectType: tt.objectType, Title: "Object", Status: "active",
			})

			resolved, err := resolver.Resolve(context.Background(), testActor(), tt.refCode)
			if err != nil {
				t.Fatalf("Resolve error = %v", err)
			}
			if resolved.RefCode != tt.refCode || resolved.Module != tt.wantModule || resolved.ObjectType != string(tt.objectType) {
				t.Fatalf("resolved = %#v", resolved)
			}
			if len(resolved.Tags) != len(tt.wantTags) || resolved.Tags[0] != tt.wantTags[0] {
				t.Fatalf("tags = %#v, want %#v", resolved.Tags, tt.wantTags)
			}
			var payload map[string]any
			if err := json.Unmarshal(resolved.PayloadJSON, &payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload["ref_code"] != tt.refCode {
				t.Fatalf("payload = %#v", payload)
			}
			if tt.objectType == ref.ObjectTypeEventAggregate {
				events, ok := payload["events"].([]any)
				if !ok || len(events) != 1 {
					t.Fatalf("aggregate payload = %#v", payload)
				}
			}
		})
	}
}

func TestBusinessReferenceResolverPropagatesDependencyAndAccessErrors(t *testing.T) {
	if _, err := (*BusinessReferenceResolver)(nil).Resolve(context.Background(), testActor(), "ACC-00000001"); !errors.Is(err, ErrDependencyUnavailable) {
		t.Fatalf("nil resolver error = %v, want dependency unavailable", err)
	}

	objectErr := errors.New("object lookup failed")
	resolver := NewBusinessReferenceResolver(&fakeObjectResolver{err: objectErr}, nil, nil, nil, nil)
	if _, err := resolver.Resolve(context.Background(), testActor(), "ACC-00000001"); !errors.Is(err, objectErr) {
		t.Fatalf("object lookup error = %v, want %v", err, objectErr)
	}

	resolver = NewBusinessReferenceResolver(&fakeObjectResolver{object: ref.ObjectRef{RefCode: "ACC-00000001", ObjectType: ref.ObjectTypeAccount}}, nil, nil, nil, nil)
	if _, err := resolver.Resolve(context.Background(), testActor(), "ACC-00000001"); !errors.Is(err, ErrDependencyUnavailable) {
		t.Fatalf("missing accounting error = %v, want dependency unavailable", err)
	}

	readErr := auth.ErrForbidden
	resolver = NewBusinessReferenceResolver(
		&fakeObjectResolver{object: ref.ObjectRef{RefCode: "ACC-00000001", ObjectType: ref.ObjectTypeAccount}},
		&fakeAccountingReader{err: readErr}, nil, nil, nil,
	)
	if _, err := resolver.Resolve(context.Background(), testActor(), "ACC-00000001"); !errors.Is(err, readErr) {
		t.Fatalf("reader error = %v, want %v", err, readErr)
	}
}

func TestPayloadForObjectRejectsUnsupportedObjectType(t *testing.T) {
	resolver := NewBusinessReferenceResolver(nil, nil, nil, nil, nil)
	_, _, err := resolver.payloadForObject(context.Background(), testActor(), ref.ObjectRef{ObjectType: ref.ObjectTypeLLMSession})
	if err == nil {
		t.Fatal("expected unsupported object type error")
	}
}

func newReferenceResolverFixture(now time.Time, object ref.ObjectRef) *BusinessReferenceResolver {
	return NewBusinessReferenceResolver(
		&fakeObjectResolver{object: object},
		&fakeAccountingReader{
			account: accounting.Account{
				RefCode: object.RefCode, Name: "Cash", Type: accounting.AccountTypeCash, Currency: "USD",
				OpeningBalanceCents: 100, BalanceCents: 125, Status: accounting.AccountStatusActive,
				Tags: []string{"cash"}, CreatedAt: now, UpdatedAt: now,
			},
			transaction: accounting.Transaction{
				RefCode: object.RefCode, AccountRefCode: "ACC-00000001", OccurredOn: now,
				Kind: accounting.TransactionKindExpense, AmountCents: 50, Title: "Lunch",
				Status: accounting.TransactionStatusPosted, Tags: []string{"tax"}, CreatedAt: now, UpdatedAt: now,
			},
		},
		&fakeNotesReader{note: notes.Note{
			RefCode: object.RefCode, Title: "Note", Markdown: "Body", Status: notes.NoteDraft,
			Tags: []string{"draft"}, CreatedAt: now, UpdatedAt: now,
		}},
		&fakeFilesReader{
			collection: files.Collection{
				RefCode: object.RefCode, Name: "Docs", Description: "Reference docs", Status: files.CollectionStatusActive,
				Tags: []string{"docs"}, CreatedAt: now, UpdatedAt: now,
			},
			file: files.File{
				RefCode: object.RefCode, CollectionRefCode: "FCL-00000001", OriginalName: "receipt.pdf",
				MimeType: "application/pdf", SizeBytes: 128, SHA256: "sha", BLAKE3: "blake3",
				Status: files.FileStatusActive, Tags: []string{"receipt"}, CreatedAt: now, UpdatedAt: now,
			},
		},
		&fakeCalendarReader{
			detail: calendar.EventAggregateDetail{
				Aggregate: calendar.EventAggregate{
					RefCode: object.RefCode, Metadata: calendar.EventAggregateMetadata{Title: "Planning"},
					Tags: []string{"work"}, CreatedAt: now,
				},
				Events: []calendar.Event{{
					RefCode: "EVT-00000001", AggregateRefCode: object.RefCode, StartsAt: now,
					DurationMinutes: 30, Metadata: calendar.EventMetadata{Title: "Planning"},
					Status: calendar.EventStatusScheduled, Tags: []string{"meeting"}, CreatedAt: now, UpdatedAt: now,
				}},
			},
			event: calendar.Event{
				RefCode: object.RefCode, AggregateRefCode: "EAG-00000001", StartsAt: now,
				DurationMinutes: 30, Metadata: calendar.EventMetadata{Title: "Planning"},
				Status: calendar.EventStatusScheduled, Tags: []string{"meeting"}, CreatedAt: now, UpdatedAt: now,
			},
		},
	)
}

type fakeObjectResolver struct {
	object ref.ObjectRef
	err    error
}

func (r *fakeObjectResolver) Resolve(_ context.Context, _ string) (ref.ObjectRef, error) {
	return r.object, r.err
}

type fakeAccountingReader struct {
	account     accounting.Account
	transaction accounting.Transaction
	err         error
}

func (r *fakeAccountingReader) GetAccount(_ context.Context, _ auth.Principal, _ string) (accounting.Account, error) {
	return r.account, r.err
}

func (r *fakeAccountingReader) GetTransaction(_ context.Context, _ auth.Principal, _ string) (accounting.Transaction, error) {
	return r.transaction, r.err
}

type fakeNotesReader struct {
	note notes.Note
	err  error
}

func (r *fakeNotesReader) GetNote(_ context.Context, _ auth.Principal, _ string) (notes.Note, error) {
	return r.note, r.err
}

type fakeFilesReader struct {
	collection files.Collection
	file       files.File
	err        error
}

func (r *fakeFilesReader) GetCollection(_ context.Context, _ auth.Principal, _ string) (files.Collection, error) {
	return r.collection, r.err
}

func (r *fakeFilesReader) GetFile(_ context.Context, _ auth.Principal, _ string) (files.File, error) {
	return r.file, r.err
}

type fakeCalendarReader struct {
	detail calendar.EventAggregateDetail
	event  calendar.Event
	err    error
}

func (r *fakeCalendarReader) GetEventAggregate(_ context.Context, _ auth.Principal, _ string) (calendar.EventAggregateDetail, error) {
	return r.detail, r.err
}

func (r *fakeCalendarReader) GetEvent(_ context.Context, _ auth.Principal, _ string) (calendar.Event, error) {
	return r.event, r.err
}
