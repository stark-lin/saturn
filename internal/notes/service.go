// This file enforces owner-only Notes business rules and coordinated writes.
package notes

import (
	"context"
	"errors"
	"strings"

	"github.com/stark-lin/go-proj/internal/platform/audit"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

var ErrDependencyUnavailable = errors.New("notes dependency is not wired")

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

type Service struct {
	repo         Repository
	transactions platformdb.TransactionRunner
	references   ObjectReferenceService
	audit        AuditService
}

func NewService(
	repo Repository,
	transactions platformdb.TransactionRunner,
	references ObjectReferenceService,
	auditService AuditService,
) *Service {
	if transactions == nil {
		transactions = platformdb.NoopTransactionRunner{}
	}
	return &Service{
		repo:         repo,
		transactions: transactions,
		references:   references,
		audit:        auditService,
	}
}

func (s *Service) ListNotes(ctx context.Context, actor auth.Principal, query Query) (Page, error) {
	if actor.IsZero() {
		return Page{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return Page{}, ErrRepositoryUnavailable
	}
	query = normalizedQuery(query)
	page, err := s.repo.ListNotes(ctx, actor.ID, query)
	if err != nil {
		return Page{}, err
	}
	if err := hydrateDerivedFields(page.Notes); err != nil {
		return Page{}, err
	}
	page.Limit = query.Limit
	page.Offset = query.Offset
	return page, nil
}

func (s *Service) CreateNote(ctx context.Context, actor auth.Principal, markdown string) (Note, error) {
	if actor.IsZero() {
		return Note{}, auth.ErrUnauthenticated
	}
	parsed, err := ParseMarkdown(markdown)
	if err != nil {
		return Note{}, err
	}
	if err := s.requireWriteDependencies(); err != nil {
		return Note{}, err
	}
	refCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeNote)
	if err != nil {
		return Note{}, err
	}

	var created Note
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		note, err := s.repo.CreateNote(txCtx, actor.ID, parsed.Title, markdown)
		if err != nil {
			return err
		}
		object, err := s.references.Register(txCtx, ref.Registration{
			OwnerID:    actor.ID,
			RefCode:    refCode,
			ObjectType: ref.ObjectTypeNote,
			ObjectID:   note.ID,
			Title:      parsed.Title,
			Tags:       parsed.Tags,
			Status:     string(NoteDraft),
		})
		if err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType:     audit.ActorTypeUser,
			ActorUserID:   actor.ID,
			Action:        audit.ActionCreate,
			TargetRefCode: object.RefCode,
			Result:        audit.ResultSuccess,
		}); err != nil {
			return err
		}
		note.ObjectRefID = object.ID
		note.RefCode = object.RefCode
		note.Tags = parsed.Tags
		note.Status = NoteStatus(object.Status)
		created = note
		return nil
	})
	if err != nil {
		return Note{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, refCode, err)
	}
	return created, nil
}

func (s *Service) GetNote(ctx context.Context, actor auth.Principal, refCode string) (Note, error) {
	if actor.IsZero() {
		return Note{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return Note{}, ErrRepositoryUnavailable
	}
	note, err := s.repo.FindNoteByRefCode(ctx, actor.ID, refCode)
	if err != nil {
		return Note{}, err
	}
	parsed, err := ParseMarkdown(note.Markdown)
	if err != nil {
		return Note{}, err
	}
	note.Title = parsed.Title
	note.Tags = parsed.Tags
	return note, nil
}

func (s *Service) UpdateNote(ctx context.Context, actor auth.Principal, refCode string, markdown string) (Note, error) {
	if actor.IsZero() {
		return Note{}, auth.ErrUnauthenticated
	}
	parsed, err := ParseMarkdown(markdown)
	if err != nil {
		return Note{}, err
	}
	if err := s.requireWriteDependencies(); err != nil {
		return Note{}, err
	}

	var updated Note
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		note, err := s.repo.UpdateNote(txCtx, actor.ID, refCode, parsed.Title, markdown)
		if err != nil {
			return err
		}
		object, err := s.references.UpdateProjection(txCtx, ref.ProjectionUpdate{
			OwnerID:    actor.ID,
			ObjectType: ref.ObjectTypeNote,
			ObjectID:   note.ID,
			Title:      parsed.Title,
			Tags:       parsed.Tags,
			Status:     string(note.Status),
		})
		if err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType:     audit.ActorTypeUser,
			ActorUserID:   actor.ID,
			Action:        audit.ActionUpdate,
			TargetRefCode: refCode,
			Result:        audit.ResultSuccess,
		}); err != nil {
			return err
		}
		note.ObjectRefID = object.ID
		note.RefCode = refCode
		note.Tags = parsed.Tags
		note.Status = NoteStatus(object.Status)
		updated = note
		return nil
	})
	if err != nil {
		return Note{}, s.recordWriteFailure(ctx, actor, audit.ActionUpdate, refCode, err)
	}
	return updated, nil
}

func (s *Service) DeleteNote(ctx context.Context, actor auth.Principal, refCode string) error {
	if actor.IsZero() {
		return auth.ErrUnauthenticated
	}
	if err := s.requireWriteDependencies(); err != nil {
		return err
	}
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		note, err := s.repo.FindNoteByRefCode(txCtx, actor.ID, refCode)
		if err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType:     audit.ActorTypeUser,
			ActorUserID:   actor.ID,
			Action:        audit.ActionDelete,
			TargetRefCode: refCode,
			Result:        audit.ResultSuccess,
		}); err != nil {
			return err
		}
		if err := s.references.Delete(txCtx, actor.ID, ref.ObjectTypeNote, note.ID); err != nil {
			return err
		}
		return s.repo.DeleteNote(txCtx, actor.ID, note.ID)
	})
	if err != nil {
		return s.recordWriteFailure(ctx, actor, audit.ActionDelete, refCode, err)
	}
	return nil
}

func (s *Service) recordWriteFailure(ctx context.Context, actor auth.Principal, action audit.Action, refCode string, operationErr error) error {
	result := audit.ResultFailed
	reason := "operation_failed"
	if errors.Is(operationErr, ErrNoteNotFound) || errors.Is(operationErr, ref.ErrNotFound) {
		result = audit.ResultDenied
		reason = "not_found"
	}
	auditErr := s.audit.RecordStandalone(ctx, audit.Event{
		ActorType:     audit.ActorTypeUser,
		ActorUserID:   actor.ID,
		Action:        action,
		TargetRefCode: refCode,
		Result:        result,
		Reason:        reason,
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

func normalizedQuery(query Query) Query {
	query.Text = strings.TrimSpace(query.Text)
	query.Tag = strings.TrimSpace(query.Tag)
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	return query
}

func hydrateDerivedFields(notes []Note) error {
	for i := range notes {
		parsed, err := ParseMarkdown(notes[i].Markdown)
		if err != nil {
			return err
		}
		notes[i].Title = parsed.Title
		notes[i].Tags = parsed.Tags
	}
	return nil
}
