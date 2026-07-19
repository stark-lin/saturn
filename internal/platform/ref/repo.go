// This file defines object reference persistence boundaries.
package ref

import "context"

type Repository interface {
	NextSequence(ctx context.Context) (int64, error)
	Register(ctx context.Context, object ObjectRef) (ObjectRef, error)
	FindByCode(ctx context.Context, code string) (ObjectRef, error)
	ListRecent(ctx context.Context, limit int) ([]ObjectRef, error)
	Search(ctx context.Context, query MetadataSearchQuery) ([]ObjectRef, error)
	UpdateProjection(ctx context.Context, update ProjectionUpdate) (ObjectRef, error)
	Delete(ctx context.Context, ownerID int64, objectType ObjectType, objectID int64) error
}
