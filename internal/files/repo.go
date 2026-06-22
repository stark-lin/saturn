// This file defines Files data access boundaries.
package files

import (
	"context"
	"errors"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

var (
	ErrRepositoryUnavailable = errors.New("files repository is not wired")
	ErrCollectionNotFound    = errors.New("file collection not found")
	ErrFileNotFound          = errors.New("file not found")
)

type Repository interface {
	ListCollections(ctx context.Context, scope auth.Scope, query CollectionQuery) (CollectionPage, error)
	CreateCollection(ctx context.Context, ownerID int64, input CreateCollectionInput) (Collection, error)
	FindCollectionByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Collection, error)
	LockCollectionByRefCode(ctx context.Context, refCode string) (Collection, error)
	DeleteCollection(ctx context.Context, ownerID int64, collectionID int64) error
	ListFileIDsForCollection(ctx context.Context, ownerID int64, collectionID int64) ([]int64, error)

	ListFiles(ctx context.Context, scope auth.Scope, query FileQuery) (FilePage, error)
	CreateFile(ctx context.Context, ownerID int64, collectionID int64, input CreateFileInput, stored StoredFile) (File, error)
	FindFileByRefCode(ctx context.Context, scope auth.Scope, refCode string) (File, error)
	LockFileByRefCode(ctx context.Context, refCode string) (File, error)
	LockFileByID(ctx context.Context, ownerID int64, fileID int64) (File, error)
	DeleteFile(ctx context.Context, ownerID int64, fileID int64) error
}
