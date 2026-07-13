// This file enforces logical Note and immutable version business rules.
package notes

import (
	"context"
	"errors"
	"strings"

	"github.com/stark-lin/saturn/internal/platform/audit"
	"github.com/stark-lin/saturn/internal/platform/auth"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

var ErrDependencyUnavailable = errors.New("notes dependency is not wired")

const deleteReasonCascadeNoteDelete = "cascade_note_delete"

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
		repo: repo, transactions: transactions, references: references, audit: auditService,
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
	noteRefCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeNote)
	if err != nil {
		return Note{}, err
	}
	versionRefCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeNoteVersion)
	if err != nil {
		return Note{}, err
	}

	var created Note
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		note, err := s.repo.CreateNote(txCtx, actor.ID)
		if err != nil {
			return err
		}
		noteObject, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: actor.ID, RefCode: noteRefCode, ObjectType: ref.ObjectTypeNote, ObjectID: note.ID,
			Title: parsed.Title, Tags: parsed.Tags, Status: string(NoteDraft),
		})
		if err != nil {
			return err
		}
		version, err := s.repo.CreateVersion(txCtx, CreateVersionInput{
			NoteID: note.ID, VersionNumber: 1, Title: parsed.Title, Content: markdown,
			ContentType: MarkdownContentType, Operation: VersionOperationCreate,
		})
		if err != nil {
			return err
		}
		versionObject, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: actor.ID, RefCode: versionRefCode, ObjectType: ref.ObjectTypeNoteVersion, ObjectID: version.ID,
			Title: parsed.Title, Tags: parsed.Tags, Status: "immutable",
		})
		if err != nil {
			return err
		}
		if err := s.repo.SetCurrentVersion(txCtx, actor.ID, note.ID, version.ID); err != nil {
			return err
		}
		if err := s.recordVersionAndNoteAudits(txCtx, actor, versionObject.RefCode, noteObject.RefCode, audit.ActionCreate, ""); err != nil {
			return err
		}
		created, err = s.repo.FindNoteByRefCode(txCtx, actor.ID, noteObject.RefCode)
		return err
	})
	if err != nil {
		return Note{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, noteRefCode, err)
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
	return s.repo.FindNoteByRefCode(ctx, actor.ID, ref.NormalizeCode(refCode))
}

func (s *Service) GetVersion(ctx context.Context, actor auth.Principal, refCode string) (Version, error) {
	if actor.IsZero() {
		return Version{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return Version{}, ErrRepositoryUnavailable
	}
	return s.repo.FindVersionByRefCode(ctx, actor.ID, ref.NormalizeCode(refCode))
}

func (s *Service) ListVersions(ctx context.Context, actor auth.Principal, noteRefCode string) ([]Version, error) {
	if actor.IsZero() {
		return nil, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return nil, ErrRepositoryUnavailable
	}
	return s.repo.ListVersions(ctx, actor.ID, ref.NormalizeCode(noteRefCode))
}

func (s *Service) UpdateNote(ctx context.Context, actor auth.Principal, refCode string, markdown string) (Note, error) {
	if actor.IsZero() {
		return Note{}, auth.ErrUnauthenticated
	}
	parsed, err := ParseMarkdown(markdown)
	if err != nil {
		return Note{}, err
	}
	return s.createNextVersion(ctx, actor, refCode, nextVersionContent{
		title: parsed.Title, content: markdown, tags: parsed.Tags, operation: VersionOperationUpdate,
	})
}

func (s *Service) DeleteNote(ctx context.Context, actor auth.Principal, refCode string) error {
	if actor.IsZero() {
		return auth.ErrUnauthenticated
	}
	if err := s.requireWriteDependencies(); err != nil {
		return err
	}
	refCode = ref.NormalizeCode(refCode)
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		note, err := s.repo.LockNoteByRefCode(txCtx, actor.ID, refCode)
		if err != nil {
			return err
		}
		versions, err := s.repo.ListVersions(txCtx, actor.ID, note.RefCode)
		if err != nil {
			return err
		}
		for _, version := range versions {
			if _, err := s.audit.Record(txCtx, audit.Event{
				ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
				TargetRefCode: version.RefCode, Result: audit.ResultSuccess, Reason: deleteReasonCascadeNoteDelete,
			}); err != nil {
				return err
			}
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
			TargetRefCode: note.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		for _, version := range versions {
			if err := s.references.Delete(txCtx, note.OwnerID, ref.ObjectTypeNoteVersion, version.ID); err != nil {
				return err
			}
		}
		if err := s.references.Delete(txCtx, note.OwnerID, ref.ObjectTypeNote, note.ID); err != nil {
			return err
		}
		return s.repo.DeleteNote(txCtx, note.OwnerID, note.ID)
	})
	if err != nil {
		return s.recordWriteFailure(ctx, actor, audit.ActionDelete, refCode, err)
	}
	return nil
}

type nextVersionContent struct {
	title     string
	content   string
	tags      []string
	operation VersionOperation
}

func (s *Service) createNextVersion(ctx context.Context, actor auth.Principal, noteRefCode string, next nextVersionContent) (Note, error) {
	if actor.IsZero() {
		return Note{}, auth.ErrUnauthenticated
	}
	if err := s.requireWriteDependencies(); err != nil {
		return Note{}, err
	}
	noteRefCode = ref.NormalizeCode(noteRefCode)
	versionRefCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeNoteVersion)
	if err != nil {
		return Note{}, err
	}

	var updated Note
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		note, err := s.repo.LockNoteByRefCode(txCtx, actor.ID, noteRefCode)
		if err != nil {
			return err
		}
		parentVersionID := note.CurrentVersionID
		version, err := s.repo.CreateVersion(txCtx, CreateVersionInput{
			NoteID: note.ID, ParentVersionID: &parentVersionID, VersionNumber: note.CurrentVersionNumber + 1,
			Title: next.title, Content: next.content, ContentType: MarkdownContentType, Operation: next.operation,
		})
		if err != nil {
			return err
		}
		versionObject, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: actor.ID, RefCode: versionRefCode, ObjectType: ref.ObjectTypeNoteVersion, ObjectID: version.ID,
			Title: next.title, Tags: next.tags, Status: "immutable",
		})
		if err != nil {
			return err
		}
		if err := s.repo.SetCurrentVersion(txCtx, actor.ID, note.ID, version.ID); err != nil {
			return err
		}
		if _, err := s.references.UpdateProjection(txCtx, ref.ProjectionUpdate{
			OwnerID: actor.ID, ObjectType: ref.ObjectTypeNote, ObjectID: note.ID,
			Title: next.title, Tags: next.tags, Status: string(NoteDraft),
		}); err != nil {
			return err
		}
		if err := s.recordVersionAndNoteAudits(txCtx, actor, versionObject.RefCode, note.RefCode, audit.ActionUpdate, ""); err != nil {
			return err
		}
		updated, err = s.repo.FindNoteByRefCode(txCtx, actor.ID, note.RefCode)
		return err
	})
	if err != nil {
		return Note{}, s.recordWriteFailure(ctx, actor, audit.ActionUpdate, noteRefCode, err)
	}
	return updated, nil
}

func (s *Service) recordVersionAndNoteAudits(
	ctx context.Context,
	actor auth.Principal,
	versionRefCode string,
	noteRefCode string,
	noteAction audit.Action,
	reason string,
) error {
	if _, err := s.audit.Record(ctx, audit.Event{
		ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
		TargetRefCode: versionRefCode, Result: audit.ResultSuccess,
	}); err != nil {
		return err
	}
	_, err := s.audit.Record(ctx, audit.Event{
		ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: noteAction,
		TargetRefCode: noteRefCode, Result: audit.ResultSuccess, Reason: reason,
	})
	return err
}

func (s *Service) recordWriteFailure(ctx context.Context, actor auth.Principal, action audit.Action, refCode string, operationErr error) error {
	result := audit.ResultFailed
	reason := "operation_failed"
	if errors.Is(operationErr, ErrNoteNotFound) || errors.Is(operationErr, ErrVersionNotFound) || errors.Is(operationErr, ref.ErrNotFound) {
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

func normalizedQuery(query Query) Query {
	query.Text = strings.TrimSpace(query.Text)
	query.Tag = strings.TrimSpace(query.Tag)
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	return query
}
