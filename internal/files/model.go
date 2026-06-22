// This file defines immutable Files collection and file domain models.
package files

import (
	"io"
	"time"
)

const (
	CollectionStatusActive = "active"
	FileStatusActive       = "active"
)

type DeleteReason string

const (
	DeleteReasonDirectFileDelete        DeleteReason = "direct_file_delete"
	DeleteReasonCascadeCollectionDelete DeleteReason = "cascade_collection_delete"
	DeleteReasonCollectionDelete        DeleteReason = "collection_delete"
)

type Collection struct {
	ID          int64
	OwnerID     int64
	ObjectRefID int64
	RefCode     string
	Name        string
	Description string
	Status      string
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type File struct {
	ID                int64
	OwnerID           int64
	CollectionID      int64
	ObjectRefID       int64
	RefCode           string
	CollectionRefCode string
	ObjectKey         string
	OriginalName      string
	MimeType          string
	SizeBytes         int64
	SHA256            string
	BLAKE3            string
	Status            string
	Tags              []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type FileTag struct {
	ID      int64
	OwnerID int64
	Name    string
}

type Attachment struct {
	ID           int64
	FileID       int64
	ResourceType string
	ResourceID   int64
}

type FileLink struct {
	ID        int64
	FileID    int64
	Token     string
	ExpiresAt time.Time
}

type FileVersion struct {
	ID        int64
	FileID    int64
	ObjectKey string
	CreatedAt time.Time
}

type TrashEntry struct {
	ID        int64
	FileID    int64
	DeletedAt time.Time
}

type CreateCollectionInput struct {
	Name        string
	Description string
	Tags        []string
}

type CreateFileInput struct {
	CollectionRefCode string
	OriginalName      string
	MimeType          string
	SizeBytes         int64
	Body              io.Reader
	Tags              []string
}

type StoredFile struct {
	ObjectKey string
	SizeBytes int64
	SHA256    string
	BLAKE3    string
}

type VerifiedDownload struct {
	File File
	Body io.ReadCloser
}
