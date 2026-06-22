// This file tests owner-only Notes service behavior.
package notes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/audit"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

func TestNewModuleBuildsNotesDependencies(t *testing.T) {
	module := NewModule(nil, nil, nil, nil)
	if module.Service == nil {
		t.Fatal("expected service")
	}
	if module.Handler == nil {
		t.Fatal("expected handler")
	}
}

func TestParseMarkdownDerivesTitleAndUniqueTags(t *testing.T) {
	parsed, err := ParseMarkdown(" Backup checklist \npostgres, backup, postgres, \n\nRun backup.")
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	if parsed.Title != "Backup checklist" {
		t.Fatalf("title = %q, want %q", parsed.Title, "Backup checklist")
	}
	if len(parsed.Tags) != 2 || parsed.Tags[0] != "postgres" || parsed.Tags[1] != "backup" {
		t.Fatalf("tags = %#v, want postgres and backup", parsed.Tags)
	}
}

func TestParseMarkdownAcceptsTitleBeginningWithHashText(t *testing.T) {
	parsed, err := ParseMarkdown("#tag overview\nreference\n\nBody")
	if err != nil || parsed.Title != "#tag overview" {
		t.Fatalf("parse hash-text title = %#v, error = %v", parsed, err)
	}
}

func TestServiceCreateNoteOwnsCurrentMarkdownAndCoordinatesDependencies(t *testing.T) {
	repo := newFakeNoteRepository()
	references := newFakeReferences()
	audits := &fakeAudits{}
	service := NewService(repo, nil, references, audits)
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}

	note, err := service.CreateNote(context.Background(), actor, "Release notes\nrelease, notes\n\nCurrent body")
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if note.OwnerID != actor.ID || note.RefCode != "NTE-00000001" || note.Title != "Release notes" {
		t.Fatalf("created note = %#v", note)
	}
	if references.registration.Status != string(NoteDraft) || note.Status != NoteDraft {
		t.Fatalf("statuses = registration %q note %q, want draft", references.registration.Status, note.Status)
	}
	if len(references.registration.Tags) != 2 || references.registration.Tags[0] != "release" || references.registration.Tags[1] != "notes" {
		t.Fatalf("registration tags = %#v, want release and notes", references.registration.Tags)
	}
	if len(audits.events) != 1 || audits.events[0].Action != audit.ActionCreate || audits.events[0].TargetRefCode != "NTE-00000001" {
		t.Fatalf("audit events = %#v, want create", audits.events)
	}
}

func TestServiceReadIsOwnerOnlyIncludingSuperuser(t *testing.T) {
	repo := newFakeNoteRepository()
	repo.notes["NTE-00000001"] = Note{
		ID:       1,
		OwnerID:  7,
		RefCode:  "NTE-00000001",
		Markdown: "Owner note\nprivate\n\nBody",
		Status:   NoteDraft,
	}
	service := NewService(repo, nil, nil, nil)

	owner, err := service.GetNote(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, "NTE-00000001")
	if err != nil || owner.Title != "Owner note" {
		t.Fatalf("owner get note = %#v, error = %v", owner, err)
	}
	for _, actor := range []auth.Principal{
		{ID: 8, Role: auth.RoleUser},
		{ID: 9, Role: auth.RoleSuperuser},
	} {
		if _, err := service.GetNote(context.Background(), actor, "NTE-00000001"); !errors.Is(err, ErrNoteNotFound) {
			t.Fatalf("actor %#v error = %v, want not found", actor, err)
		}
	}
}

func TestServiceListUsesActorOwnershipIncludingSuperuser(t *testing.T) {
	repo := newFakeNoteRepository()
	repo.notes["NTE-00000001"] = Note{ID: 1, OwnerID: 7, RefCode: "NTE-00000001", Markdown: "Owner note\n\nBody", Status: NoteDraft}
	repo.notes["NTE-00000002"] = Note{ID: 2, OwnerID: 9, RefCode: "NTE-00000002", Markdown: "Admin note\n\nBody", Status: NoteDraft}
	service := NewService(repo, nil, nil, nil)

	page, err := service.ListNotes(context.Background(), auth.Principal{ID: 9, Role: auth.RoleSuperuser}, Query{})
	if err != nil {
		t.Fatalf("list superuser notes: %v", err)
	}
	if len(page.Notes) != 1 || page.Notes[0].OwnerID != 9 {
		t.Fatalf("listed notes = %#v, want only actor-owned notes", page.Notes)
	}
}

func TestServiceUpdateReplacesCurrentMarkdownWithoutVersionBehavior(t *testing.T) {
	repo := newFakeNoteRepository()
	repo.notes["NTE-00000001"] = Note{ID: 1, OwnerID: 7, ObjectRefID: 4, RefCode: "NTE-00000001", Markdown: "Old\nold\n\nBody", Status: NoteDraft}
	references := newFakeReferences()
	audits := &fakeAudits{}
	service := NewService(repo, nil, references, audits)

	updated, err := service.UpdateNote(
		context.Background(),
		auth.Principal{ID: 7, Role: auth.RoleUser},
		"NTE-00000001",
		"New title\nfresh\n\nReplacement",
	)
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if updated.Markdown != "New title\nfresh\n\nReplacement" || updated.Title != "New title" {
		t.Fatalf("updated note = %#v", updated)
	}
	if repo.notes["NTE-00000001"].Markdown != updated.Markdown {
		t.Fatalf("stored markdown = %q, want current replacement", repo.notes["NTE-00000001"].Markdown)
	}
	if len(references.update.Tags) != 1 || references.update.Tags[0] != "fresh" {
		t.Fatalf("projection tags = %#v, want fresh", references.update.Tags)
	}
	if len(audits.events) != 1 || audits.events[0].Action != audit.ActionUpdate {
		t.Fatalf("audit events = %#v, want update", audits.events)
	}
}

func TestServiceUpdateRecordsDeniedOnlyAfterTargetWriteFails(t *testing.T) {
	repo := newFakeNoteRepository()
	audits := &fakeAudits{}
	service := NewService(repo, nil, newFakeReferences(), audits)

	_, err := service.UpdateNote(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, "NTE-00000009", "Title\n\nBody")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("update error = %v, want not found", err)
	}
	if len(audits.events) != 1 || audits.events[0].Result != audit.ResultDenied || audits.events[0].TargetRefCode != "NTE-00000009" {
		t.Fatalf("failed update audits = %#v, want denied target audit", audits.events)
	}
}

func TestServiceDeleteNoteRemovesReferenceAndRecordsAudit(t *testing.T) {
	repo := newFakeNoteRepository()
	repo.notes["NTE-00000001"] = Note{
		ID: 1, OwnerID: 7, RefCode: "NTE-00000001", Markdown: "Old note\nprivate\n\nBody", Status: NoteDraft,
	}
	references := newFakeReferences()
	audits := &fakeAudits{}
	service := NewService(repo, nil, references, audits)

	err := service.DeleteNote(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, "NTE-00000001")
	if err != nil {
		t.Fatalf("delete note: %v", err)
	}
	if _, exists := repo.notes["NTE-00000001"]; exists {
		t.Fatalf("note remained after delete: %#v", repo.notes["NTE-00000001"])
	}
	if len(references.deletes) != 1 ||
		references.deletes[0].objectType != ref.ObjectTypeNote ||
		references.deletes[0].objectID != 1 {
		t.Fatalf("reference deletes = %#v", references.deletes)
	}
	if len(audits.events) != 1 ||
		audits.events[0].Action != audit.ActionDelete ||
		audits.events[0].Result != audit.ResultSuccess ||
		audits.events[0].TargetRefCode != "NTE-00000001" {
		t.Fatalf("delete audits = %#v", audits.events)
	}
}

func TestServiceDeleteRecordsDeniedForMissingNote(t *testing.T) {
	repo := newFakeNoteRepository()
	audits := &fakeAudits{}
	service := NewService(repo, nil, newFakeReferences(), audits)

	err := service.DeleteNote(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, "NTE-00000009")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("delete error = %v, want note not found", err)
	}
	if len(audits.events) != 1 ||
		audits.events[0].Result != audit.ResultDenied ||
		audits.events[0].Reason != "not_found" ||
		audits.events[0].TargetRefCode != "NTE-00000009" {
		t.Fatalf("failed delete audits = %#v", audits.events)
	}
}

type fakeNoteRepository struct {
	nextID int64
	notes  map[string]Note
}

func newFakeNoteRepository() *fakeNoteRepository {
	return &fakeNoteRepository{notes: make(map[string]Note)}
}

func (r *fakeNoteRepository) ListNotes(_ context.Context, ownerID int64, query Query) (Page, error) {
	page := Page{Limit: query.Limit, Offset: query.Offset}
	for _, note := range r.notes {
		if note.OwnerID == ownerID {
			page.Notes = append(page.Notes, note)
		}
	}
	return page, nil
}

func (r *fakeNoteRepository) CreateNote(_ context.Context, ownerID int64, title string, markdown string) (Note, error) {
	r.nextID++
	note := Note{ID: r.nextID, OwnerID: ownerID, Title: title, Markdown: markdown, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return note, nil
}

func (r *fakeNoteRepository) FindNoteByRefCode(_ context.Context, ownerID int64, refCode string) (Note, error) {
	note, exists := r.notes[refCode]
	if !exists || note.OwnerID != ownerID {
		return Note{}, ErrNoteNotFound
	}
	return note, nil
}

func (r *fakeNoteRepository) UpdateNote(_ context.Context, ownerID int64, refCode string, title string, markdown string) (Note, error) {
	note, err := r.FindNoteByRefCode(context.Background(), ownerID, refCode)
	if err != nil {
		return Note{}, err
	}
	note.Title = title
	note.Markdown = markdown
	r.notes[refCode] = note
	return note, nil
}

func (r *fakeNoteRepository) DeleteNote(_ context.Context, ownerID int64, noteID int64) error {
	for code, note := range r.notes {
		if note.OwnerID == ownerID && note.ID == noteID {
			delete(r.notes, code)
			return nil
		}
	}
	return ErrNoteNotFound
}

type fakeReferences struct {
	nextID       int64
	registration ref.Registration
	update       ref.ProjectionUpdate
	deletes      []fakeReferenceDelete
}

type fakeReferenceDelete struct {
	objectType ref.ObjectType
	objectID   int64
}

func newFakeReferences() *fakeReferences {
	return &fakeReferences{}
}

func (r *fakeReferences) ClaimCode(_ context.Context, _ ref.ObjectType) (string, error) {
	return "NTE-00000001", nil
}

func (r *fakeReferences) Register(_ context.Context, registration ref.Registration) (ref.ObjectRef, error) {
	r.nextID++
	r.registration = registration
	return ref.ObjectRef{ID: r.nextID, RefCode: "NTE-00000001", Status: registration.Status}, nil
}

func (r *fakeReferences) UpdateProjection(_ context.Context, update ref.ProjectionUpdate) (ref.ObjectRef, error) {
	r.nextID++
	r.update = update
	return ref.ObjectRef{ID: r.nextID, RefCode: "NTE-00000001", Title: update.Title, Status: update.Status}, nil
}

func (r *fakeReferences) Delete(_ context.Context, _ int64, objectType ref.ObjectType, objectID int64) error {
	r.deletes = append(r.deletes, fakeReferenceDelete{objectType: objectType, objectID: objectID})
	return nil
}

type fakeAudits struct {
	events []audit.Event
}

func (a *fakeAudits) Record(_ context.Context, event audit.Event) (audit.Event, error) {
	a.events = append(a.events, event)
	return event, nil
}

func (a *fakeAudits) RecordStandalone(_ context.Context, event audit.Event) error {
	a.events = append(a.events, event)
	return nil
}
