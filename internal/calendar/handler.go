// This file defines Calendar HTTP handler dependencies.
package calendar

import (
	"context"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

type EventService interface {
	ListEventAggregates(ctx context.Context, actor auth.Principal, query EventAggregateQuery) (EventAggregatePage, error)
	CreateEventAggregate(ctx context.Context, actor auth.Principal, input CreateEventAggregateInput) (EventAggregateDetail, error)
	CreateEvent(ctx context.Context, actor auth.Principal, aggregateRefCode string, input CreateEventInput) (EventAggregateDetail, error)
	GetEventAggregate(ctx context.Context, actor auth.Principal, refCode string) (EventAggregateDetail, error)
	DeleteEventAggregate(ctx context.Context, actor auth.Principal, refCode string) error
	CalendarView(ctx context.Context, actor auth.Principal, query CalendarViewQuery) (CalendarView, error)
	GetEvent(ctx context.Context, actor auth.Principal, refCode string) (Event, error)
	FinishEvent(ctx context.Context, actor auth.Principal, refCode string) (Event, error)
	VoidEvent(ctx context.Context, actor auth.Principal, refCode string) (Event, error)
}

type Handler struct {
	service EventService
}

func NewHandler(service EventService) *Handler {
	return &Handler{service: service}
}
