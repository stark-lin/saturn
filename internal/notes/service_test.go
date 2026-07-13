// This file tests logical Note and immutable version service behavior.
package notes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stark-lin/saturn/internal/platform/audit"
	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

func TestNewModuleBuildsNotesDependencies(t *testing.T) {
	module := NewModule(nil, nil, nil, nil)
	if module.Service == nil || module.Handler == nil {
		t.Fatalf("module = %#v, want service and handler", module)
	}
}

func TestParseMarkdownDerivesTitleAndUniqueTags(t *testing.T) {
	parsed, err := ParseMarkdown(" Backup checklist \npostgres, backup, postgres, \n\nRun backup.")
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	if parsed.Title != "Backup checklist" || len(parsed.Tags) != 2 || parsed.Tags[0] != "postgres" || parsed.Tags[1] != "backup" {
		t.Fatalf("parsed markdown = %#v", parsed)
	}
}

func TestParseMarkdownAcceptsTitleBeginningWithHashText(t *testing.T) {
	parsed, err := ParseMarkdown("#tag overview\nreference\n\nBody")
	if err != nil || parsed.Title != "#tag overview" {
		t.Fatalf("parsed markdown = %#v, error = %v", parsed, err)
	}
}

func TestServiceCreateRegistersLogicalAndInitialVersionObjects(t *testing.T) {
	service, repo, references, audits := newServiceFixture()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}

	note, err := service.CreateNote(context.Background(), actor, "Release notes\nrelease, notes\n\nCurrent body")
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if note.RefCode != "NTE-00000001" || note.CurrentVersionRef != "NTE-00000002" || note.CurrentVersionNumber != 1 {
		t.Fatalf("created note = %#v", note)
	}
	if note.CurrentVersionOperation != VersionOperationCreate || note.Markdown != "Release notes\nrelease, notes\n\nCurrent body" {
		t.Fatalf("current version fields = %#v", note)
	}
	if len(references.registrations) != 2 || references.registrations[0].ObjectType != ref.ObjectTypeNote || references.registrations[1].ObjectType != ref.ObjectTypeNoteVersion {
		t.Fatalf("registrations = %#v", references.registrations)
	}
	if len(repo.versions) != 1 || len(audits.events) != 2 || audits.events[0].TargetRefCode != note.CurrentVersionRef || audits.events[1].TargetRefCode != note.RefCode {
		t.Fatalf("versions = %#v audits = %#v", repo.versions, audits.events)
	}
}

func TestServiceUpdateAppendsVersionAndAdvancesCurrentPointer(t *testing.T) {
	service, repo, _, _ := newServiceFixture()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}
	created := mustCreateNote(t, service, actor, "Old\nold\n\nBody")

	updated, err := service.UpdateNote(context.Background(), actor, created.RefCode, "New title\nfresh\n\nReplacement")
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if updated.RefCode != created.RefCode || updated.CurrentVersionRef != "NTE-00000003" || updated.CurrentVersionNumber != 2 {
		t.Fatalf("updated note = %#v", updated)
	}
	if updated.CurrentParentVersionRef != created.CurrentVersionRef || updated.CurrentVersionOperation != VersionOperationUpdate {
		t.Fatalf("updated lineage = %#v", updated)
	}
	first, err := service.GetVersion(context.Background(), actor, created.CurrentVersionRef)
	if err != nil || first.Content != "Old\nold\n\nBody" {
		t.Fatalf("initial immutable version = %#v, error = %v", first, err)
	}
	if len(repo.versions) != 2 {
		t.Fatalf("version count = %d, want 2", len(repo.versions))
	}
}

func TestServiceHardDeleteRemovesNoteVersionsAndReferences(t *testing.T) {
	service, repo, references, audits := newServiceFixture()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}
	created := mustCreateNote(t, service, actor, "Private\nsecret\n\nBody")
	updated, err := service.UpdateNote(context.Background(), actor, created.RefCode, "Private updated\nsecret\n\nReplacement")
	if err != nil {
		t.Fatalf("update note before delete: %v", err)
	}

	if err := service.DeleteNote(context.Background(), actor, created.RefCode); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	if _, err := service.GetNote(context.Background(), actor, created.RefCode); !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("deleted note read error = %v", err)
	}
	if _, err := service.GetVersion(context.Background(), actor, created.CurrentVersionRef); !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("deleted initial version read error = %v", err)
	}
	if _, err := service.GetVersion(context.Background(), actor, updated.CurrentVersionRef); !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("deleted current version read error = %v", err)
	}
	if len(repo.notes) != 0 || len(repo.versions) != 0 {
		t.Fatalf("hard-deleted repository state = notes %#v versions %#v", repo.notes, repo.versions)
	}
	if len(references.deletes) != 3 || references.deletes[0].objectType != ref.ObjectTypeNoteVersion || references.deletes[1].objectType != ref.ObjectTypeNoteVersion || references.deletes[2].objectType != ref.ObjectTypeNote {
		t.Fatalf("deleted references = %#v", references.deletes)
	}
	if len(audits.events) != 7 || audits.events[4].Reason != deleteReasonCascadeNoteDelete || audits.events[5].Reason != deleteReasonCascadeNoteDelete || audits.events[6].TargetRefCode != created.RefCode {
		t.Fatalf("delete audits = %#v", audits.events)
	}
}

func TestServiceReadsRemainOwnerOnlyIncludingSuperuser(t *testing.T) {
	service, _, _, _ := newServiceFixture()
	owner := auth.Principal{ID: 7, Role: auth.RoleUser}
	created := mustCreateNote(t, service, owner, "Owner note\nprivate\n\nBody")

	for _, actor := range []auth.Principal{{ID: 8, Role: auth.RoleUser}, {ID: 9, Role: auth.RoleSuperuser}} {
		if _, err := service.GetNote(context.Background(), actor, created.RefCode); !errors.Is(err, ErrNoteNotFound) {
			t.Fatalf("actor %#v note error = %v", actor, err)
		}
		if _, err := service.GetVersion(context.Background(), actor, created.CurrentVersionRef); !errors.Is(err, ErrVersionNotFound) {
			t.Fatalf("actor %#v version error = %v", actor, err)
		}
	}
}

func TestServiceUpdateMissingNoteRecordsDeniedAudit(t *testing.T) {
	service, _, _, audits := newServiceFixture()
	_, err := service.UpdateNote(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, "NTE-00000009", "Title\n\nBody")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("update error = %v", err)
	}
	if len(audits.events) != 1 || audits.events[0].Result != audit.ResultDenied || audits.events[0].TargetRefCode != "NTE-00000009" {
		t.Fatalf("audits = %#v", audits.events)
	}
}

func mustCreateNote(t *testing.T, service *Service, actor auth.Principal, markdown string) Note {
	t.Helper()
	note, err := service.CreateNote(context.Background(), actor, markdown)
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	return note
}

func newServiceFixture() (*Service, *fakeNoteRepository, *fakeReferences, *fakeAudits) {
	repo := newFakeNoteRepository()
	references := &fakeReferences{repo: repo}
	audits := &fakeAudits{}
	return NewService(repo, nil, references, audits), repo, references, audits
}

type fakeNoteRepository struct {
	nextNoteID    int64
	nextVersionID int64
	notes         map[int64]Note
	versions      map[int64]Version
}

func newFakeNoteRepository() *fakeNoteRepository {
	return &fakeNoteRepository{notes: make(map[int64]Note), versions: make(map[int64]Version)}
}

func (r *fakeNoteRepository) ListNotes(_ context.Context, ownerID int64, query Query) (Page, error) {
	notes := make([]Note, 0)
	for _, note := range r.notes {
		note = r.hydrateNote(note)
		if note.OwnerID != ownerID {
			continue
		}
		if query.Text != "" && !strings.Contains(note.Title+note.Markdown, query.Text) {
			continue
		}
		notes = append(notes, note)
	}
	return Page{Notes: notes, Limit: query.Limit, Offset: query.Offset}, nil
}

func (r *fakeNoteRepository) CreateNote(_ context.Context, ownerID int64) (Note, error) {
	r.nextNoteID++
	now := time.Unix(r.nextNoteID, 0).UTC()
	note := Note{ID: r.nextNoteID, OwnerID: ownerID, CreatedAt: now, UpdatedAt: now}
	r.notes[note.ID] = note
	return note, nil
}

func (r *fakeNoteRepository) FindNoteByRefCode(_ context.Context, ownerID int64, refCode string) (Note, error) {
	for _, note := range r.notes {
		if note.OwnerID == ownerID && note.RefCode == refCode {
			return r.hydrateNote(note), nil
		}
	}
	return Note{}, ErrNoteNotFound
}

func (r *fakeNoteRepository) LockNoteByRefCode(ctx context.Context, ownerID int64, refCode string) (Note, error) {
	return r.FindNoteByRefCode(ctx, ownerID, refCode)
}

func (r *fakeNoteRepository) CreateVersion(_ context.Context, input CreateVersionInput) (Version, error) {
	r.nextVersionID++
	note := r.notes[input.NoteID]
	version := Version{
		ID: r.nextVersionID, NoteID: input.NoteID, ParentVersionID: input.ParentVersionID,
		OwnerID: note.OwnerID, VersionNumber: input.VersionNumber, Title: input.Title,
		Content: input.Content, ContentType: input.ContentType, Operation: input.Operation,
		CreatedAt: time.Unix(100+r.nextVersionID, 0).UTC(),
	}
	r.versions[version.ID] = version
	return version, nil
}

func (r *fakeNoteRepository) FindVersionByRefCode(_ context.Context, ownerID int64, refCode string) (Version, error) {
	for _, version := range r.versions {
		if version.OwnerID == ownerID && version.RefCode == refCode {
			return r.hydrateVersion(version), nil
		}
	}
	return Version{}, ErrVersionNotFound
}

func (r *fakeNoteRepository) ListVersions(_ context.Context, ownerID int64, noteRefCode string) ([]Version, error) {
	note, err := r.FindNoteByRefCode(context.Background(), ownerID, noteRefCode)
	if err != nil {
		return nil, err
	}
	versions := make([]Version, 0)
	for _, version := range r.versions {
		if version.NoteID == note.ID {
			versions = append(versions, r.hydrateVersion(version))
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].VersionNumber > versions[j].VersionNumber })
	return versions, nil
}

func (r *fakeNoteRepository) SetCurrentVersion(_ context.Context, ownerID int64, noteID int64, versionID int64) error {
	note, ok := r.notes[noteID]
	if !ok || note.OwnerID != ownerID {
		return ErrNoteNotFound
	}
	note.CurrentVersionID = versionID
	note.UpdatedAt = note.UpdatedAt.Add(time.Second)
	r.notes[noteID] = note
	return nil
}

func (r *fakeNoteRepository) DeleteNote(_ context.Context, ownerID int64, noteID int64) error {
	note, ok := r.notes[noteID]
	if !ok || note.OwnerID != ownerID {
		return ErrNoteNotFound
	}
	delete(r.notes, noteID)
	for versionID, version := range r.versions {
		if version.NoteID == noteID {
			delete(r.versions, versionID)
		}
	}
	return nil
}

func (r *fakeNoteRepository) hydrateNote(note Note) Note {
	version := r.hydrateVersion(r.versions[note.CurrentVersionID])
	note.CurrentVersionRef = version.RefCode
	note.CurrentParentVersionID = version.ParentVersionID
	note.CurrentParentVersionRef = version.ParentVersionRef
	note.CurrentVersionNumber = version.VersionNumber
	note.Title = version.Title
	note.Markdown = version.Content
	note.ContentType = version.ContentType
	note.CurrentVersionOperation = version.Operation
	return note
}

func (r *fakeNoteRepository) hydrateVersion(version Version) Version {
	note := r.notes[version.NoteID]
	version.NoteRefCode = note.RefCode
	if version.ParentVersionID != nil {
		version.ParentVersionRef = r.versions[*version.ParentVersionID].RefCode
	}
	return version
}

type fakeReferences struct {
	nextID        int64
	sequence      int64
	repo          *fakeNoteRepository
	registrations []ref.Registration
	updates       []ref.ProjectionUpdate
	deletes       []fakeReferenceDelete
}

type fakeReferenceDelete struct {
	objectType ref.ObjectType
	objectID   int64
}

func (r *fakeReferences) ClaimCode(_ context.Context, objectType ref.ObjectType) (string, error) {
	r.sequence++
	return fmt.Sprintf("NTE-%08X", r.sequence), nil
}

func (r *fakeReferences) Register(_ context.Context, registration ref.Registration) (ref.ObjectRef, error) {
	r.nextID++
	r.registrations = append(r.registrations, registration)
	object := ref.ObjectRef{
		ID: r.nextID, OwnerID: registration.OwnerID, RefCode: registration.RefCode,
		ObjectType: registration.ObjectType, ObjectID: registration.ObjectID,
		Title: registration.Title, Tags: registration.Tags, Status: registration.Status,
	}
	if registration.ObjectType == ref.ObjectTypeNote {
		note := r.repo.notes[registration.ObjectID]
		note.ObjectRefID = object.ID
		note.RefCode = object.RefCode
		note.Title = object.Title
		note.Tags = object.Tags
		note.Status = NoteStatus(object.Status)
		r.repo.notes[note.ID] = note
	} else {
		version := r.repo.versions[registration.ObjectID]
		version.ObjectRefID = object.ID
		version.RefCode = object.RefCode
		version.Tags = object.Tags
		r.repo.versions[version.ID] = version
	}
	return object, nil
}

func (r *fakeReferences) UpdateProjection(_ context.Context, update ref.ProjectionUpdate) (ref.ObjectRef, error) {
	r.updates = append(r.updates, update)
	note := r.repo.notes[update.ObjectID]
	note.Title = update.Title
	note.Tags = update.Tags
	note.Status = NoteStatus(update.Status)
	r.repo.notes[note.ID] = note
	return ref.ObjectRef{ID: note.ObjectRefID, RefCode: note.RefCode, Title: note.Title, Tags: note.Tags, Status: string(note.Status)}, nil
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
