// This file defines Notes domain models.
package notes

import "time"

type NoteStatus string

const (
	NoteDraft NoteStatus = "draft"
)

type Note struct {
	ID          int64
	OwnerID     int64
	ObjectRefID int64
	RefCode     string
	Title       string
	Markdown    string
	Tags        []string
	Status      NoteStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
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

type Revision struct {
	ID        int64
	NoteID    int64
	Markdown  string
	CreatedAt time.Time
}
