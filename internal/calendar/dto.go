// This file defines Calendar HTTP request and response payloads.
package calendar

import "time"

type CreateEventAggregateRequest struct {
	Metadata EventAggregateMetadataRequest `json:"metadata"`
	Tags     []string                      `json:"tags"`
}

type CreateEventRequest struct {
	Metadata        EventMetadataRequest `json:"metadata"`
	Tags            []string             `json:"tags"`
	StartsAt        string               `json:"starts_at"`
	DurationMinutes int                  `json:"duration_minutes"`
	Recurrence      RecurrenceRequest    `json:"recurrence"`
}

type EventAggregateMetadataRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Timezone    string `json:"timezone"`
}

type EventMetadataRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
}

type RecurrenceRequest struct {
	Kind      RecurrenceKind `json:"kind"`
	Weekdays  []Weekday      `json:"weekdays"`
	WeekCount int            `json:"week_count"`
}

type EventAggregateMetadataDetail struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Timezone    string `json:"timezone"`
}

type EventMetadataDetail struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
}

type EventAggregateDetailPayload struct {
	RefCode   string                       `json:"ref_code"`
	Metadata  EventAggregateMetadataDetail `json:"metadata"`
	Tags      []string                     `json:"tags"`
	CreatedAt time.Time                    `json:"created_at"`
}

type EventDetail struct {
	RefCode          string              `json:"ref_code"`
	AggregateRefCode string              `json:"aggregate_ref_code"`
	StartsAt         time.Time           `json:"starts_at"`
	DurationMinutes  int                 `json:"duration_minutes"`
	Metadata         EventMetadataDetail `json:"metadata"`
	Status           EventStatus         `json:"status"`
	Tags             []string            `json:"tags"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

type EventAggregateResponse struct {
	Aggregate EventAggregateDetailPayload `json:"aggregate"`
	Events    []EventDetail               `json:"events"`
}

type EventAggregatesResponse struct {
	Aggregates []EventAggregateDetailPayload `json:"aggregates"`
	Pagination Pagination                    `json:"pagination"`
}

type EventResponse struct {
	Event EventDetail `json:"event"`
}

type CalendarViewResponse struct {
	From       time.Time     `json:"from"`
	To         time.Time     `json:"to"`
	Events     []EventDetail `json:"events"`
	Pagination Pagination    `json:"pagination"`
}

func eventAggregateResponse(detail EventAggregateDetail) EventAggregateResponse {
	return EventAggregateResponse{
		Aggregate: eventAggregateDetail(detail.Aggregate),
		Events:    eventDetails(detail.Events),
	}
}

func eventAggregatesResponse(page EventAggregatePage) EventAggregatesResponse {
	aggregates := make([]EventAggregateDetailPayload, 0, len(page.Aggregates))
	for _, aggregate := range page.Aggregates {
		aggregates = append(aggregates, eventAggregateDetail(aggregate))
	}
	return EventAggregatesResponse{
		Aggregates: aggregates,
		Pagination: Pagination{Limit: page.Limit, Offset: page.Offset, HasMore: page.HasMore},
	}
}

func eventResponse(event Event) EventResponse {
	return EventResponse{Event: eventDetail(event)}
}

func calendarViewResponse(view CalendarView) CalendarViewResponse {
	return CalendarViewResponse{
		From: view.From, To: view.To, Events: eventDetails(view.Events),
		Pagination: Pagination{Limit: view.Limit, Offset: view.Offset, HasMore: view.HasMore},
	}
}

func eventAggregateDetail(aggregate EventAggregate) EventAggregateDetailPayload {
	tags := aggregate.Tags
	if tags == nil {
		tags = []string{}
	}
	return EventAggregateDetailPayload{
		RefCode: aggregate.RefCode,
		Metadata: EventAggregateMetadataDetail{
			Title: aggregate.Metadata.Title, Description: aggregate.Metadata.Description,
			Location: aggregate.Metadata.Location, Timezone: aggregate.Metadata.Timezone,
		},
		Tags:      tags,
		CreatedAt: aggregate.CreatedAt,
	}
}

func eventDetails(events []Event) []EventDetail {
	details := make([]EventDetail, 0, len(events))
	for _, event := range events {
		details = append(details, eventDetail(event))
	}
	return details
}

func eventDetail(event Event) EventDetail {
	tags := event.Tags
	if tags == nil {
		tags = []string{}
	}
	return EventDetail{
		RefCode: event.RefCode, AggregateRefCode: event.AggregateRefCode,
		StartsAt: event.StartsAt, DurationMinutes: event.DurationMinutes,
		Metadata: EventMetadataDetail{
			Title: event.Metadata.Title, Description: event.Metadata.Description, Location: event.Metadata.Location,
		},
		Status: event.Status, Tags: tags, CreatedAt: event.CreatedAt, UpdatedAt: event.UpdatedAt,
	}
}

func (r CreateEventAggregateRequest) input() CreateEventAggregateInput {
	return CreateEventAggregateInput{
		Metadata: EventAggregateMetadata{
			Title: r.Metadata.Title, Description: r.Metadata.Description,
			Location: r.Metadata.Location, Timezone: r.Metadata.Timezone,
		},
		Tags: r.Tags,
	}
}

func (r CreateEventRequest) input(startsAt time.Time) CreateEventInput {
	return CreateEventInput{
		Metadata: EventMetadata{
			Title: r.Metadata.Title, Description: r.Metadata.Description, Location: r.Metadata.Location,
		},
		Tags:            r.Tags,
		StartsAt:        startsAt,
		DurationMinutes: r.DurationMinutes,
		Recurrence: RecurrenceInput{
			Kind: r.Recurrence.Kind, Weekdays: r.Recurrence.Weekdays, WeekCount: r.Recurrence.WeekCount,
		},
	}
}
