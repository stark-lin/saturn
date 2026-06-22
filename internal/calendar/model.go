// This file defines Calendar event aggregate domain models.
package calendar

import "time"

type EventStatus string

const (
	EventStatusScheduled EventStatus = "scheduled"
	EventStatusFinished  EventStatus = "finished"
	EventStatusVoided    EventStatus = "voided"

	EventAggregateStatusActive = "active"
)

type RecurrenceKind string

const (
	RecurrenceKindSingle RecurrenceKind = "single"
	RecurrenceKindWeekly RecurrenceKind = "weekly"
)

type Weekday string

const (
	WeekdayMonday    Weekday = "mon"
	WeekdayTuesday   Weekday = "tue"
	WeekdayWednesday Weekday = "wed"
	WeekdayThursday  Weekday = "thu"
	WeekdayFriday    Weekday = "fri"
	WeekdaySaturday  Weekday = "sat"
	WeekdaySunday    Weekday = "sun"
)

type EventAggregateMetadata struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Timezone    string `json:"timezone"`
}

type EventMetadata struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
}

type EventAggregate struct {
	ID          int64
	OwnerID     int64
	ObjectRefID int64
	RefCode     string
	Metadata    EventAggregateMetadata
	Tags        []string
	CreatedAt   time.Time
}

type Event struct {
	ID               int64
	OwnerID          int64
	AggregateID      int64
	AggregateRefCode string
	ObjectRefID      int64
	RefCode          string
	StartsAt         time.Time
	DurationMinutes  int
	Metadata         EventMetadata
	Status           EventStatus
	Tags             []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type EventAggregateDetail struct {
	Aggregate EventAggregate
	Events    []Event
}

type CalendarView struct {
	From    time.Time
	To      time.Time
	Events  []Event
	Limit   int
	Offset  int
	HasMore bool
}

type RecurrenceInput struct {
	Kind      RecurrenceKind
	Weekdays  []Weekday
	WeekCount int
}

type CreateEventAggregateInput struct {
	Metadata EventAggregateMetadata
	Tags     []string
}

type CreateEventInput struct {
	Metadata        EventMetadata
	Tags            []string
	StartsAt        time.Time
	DurationMinutes int
	Recurrence      RecurrenceInput
}
