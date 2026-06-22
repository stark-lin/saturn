// This file resolves authorized cross-module references for LLM requests.
package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stark-lin/go-proj/internal/accounting"
	"github.com/stark-lin/go-proj/internal/calendar"
	"github.com/stark-lin/go-proj/internal/files"
	"github.com/stark-lin/go-proj/internal/notes"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

type ObjectResolver interface {
	Resolve(ctx context.Context, code string) (ref.ObjectRef, error)
}

type AccountingReader interface {
	GetAccount(ctx context.Context, actor auth.Principal, refCode string) (accounting.Account, error)
	GetTransaction(ctx context.Context, actor auth.Principal, refCode string) (accounting.Transaction, error)
}

type NotesReader interface {
	GetNote(ctx context.Context, actor auth.Principal, refCode string) (notes.Note, error)
}

type FilesReader interface {
	GetCollection(ctx context.Context, actor auth.Principal, refCode string) (files.Collection, error)
	GetFile(ctx context.Context, actor auth.Principal, refCode string) (files.File, error)
}

type CalendarReader interface {
	GetEventAggregate(ctx context.Context, actor auth.Principal, refCode string) (calendar.EventAggregateDetail, error)
	GetEvent(ctx context.Context, actor auth.Principal, refCode string) (calendar.Event, error)
}

type BusinessReferenceResolver struct {
	objects    ObjectResolver
	accounting AccountingReader
	notes      NotesReader
	files      FilesReader
	calendar   CalendarReader
}

func NewBusinessReferenceResolver(
	objects ObjectResolver,
	accountingReader AccountingReader,
	notesReader NotesReader,
	filesReader FilesReader,
	calendarReader CalendarReader,
) *BusinessReferenceResolver {
	return &BusinessReferenceResolver{
		objects: objects, accounting: accountingReader, notes: notesReader, files: filesReader, calendar: calendarReader,
	}
}

func (r *BusinessReferenceResolver) Resolve(ctx context.Context, actor auth.Principal, refCode string) (ResolvedReference, error) {
	if r == nil || r.objects == nil {
		return ResolvedReference{}, ErrDependencyUnavailable
	}
	object, err := r.objects.Resolve(ctx, refCode)
	if err != nil {
		return ResolvedReference{}, err
	}
	module, err := ref.ModuleForObjectType(object.ObjectType)
	if err != nil {
		return ResolvedReference{}, err
	}
	payload, tags, err := r.payloadForObject(ctx, actor, object)
	if err != nil {
		return ResolvedReference{}, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return ResolvedReference{}, err
	}
	return ResolvedReference{
		ObjectRefID: object.ID,
		RefCode:     object.RefCode,
		Module:      string(module),
		ObjectType:  string(object.ObjectType),
		Title:       object.Title,
		Status:      object.Status,
		Tags:        tags,
		PayloadJSON: json.RawMessage(payloadJSON),
	}, nil
}

func (r *BusinessReferenceResolver) payloadForObject(ctx context.Context, actor auth.Principal, object ref.ObjectRef) (any, []string, error) {
	switch object.ObjectType {
	case ref.ObjectTypeAccount:
		if r.accounting == nil {
			return nil, nil, ErrDependencyUnavailable
		}
		account, err := r.accounting.GetAccount(ctx, actor, object.RefCode)
		if err != nil {
			return nil, nil, err
		}
		return map[string]any{
			"ref_code": account.RefCode, "name": account.Name, "type": account.Type, "currency": account.Currency,
			"opening_balance_cents": account.OpeningBalanceCents, "balance_cents": account.BalanceCents,
			"status": account.Status, "tags": account.Tags, "created_at": account.CreatedAt, "updated_at": account.UpdatedAt,
		}, account.Tags, nil
	case ref.ObjectTypeTransaction:
		if r.accounting == nil {
			return nil, nil, ErrDependencyUnavailable
		}
		transaction, err := r.accounting.GetTransaction(ctx, actor, object.RefCode)
		if err != nil {
			return nil, nil, err
		}
		return map[string]any{
			"ref_code": transaction.RefCode, "account_ref_code": transaction.AccountRefCode,
			"occurred_on": transaction.OccurredOn, "kind": transaction.Kind, "amount_cents": transaction.AmountCents,
			"title": transaction.Title, "note": transaction.Note, "status": transaction.Status,
			"tags": transaction.Tags, "created_at": transaction.CreatedAt, "updated_at": transaction.UpdatedAt,
		}, transaction.Tags, nil
	case ref.ObjectTypeNote:
		if r.notes == nil {
			return nil, nil, ErrDependencyUnavailable
		}
		note, err := r.notes.GetNote(ctx, actor, object.RefCode)
		if err != nil {
			return nil, nil, err
		}
		return map[string]any{
			"ref_code": note.RefCode, "title": note.Title, "markdown": note.Markdown,
			"status": note.Status, "tags": note.Tags, "created_at": note.CreatedAt, "updated_at": note.UpdatedAt,
		}, note.Tags, nil
	case ref.ObjectTypeFileCollection:
		if r.files == nil {
			return nil, nil, ErrDependencyUnavailable
		}
		collection, err := r.files.GetCollection(ctx, actor, object.RefCode)
		if err != nil {
			return nil, nil, err
		}
		return map[string]any{
			"ref_code": collection.RefCode, "name": collection.Name, "description": collection.Description,
			"status": collection.Status, "tags": collection.Tags, "created_at": collection.CreatedAt, "updated_at": collection.UpdatedAt,
		}, collection.Tags, nil
	case ref.ObjectTypeFile:
		if r.files == nil {
			return nil, nil, ErrDependencyUnavailable
		}
		file, err := r.files.GetFile(ctx, actor, object.RefCode)
		if err != nil {
			return nil, nil, err
		}
		return map[string]any{
			"ref_code": file.RefCode, "collection_ref_code": file.CollectionRefCode,
			"original_name": file.OriginalName, "mime_type": file.MimeType, "size_bytes": file.SizeBytes,
			"sha256": file.SHA256, "blake3": file.BLAKE3, "status": file.Status,
			"tags": file.Tags, "created_at": file.CreatedAt, "updated_at": file.UpdatedAt,
		}, file.Tags, nil
	case ref.ObjectTypeEventAggregate:
		if r.calendar == nil {
			return nil, nil, ErrDependencyUnavailable
		}
		detail, err := r.calendar.GetEventAggregate(ctx, actor, object.RefCode)
		if err != nil {
			return nil, nil, err
		}
		events := make([]map[string]any, 0, len(detail.Events))
		for _, event := range detail.Events {
			events = append(events, eventPayload(event))
		}
		return map[string]any{
			"ref_code": detail.Aggregate.RefCode, "metadata": detail.Aggregate.Metadata,
			"tags": detail.Aggregate.Tags, "created_at": detail.Aggregate.CreatedAt, "events": events,
		}, detail.Aggregate.Tags, nil
	case ref.ObjectTypeEvent:
		if r.calendar == nil {
			return nil, nil, ErrDependencyUnavailable
		}
		event, err := r.calendar.GetEvent(ctx, actor, object.RefCode)
		if err != nil {
			return nil, nil, err
		}
		return eventPayload(event), event.Tags, nil
	default:
		return nil, nil, fmt.Errorf("unsupported LLM reference object type: %s", object.ObjectType)
	}
}

func eventPayload(event calendar.Event) map[string]any {
	return map[string]any{
		"ref_code": event.RefCode, "aggregate_ref_code": event.AggregateRefCode,
		"starts_at": event.StartsAt, "duration_minutes": event.DurationMinutes,
		"metadata": event.Metadata, "status": event.Status, "tags": event.Tags,
		"created_at": event.CreatedAt, "updated_at": event.UpdatedAt,
	}
}
