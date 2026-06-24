// This file defines Calendar data access boundaries.
package calendar

import (
	"context"
	"errors"

	"github.com/stark-lin/saturn/internal/platform/auth"
)

var (
	ErrEventAggregateNotFound = errors.New("event aggregate not found")
	ErrEventNotFound          = errors.New("event not found")
	ErrEventAlreadyFinished   = errors.New("event is already finished")
	ErrEventAlreadyVoided     = errors.New("event is already voided")
)

type Repository interface {
	ListEventAggregates(ctx context.Context, scope auth.Scope, query EventAggregateQuery) (EventAggregatePage, error)
	CreateEventAggregate(ctx context.Context, ownerID int64, input CreateEventAggregateInput) (EventAggregate, error)
	FindEventAggregateByRefCode(ctx context.Context, scope auth.Scope, refCode string) (EventAggregate, error)
	LockEventAggregateByRefCode(ctx context.Context, refCode string) (EventAggregate, error)
	DeleteEventAggregate(ctx context.Context, ownerID int64, aggregateID int64) error

	ListViewEvents(ctx context.Context, scope auth.Scope, query CalendarViewQuery) (EventPage, error)
	CreateEvent(ctx context.Context, ownerID int64, aggregateID int64, input CreateEventInput) (Event, error)
	ListEventsForAggregate(ctx context.Context, ownerID int64, aggregateID int64) ([]Event, error)
	ListEventIDsForAggregate(ctx context.Context, ownerID int64, aggregateID int64) ([]int64, error)
	FindEventByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Event, error)
	LockEventByRefCode(ctx context.Context, refCode string) (Event, error)
	FinishEvent(ctx context.Context, event Event) (Event, error)
	VoidEvent(ctx context.Context, event Event) (Event, error)
}
