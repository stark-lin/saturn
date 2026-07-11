// This file defines Notes domain models.
package notes

import "time"

type NoteStatus string

const (
	NoteDraft   NoteStatus = "draft"
	NoteDeleted NoteStatus = "deleted"
)

type VersionOperation string

const (
	VersionOperationCreate  VersionOperation = "create"
	VersionOperationUpdate  VersionOperation = "update"
	VersionOperationRestore VersionOperation = "restore"
	MarkdownContentType                      = "text/markdown"
)

type Note struct {
	ID                      int64
	OwnerID                 int64
	CurrentVersionID        int64
	ObjectRefID             int64
	RefCode                 string
	CurrentVersionRef       string
	CurrentParentVersionID  *int64
	CurrentParentVersionRef string
	CurrentVersionNumber    int64
	Title                   string
	Markdown                string
	ContentType             string
	CurrentVersionOperation VersionOperation
	Tags                    []string
	Status                  NoteStatus
	CreatedAt               time.Time
	UpdatedAt               time.Time
	DeletedAt               *time.Time
}

type Version struct {
	ID               int64
	NoteID           int64
	ParentVersionID  *int64
	OwnerID          int64
	ObjectRefID      int64
	RefCode          string
	NoteRefCode      string
	ParentVersionRef string
	VersionNumber    int64
	Title            string
	Content          string
	ContentType      string
	Operation        VersionOperation
	Tags             []string
	CreatedAt        time.Time
}

type Tag struct {
	ID      int64
	OwnerID int64
	Name    string
}

type Collection struct {
	ID      int64
	OwnerID int64
	Name    string
}

type NoteLink struct {
	ID           int64
	SourceNoteID int64
	TargetNoteID int64
}

type NoteTemplate struct {
	ID      int64
	OwnerID int64
	Name    string
	Body    string
}

type NoteSource struct {
	ID       int64
	OwnerID  int64
	Kind     string
	Title    string
	Endpoint string
}
