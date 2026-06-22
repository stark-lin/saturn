// This file defines object storage metadata persistence boundaries.
package storage

import "context"

type Repository interface {
	Save(ctx context.Context, object Object) error
	Find(ctx context.Context, key string) (Object, error)
	Delete(ctx context.Context, key string) error
}
