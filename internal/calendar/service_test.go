// This file tests Calendar event aggregate service invariants.
package calendar

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stark-lin/saturn/internal/platform/audit"
	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

func TestNewModuleBuildsCalendarDependencies(t *testing.T) {
	module := NewModule(nil, nil, nil, nil)
	if module.Service == nil || module.Handler == nil {
		t.Fatal("expected calendar service and handler")
	}
}

func TestServiceCreatesEmptyEventAggregateWithRefsAndTags(t *testing.T) {
	service, _, references, audits := newTestService()

	detail, err := service.CreateEventAggregate(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, CreateEventAggregateInput{
		Metadata: EventAggregateMetadata{Title: " Sprint ", Description: " Planning "},
		Tags:     []string{" work ", "calendar", "work"},
	})
	if err != nil {
		t.Fatalf("create event aggregate: %v", err)
	}
	if detail.Aggregate.RefCode != "CAL-00000001" || detail.Aggregate.ObjectRefID == 0 {
		t.Fatalf("aggregate reference = %#v", detail.Aggregate)
	}
	if len(detail.Events) != 0 {
		t.Fatalf("created events = %#v", detail.Events)
	}
	if len(references.registrations) != 1 || references.registrations[0].ObjectType != ref.ObjectTypeEventAggregate {
		t.Fatalf("reference registrations = %#v", references.registrations)
	}
	if len(references.registrations[0].Tags) != 2 ||
		references.registrations[0].Tags[0] != "work" ||
		references.registrations[0].Tags[1] != "calendar" {
		t.Fatalf("registration tags = %#v", references.registrations)
	}
	if len(audits.successes) != 1 || audits.successes[0].TargetRefCode != detail.Aggregate.RefCode {
		t.Fatalf("audit successes = %#v", audits.successes)
	}
}

func TestServiceCreatesEventUnderAggregateWithRefsDurationAndTags(t *testing.T) {
	service, repo, references, audits := newTestService()
	aggregate, err := service.CreateEventAggregate(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, CreateEventAggregateInput{
		Metadata: EventAggregateMetadata{Title: "Sprint"},
		Tags:     []string{" work "},
	})
	if err != nil {
		t.Fatalf("create event aggregate: %v", err)
	}
	repo.storeAggregate(aggregate.Aggregate)
	startsAt := time.Date(2026, time.June, 1, 9, 30, 0, 0, time.UTC)

	detail, err := service.CreateEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, aggregate.Aggregate.RefCode, CreateEventInput{
		Metadata:        EventMetadata{Title: "Planning"},
		Tags:            []string{" meeting ", "meeting"},
		StartsAt:        startsAt,
		DurationMinutes: 45,
		Recurrence:      RecurrenceInput{Kind: RecurrenceKindSingle},
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if detail.Aggregate.RefCode != aggregate.Aggregate.RefCode {
		t.Fatalf("detail aggregate = %#v", detail.Aggregate)
	}
	if len(detail.Events) != 1 || detail.Events[0].RefCode != "CAL-00000002" ||
		detail.Events[0].AggregateRefCode != detail.Aggregate.RefCode ||
		detail.Events[0].DurationMinutes != 45 {
		t.Fatalf("created events = %#v", detail.Events)
	}
	if references.registrations[0].ObjectType != ref.ObjectTypeEventAggregate ||
		references.registrations[1].ObjectType != ref.ObjectTypeEvent {
		t.Fatalf("reference registrations = %#v", references.registrations)
	}
	if len(references.registrations[0].Tags) != 1 ||
		references.registrations[0].Tags[0] != "work" ||
		len(references.registrations[1].Tags) != 1 ||
		references.registrations[1].Tags[0] != "meeting" {
		t.Fatalf("registration tags = %#v", references.registrations)
	}
	if len(audits.successes) != 2 || audits.successes[1].TargetRefCode != detail.Events[0].RefCode {
		t.Fatalf("audit successes = %#v", audits.successes)
	}
}

func TestServiceExpandsWeeklyEventsByWeekdayAndWeekCount(t *testing.T) {
	service, repo, _, _ := newTestService()
	startsAt := time.Date(2026, time.June, 1, 9, 0, 0, 0, time.UTC)
	aggregate, err := service.CreateEventAggregate(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, CreateEventAggregateInput{
		Metadata: EventAggregateMetadata{Title: "Training"},
	})
	if err != nil {
		t.Fatalf("create aggregate: %v", err)
	}
	repo.storeAggregate(aggregate.Aggregate)

	detail, err := service.CreateEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, aggregate.Aggregate.RefCode, CreateEventInput{
		Metadata:        EventMetadata{Title: "Training block"},
		StartsAt:        startsAt,
		DurationMinutes: 60,
		Recurrence: RecurrenceInput{
			Kind:      RecurrenceKindWeekly,
			Weekdays:  []Weekday{WeekdayWednesday, WeekdayMonday, WeekdayMonday},
			WeekCount: 2,
		},
	})
	if err != nil {
		t.Fatalf("create weekly aggregate: %v", err)
	}
	if len(detail.Events) != 4 {
		t.Fatalf("weekly event count = %d, want 4: %#v", len(detail.Events), detail.Events)
	}
	got := make([]string, 0, len(detail.Events))
	for _, event := range detail.Events {
		got = append(got, event.StartsAt.Format(time.DateOnly))
	}
	want := []string{"2026-06-01", "2026-06-03", "2026-06-08", "2026-06-10"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("weekly dates = %#v, want %#v", got, want)
		}
	}
}

func TestServiceVoidsEventAndKeepsItOutOfMainViewButInAggregateEvents(t *testing.T) {
	service, repo, references, audits := newTestService()
	aggregate := EventAggregate{ID: 1, OwnerID: 7, RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Shift"}}
	event := Event{
		ID: 2, OwnerID: 7, AggregateID: 1, AggregateRefCode: aggregate.RefCode, RefCode: "CAL-00000002",
		StartsAt: time.Date(2026, time.June, 2, 10, 0, 0, 0, time.UTC), DurationMinutes: 30,
		Metadata: EventMetadata{Title: "Shift block"}, Status: EventStatusScheduled,
	}
	repo.storeAggregate(aggregate)
	repo.storeEvent(event)

	voided, err := service.VoidEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, event.RefCode)
	if err != nil {
		t.Fatalf("void event: %v", err)
	}
	if voided.Status != EventStatusVoided || references.updates[0].Status != string(EventStatusVoided) {
		t.Fatalf("voided event = %#v updates = %#v", voided, references.updates)
	}
	if audits.successes[0].Reason != "void" {
		t.Fatalf("void audit = %#v", audits.successes[0])
	}

	view, err := service.CalendarView(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, CalendarViewQuery{
		From: event.StartsAt.Add(-time.Hour), To: event.StartsAt.Add(time.Hour), Limit: 25,
	})
	if err != nil {
		t.Fatalf("calendar view: %v", err)
	}
	if len(view.Events) != 0 {
		t.Fatalf("voided event appeared in main view: %#v", view.Events)
	}
	detail, err := service.GetEventAggregate(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, aggregate.RefCode)
	if err != nil {
		t.Fatalf("get aggregate: %v", err)
	}
	if len(detail.Events) != 1 || detail.Events[0].Status != EventStatusVoided {
		t.Fatalf("aggregate events = %#v, want voided child retained", detail.Events)
	}
	if _, err := service.VoidEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, event.RefCode); !errors.Is(err, ErrEventAlreadyVoided) {
		t.Fatalf("second void error = %v, want already voided", err)
	}
}

func TestServiceFinishesEventAndKeepsItOutOfMainViewButInAggregateEvents(t *testing.T) {
	service, repo, references, audits := newTestService()
	aggregate := EventAggregate{ID: 1, OwnerID: 7, RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Shift"}}
	event := Event{
		ID: 2, OwnerID: 7, AggregateID: 1, AggregateRefCode: aggregate.RefCode, RefCode: "CAL-00000002",
		StartsAt: time.Date(2026, time.June, 2, 10, 0, 0, 0, time.UTC), DurationMinutes: 30,
		Metadata: EventMetadata{Title: "Shift block"}, Status: EventStatusScheduled,
	}
	repo.storeAggregate(aggregate)
	repo.storeEvent(event)

	finished, err := service.FinishEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, event.RefCode)
	if err != nil {
		t.Fatalf("finish event: %v", err)
	}
	if finished.Status != EventStatusFinished || references.updates[0].Status != string(EventStatusFinished) {
		t.Fatalf("finished event = %#v updates = %#v", finished, references.updates)
	}
	if audits.successes[0].Reason != "finish" {
		t.Fatalf("finish audit = %#v", audits.successes[0])
	}

	view, err := service.CalendarView(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, CalendarViewQuery{
		From: event.StartsAt.Add(-time.Hour), To: event.StartsAt.Add(time.Hour), Limit: 25,
	})
	if err != nil {
		t.Fatalf("calendar view: %v", err)
	}
	if len(view.Events) != 0 {
		t.Fatalf("finished event appeared in main view: %#v", view.Events)
	}
	detail, err := service.GetEventAggregate(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, aggregate.RefCode)
	if err != nil {
		t.Fatalf("get aggregate: %v", err)
	}
	if len(detail.Events) != 1 || detail.Events[0].Status != EventStatusFinished {
		t.Fatalf("aggregate events = %#v, want finished child retained", detail.Events)
	}
	if _, err := service.FinishEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, event.RefCode); !errors.Is(err, ErrEventAlreadyFinished) {
		t.Fatalf("second finish error = %v, want already finished", err)
	}
}

func TestServiceVoidsFinishedEvent(t *testing.T) {
	service, repo, references, audits := newTestService()
	aggregate := EventAggregate{ID: 1, OwnerID: 7, RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Shift"}}
	event := Event{
		ID: 2, OwnerID: 7, AggregateID: 1, AggregateRefCode: aggregate.RefCode, RefCode: "CAL-00000002",
		StartsAt: time.Date(2026, time.June, 2, 10, 0, 0, 0, time.UTC), DurationMinutes: 30,
		Metadata: EventMetadata{Title: "Shift block"}, Status: EventStatusFinished,
	}
	repo.storeAggregate(aggregate)
	repo.storeEvent(event)

	voided, err := service.VoidEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, event.RefCode)
	if err != nil {
		t.Fatalf("void finished event: %v", err)
	}
	if voided.Status != EventStatusVoided || references.updates[0].Status != string(EventStatusVoided) {
		t.Fatalf("voided event = %#v updates = %#v", voided, references.updates)
	}
	if audits.successes[0].Reason != "void" {
		t.Fatalf("void audit = %#v", audits.successes[0])
	}
}

func TestServiceRejectsFinishForVoidedEvent(t *testing.T) {
	service, repo, _, _ := newTestService()
	aggregate := EventAggregate{ID: 1, OwnerID: 7, RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Shift"}}
	event := Event{ID: 2, OwnerID: 7, AggregateID: 1, RefCode: "CAL-00000002", Status: EventStatusVoided}
	repo.storeAggregate(aggregate)
	repo.storeEvent(event)

	if _, err := service.FinishEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, event.RefCode); !errors.Is(err, ErrEventAlreadyVoided) {
		t.Fatalf("finish voided error = %v, want already voided", err)
	}
}

func TestServiceDeletesEventAggregateAndAllEventReferences(t *testing.T) {
	service, repo, references, audits := newTestService()
	aggregate := EventAggregate{ID: 1, OwnerID: 7, RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Roster"}}
	repo.storeAggregate(aggregate)
	repo.storeEvent(Event{ID: 2, OwnerID: 7, AggregateID: 1, RefCode: "CAL-00000002", Status: EventStatusScheduled})
	repo.storeEvent(Event{ID: 3, OwnerID: 7, AggregateID: 1, RefCode: "CAL-00000003", Status: EventStatusVoided})

	if err := service.DeleteEventAggregate(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, aggregate.RefCode); err != nil {
		t.Fatalf("delete event aggregate: %v", err)
	}
	if _, exists := repo.aggregates[aggregate.ID]; exists || len(repo.events) != 0 {
		t.Fatalf("deleted aggregate remained: aggregates %#v events %#v", repo.aggregates, repo.events)
	}
	if len(references.deletes) != 3 ||
		references.deletes[0].objectType != ref.ObjectTypeEvent ||
		references.deletes[2].objectType != ref.ObjectTypeEventAggregate {
		t.Fatalf("deleted references = %#v", references.deletes)
	}
	if len(audits.successes) != 3 ||
		audits.successes[0].TargetRefCode != "CAL-00000002" ||
		audits.successes[0].Reason != "cascade_event_aggregate" ||
		audits.successes[2].TargetRefCode != aggregate.RefCode {
		t.Fatalf("delete audit successes = %#v", audits.successes)
	}
}

func TestServiceRejectsInvalidWeeklyInputBeforeWriting(t *testing.T) {
	service, repo, _, _ := newTestService()
	aggregate := EventAggregate{ID: 1, OwnerID: 7, RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Bad recurrence"}}
	repo.storeAggregate(aggregate)
	_, err := service.CreateEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, aggregate.RefCode, CreateEventInput{
		Metadata:        EventMetadata{Title: "Bad event"},
		StartsAt:        time.Now(),
		DurationMinutes: 30,
		Recurrence: RecurrenceInput{
			Kind:      RecurrenceKindWeekly,
			Weekdays:  []Weekday{"funday"},
			WeekCount: 1,
		},
	})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("invalid weekly input error = %v", err)
	}
	if len(repo.aggregates) != 1 || len(repo.events) != 0 {
		t.Fatalf("invalid input wrote repository state: %#v %#v", repo.aggregates, repo.events)
	}
}

func TestServiceListsAggregatesAndGetsEvent(t *testing.T) {
	service, repo, _, _ := newTestService()
	aggregate := EventAggregate{ID: 1, OwnerID: 7, RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Shift"}}
	event := Event{
		ID: 2, OwnerID: 7, AggregateID: aggregate.ID, AggregateRefCode: aggregate.RefCode, RefCode: "CAL-00000002",
		StartsAt: time.Date(2026, time.June, 2, 10, 0, 0, 0, time.UTC), DurationMinutes: 30,
		Metadata: EventMetadata{Title: "Shift block"}, Status: EventStatusScheduled,
	}
	repo.storeAggregate(aggregate)
	repo.storeEvent(event)

	page, err := service.ListEventAggregates(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, EventAggregateQuery{Limit: 5})
	if err != nil {
		t.Fatalf("list aggregates: %v", err)
	}
	if len(page.Aggregates) != 1 || page.Aggregates[0].RefCode != aggregate.RefCode || page.Limit != 5 {
		t.Fatalf("aggregate page = %#v", page)
	}
	gotEvent, err := service.GetEvent(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, "CAL-00000002")
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if gotEvent.RefCode != event.RefCode || gotEvent.Metadata.Title != "Shift block" {
		t.Fatalf("event = %#v", gotEvent)
	}
	if _, err := service.GetEvent(context.Background(), auth.Principal{ID: 8, Role: auth.RoleUser}, "CAL-00000002"); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("other owner get error = %v, want not found", err)
	}
}

func newTestService() (*Service, *fakeRepository, *fakeReferences, *fakeAudits) {
	repo := &fakeRepository{aggregates: make(map[int64]EventAggregate), events: make(map[int64]Event)}
	references := &fakeReferences{}
	audits := &fakeAudits{}
	return NewService(repo, nil, references, audits), repo, references, audits
}

type fakeRepository struct {
	nextID     int64
	aggregates map[int64]EventAggregate
	events     map[int64]Event
}

func (r *fakeRepository) storeAggregate(aggregate EventAggregate) {
	r.aggregates[aggregate.ID] = aggregate
}

func (r *fakeRepository) storeEvent(event Event) {
	r.events[event.ID] = event
}

func (r *fakeRepository) ListEventAggregates(_ context.Context, scope auth.Scope, query EventAggregateQuery) (EventAggregatePage, error) {
	aggregates := make([]EventAggregate, 0, len(r.aggregates))
	for _, aggregate := range r.aggregates {
		if scope.All || aggregate.OwnerID == scope.OwnerID {
			aggregates = append(aggregates, aggregate)
		}
	}
	sort.Slice(aggregates, func(i, j int) bool { return aggregates[i].ID > aggregates[j].ID })
	return EventAggregatePage{Aggregates: aggregates, Limit: query.Limit, Offset: query.Offset}, nil
}

func (r *fakeRepository) CreateEventAggregate(_ context.Context, ownerID int64, input CreateEventAggregateInput) (EventAggregate, error) {
	r.nextID++
	aggregate := EventAggregate{ID: r.nextID, OwnerID: ownerID, Metadata: input.Metadata, Tags: input.Tags, CreatedAt: time.Now().UTC()}
	r.aggregates[aggregate.ID] = aggregate
	return aggregate, nil
}

func (r *fakeRepository) FindEventAggregateByRefCode(_ context.Context, scope auth.Scope, refCode string) (EventAggregate, error) {
	for _, aggregate := range r.aggregates {
		if aggregate.RefCode == refCode && (scope.All || aggregate.OwnerID == scope.OwnerID) {
			return aggregate, nil
		}
	}
	return EventAggregate{}, ErrEventAggregateNotFound
}

func (r *fakeRepository) LockEventAggregateByRefCode(ctx context.Context, refCode string) (EventAggregate, error) {
	return r.FindEventAggregateByRefCode(ctx, auth.Scope{All: true}, refCode)
}

func (r *fakeRepository) DeleteEventAggregate(_ context.Context, ownerID int64, aggregateID int64) error {
	aggregate, exists := r.aggregates[aggregateID]
	if !exists || aggregate.OwnerID != ownerID {
		return ErrEventAggregateNotFound
	}
	delete(r.aggregates, aggregateID)
	for id, event := range r.events {
		if event.AggregateID == aggregateID {
			delete(r.events, id)
		}
	}
	return nil
}

func (r *fakeRepository) ListViewEvents(_ context.Context, scope auth.Scope, query CalendarViewQuery) (EventPage, error) {
	events := make([]Event, 0, len(r.events))
	for _, event := range r.events {
		if !scope.All && event.OwnerID != scope.OwnerID {
			continue
		}
		if event.Status != EventStatusScheduled || event.StartsAt.Before(query.From) || event.StartsAt.After(query.To) {
			continue
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].StartsAt.Before(events[j].StartsAt) })
	return EventPage{Events: events, Limit: query.Limit, Offset: query.Offset}, nil
}

func (r *fakeRepository) CreateEvent(_ context.Context, ownerID int64, aggregateID int64, input CreateEventInput) (Event, error) {
	r.nextID++
	aggregate := r.aggregates[aggregateID]
	event := Event{
		ID: r.nextID, OwnerID: ownerID, AggregateID: aggregateID, AggregateRefCode: aggregate.RefCode,
		StartsAt: input.StartsAt, DurationMinutes: input.DurationMinutes,
		Metadata: input.Metadata, Status: EventStatusScheduled, Tags: input.Tags, CreatedAt: time.Now().UTC(),
	}
	r.events[event.ID] = event
	return event, nil
}

func (r *fakeRepository) ListEventsForAggregate(_ context.Context, ownerID int64, aggregateID int64) ([]Event, error) {
	events := make([]Event, 0)
	for _, event := range r.events {
		if event.OwnerID == ownerID && event.AggregateID == aggregateID {
			events = append(events, event)
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].StartsAt.Equal(events[j].StartsAt) {
			return events[i].ID < events[j].ID
		}
		return events[i].StartsAt.Before(events[j].StartsAt)
	})
	return events, nil
}

func (r *fakeRepository) ListEventIDsForAggregate(_ context.Context, ownerID int64, aggregateID int64) ([]int64, error) {
	ids := make([]int64, 0)
	for id, event := range r.events {
		if event.OwnerID == ownerID && event.AggregateID == aggregateID {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (r *fakeRepository) FindEventByRefCode(_ context.Context, scope auth.Scope, refCode string) (Event, error) {
	for _, event := range r.events {
		if event.RefCode == refCode && (scope.All || event.OwnerID == scope.OwnerID) {
			return event, nil
		}
	}
	return Event{}, ErrEventNotFound
}

func (r *fakeRepository) LockEventByRefCode(ctx context.Context, refCode string) (Event, error) {
	return r.FindEventByRefCode(ctx, auth.Scope{All: true}, refCode)
}

func (r *fakeRepository) FinishEvent(_ context.Context, event Event) (Event, error) {
	if event.Status != EventStatusScheduled {
		return Event{}, ErrEventAlreadyFinished
	}
	event.Status = EventStatusFinished
	event.UpdatedAt = time.Now().UTC()
	r.events[event.ID] = event
	return event, nil
}

func (r *fakeRepository) VoidEvent(_ context.Context, event Event) (Event, error) {
	if event.Status == EventStatusVoided {
		return Event{}, ErrEventAlreadyVoided
	}
	event.Status = EventStatusVoided
	event.UpdatedAt = time.Now().UTC()
	r.events[event.ID] = event
	return event, nil
}

type fakeReferences struct {
	sequence      int64
	registrations []ref.Registration
	updates       []ref.ProjectionUpdate
	deletes       []fakeReferenceDelete
}

type fakeReferenceDelete struct {
	objectType ref.ObjectType
	objectID   int64
}

func (r *fakeReferences) ClaimCode(_ context.Context, _ ref.ObjectType) (string, error) {
	r.sequence++
	return fmt.Sprintf("CAL-%08X", r.sequence), nil
}

func (r *fakeReferences) Register(_ context.Context, registration ref.Registration) (ref.ObjectRef, error) {
	r.registrations = append(r.registrations, registration)
	return ref.ObjectRef{ID: int64(len(r.registrations)), RefCode: registration.RefCode, Status: registration.Status}, nil
}

func (r *fakeReferences) UpdateProjection(_ context.Context, update ref.ProjectionUpdate) (ref.ObjectRef, error) {
	r.updates = append(r.updates, update)
	return ref.ObjectRef{Status: update.Status}, nil
}

func (r *fakeReferences) Delete(_ context.Context, _ int64, objectType ref.ObjectType, objectID int64) error {
	r.deletes = append(r.deletes, fakeReferenceDelete{objectType: objectType, objectID: objectID})
	return nil
}

type fakeAudits struct {
	successes []audit.Event
	failures  []audit.Event
}

func (a *fakeAudits) Record(_ context.Context, event audit.Event) (audit.Event, error) {
	a.successes = append(a.successes, event)
	return event, nil
}

func (a *fakeAudits) RecordStandalone(_ context.Context, event audit.Event) error {
	a.failures = append(a.failures, event)
	return nil
}
