// This file defines audit event persistence boundaries.
package audit

import "context"

type Repository interface {
	Insert(ctx context.Context, event Event) (Event, error)
	List(ctx context.Context, query Query) ([]Event, error)
}
