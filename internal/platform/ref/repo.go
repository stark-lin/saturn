// This file defines object reference persistence boundaries.
package ref

import "context"

type Repository interface {
	NextSequence(ctx context.Context) (int64, error)
	Register(ctx context.Context, object ObjectRef) (ObjectRef, error)
	FindByCode(ctx context.Context, code string) (ObjectRef, error)
	ListRecentByOwner(ctx context.Context, ownerID int64, limit int) ([]ObjectRef, error)
	SearchByOwner(ctx context.Context, ownerID int64, query MetadataSearchQuery) ([]ObjectRef, error)
	UpdateProjection(ctx context.Context, update ProjectionUpdate) (ObjectRef, error)
	Delete(ctx context.Context, ownerID int64, objectType ObjectType, objectID int64) error
}
