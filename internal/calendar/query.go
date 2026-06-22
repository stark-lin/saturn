// This file defines Calendar list and range query parameters.
package calendar

import "time"

const (
	DefaultLimit = 25
	MaxLimit     = 100
)

type EventAggregateQuery struct {
	Limit  int
	Offset int
}

type CalendarViewQuery struct {
	From   time.Time
	To     time.Time
	Limit  int
	Offset int
}

type EventAggregatePage struct {
	Aggregates []EventAggregate
	Limit      int
	Offset     int
	HasMore    bool
}

type EventPage struct {
	Events  []Event
	Limit   int
	Offset  int
	HasMore bool
}
