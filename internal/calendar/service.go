// This file enforces Calendar event aggregate business boundaries.
package calendar

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/stark-lin/saturn/internal/platform/audit"
	"github.com/stark-lin/saturn/internal/platform/auth"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

var (
	ErrInvalidEventAggregate = errors.New("invalid event aggregate")
	ErrInvalidEvent          = errors.New("invalid event")
	ErrInvalidQuery          = errors.New("invalid calendar query")
)

const maxRecurrenceWeekCount = 520

type ObjectReferenceService interface {
	ClaimCode(ctx context.Context, objectType ref.ObjectType) (string, error)
	Register(ctx context.Context, registration ref.Registration) (ref.ObjectRef, error)
	UpdateProjection(ctx context.Context, update ref.ProjectionUpdate) (ref.ObjectRef, error)
	Delete(ctx context.Context, ownerID int64, objectType ref.ObjectType, objectID int64) error
}

type AuditService interface {
	Record(ctx context.Context, event audit.Event) (audit.Event, error)
	RecordStandalone(ctx context.Context, event audit.Event) error
}

type Service struct {
	repo         Repository
	transactions platformdb.TransactionRunner
	references   ObjectReferenceService
	audit        AuditService
	authorizer   *auth.Authorizer
}

func NewService(
	repo Repository,
	transactions platformdb.TransactionRunner,
	references ObjectReferenceService,
	auditService AuditService,
) *Service {
	if transactions == nil {
		transactions = platformdb.NoopTransactionRunner{}
	}
	return &Service{
		repo: repo, transactions: transactions, references: references, audit: auditService,
		authorizer: auth.NewAuthorizer(),
	}
}

func (s *Service) ListEventAggregates(ctx context.Context, actor auth.Principal, query EventAggregateQuery) (EventAggregatePage, error) {
	return s.repo.ListEventAggregates(ctx, auth.ScopeForPrincipal(actor), query)
}

func (s *Service) CreateEventAggregate(ctx context.Context, actor auth.Principal, input CreateEventAggregateInput) (EventAggregateDetail, error) {
	input, err := normalizeCreateEventAggregateInput(input)
	if err != nil {
		return EventAggregateDetail{}, err
	}

	aggregateRefCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeEventAggregate)
	if err != nil {
		return EventAggregateDetail{}, err
	}

	var created EventAggregateDetail
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		aggregate, err := s.repo.CreateEventAggregate(txCtx, actor.ID, input)
		if err != nil {
			return err
		}
		aggregateRef, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: actor.ID, RefCode: aggregateRefCode, ObjectType: ref.ObjectTypeEventAggregate,
			ObjectID: aggregate.ID, Title: aggregateProjectionTitle(aggregate), Tags: input.Tags, Status: EventAggregateStatusActive,
		})
		if err != nil {
			return err
		}

		aggregate.ObjectRefID = aggregateRef.ID
		aggregate.RefCode = aggregateRef.RefCode
		aggregate.Tags = input.Tags
		created.Aggregate = aggregate
		created.Events = []Event{}

		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
			TargetRefCode: aggregate.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return EventAggregateDetail{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, aggregateRefCode, err)
	}
	return created, nil
}

func (s *Service) CreateEvent(ctx context.Context, actor auth.Principal, aggregateRefCode string, input CreateEventInput) (EventAggregateDetail, error) {
	aggregateRefCode = ref.NormalizeCode(aggregateRefCode)
	if !ref.ValidCode(aggregateRefCode) || !ref.CodeMatchesObjectType(aggregateRefCode, ref.ObjectTypeEventAggregate) {
		return EventAggregateDetail{}, ErrInvalidEvent
	}
	input, err := normalizeCreateEventInput(input)
	if err != nil {
		return EventAggregateDetail{}, err
	}
	eventInputs, err := expandEventInputs(input)
	if err != nil {
		return EventAggregateDetail{}, err
	}

	eventRefCodes := make([]string, 0, len(eventInputs))
	for range eventInputs {
		refCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeEvent)
		if err != nil {
			return EventAggregateDetail{}, err
		}
		eventRefCodes = append(eventRefCodes, refCode)
	}

	var created EventAggregateDetail
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		aggregate, err := s.repo.LockEventAggregateByRefCode(txCtx, aggregateRefCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionUpdate, "event_aggregate", aggregate.ID, aggregate.OwnerID); err != nil {
			return err
		}

		created.Aggregate = aggregate
		created.Events = make([]Event, 0, len(eventInputs))
		for index, eventInput := range eventInputs {
			event, err := s.repo.CreateEvent(txCtx, aggregate.OwnerID, aggregate.ID, eventInput)
			if err != nil {
				return err
			}
			eventRef, err := s.references.Register(txCtx, ref.Registration{
				OwnerID: aggregate.OwnerID, RefCode: eventRefCodes[index], ObjectType: ref.ObjectTypeEvent,
				ObjectID: event.ID, Title: eventProjectionTitle(event), Tags: eventInput.Tags, Status: string(EventStatusScheduled),
			})
			if err != nil {
				return err
			}
			event.ObjectRefID = eventRef.ID
			event.RefCode = eventRef.RefCode
			event.AggregateRefCode = aggregate.RefCode
			event.Status = EventStatusScheduled
			event.Tags = eventInput.Tags
			if _, err := s.audit.Record(txCtx, audit.Event{
				ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
				TargetRefCode: event.RefCode, Result: audit.ResultSuccess,
			}); err != nil {
				return err
			}
			created.Events = append(created.Events, event)
		}
		return nil
	})
	if err != nil {
		return EventAggregateDetail{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, eventRefCodes[0], err)
	}
	return created, nil
}

func (s *Service) GetEventAggregate(ctx context.Context, actor auth.Principal, refCode string) (EventAggregateDetail, error) {
	aggregate, err := s.repo.FindEventAggregateByRefCode(ctx, auth.ScopeForPrincipal(actor), refCode)
	if err != nil {
		return EventAggregateDetail{}, err
	}
	events, err := s.repo.ListEventsForAggregate(ctx, aggregate.OwnerID, aggregate.ID)
	if err != nil {
		return EventAggregateDetail{}, err
	}
	return EventAggregateDetail{Aggregate: aggregate, Events: events}, nil
}

func (s *Service) DeleteEventAggregate(ctx context.Context, actor auth.Principal, refCode string) error {
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		aggregate, err := s.repo.LockEventAggregateByRefCode(txCtx, refCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionDelete, "event_aggregate", aggregate.ID, aggregate.OwnerID); err != nil {
			return err
		}
		events, err := s.repo.ListEventsForAggregate(txCtx, aggregate.OwnerID, aggregate.ID)
		if err != nil {
			return err
		}
		for _, event := range events {
			if _, err := s.audit.Record(txCtx, audit.Event{
				ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
				TargetRefCode: event.RefCode, Result: audit.ResultSuccess, Reason: "cascade_event_aggregate",
			}); err != nil {
				return err
			}
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
			TargetRefCode: aggregate.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		for _, event := range events {
			if err := s.references.Delete(txCtx, aggregate.OwnerID, ref.ObjectTypeEvent, event.ID); err != nil {
				return err
			}
		}
		if err := s.references.Delete(txCtx, aggregate.OwnerID, ref.ObjectTypeEventAggregate, aggregate.ID); err != nil {
			return err
		}
		return s.repo.DeleteEventAggregate(txCtx, aggregate.OwnerID, aggregate.ID)
	})
	if err != nil {
		return s.recordWriteFailure(ctx, actor, audit.ActionDelete, refCode, err)
	}
	return nil
}

func (s *Service) CalendarView(ctx context.Context, actor auth.Principal, query CalendarViewQuery) (CalendarView, error) {
	page, err := s.repo.ListViewEvents(ctx, auth.ScopeForPrincipal(actor), query)
	if err != nil {
		return CalendarView{}, err
	}
	return CalendarView{
		From: query.From, To: query.To, Events: page.Events,
		Limit: page.Limit, Offset: page.Offset, HasMore: page.HasMore,
	}, nil
}

func (s *Service) GetEvent(ctx context.Context, actor auth.Principal, refCode string) (Event, error) {
	return s.repo.FindEventByRefCode(ctx, auth.ScopeForPrincipal(actor), refCode)
}

func (s *Service) FinishEvent(ctx context.Context, actor auth.Principal, refCode string) (Event, error) {
	var finished Event
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		event, err := s.repo.LockEventByRefCode(txCtx, refCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionUpdate, "event", event.ID, event.OwnerID); err != nil {
			return err
		}
		if event.Status == EventStatusFinished {
			return ErrEventAlreadyFinished
		}
		if event.Status == EventStatusVoided {
			return ErrEventAlreadyVoided
		}
		event, err = s.repo.FinishEvent(txCtx, event)
		if err != nil {
			return err
		}
		if _, err := s.references.UpdateProjection(txCtx, ref.ProjectionUpdate{
			OwnerID: event.OwnerID, ObjectType: ref.ObjectTypeEvent, ObjectID: event.ID,
			Title: eventProjectionTitle(event), Tags: event.Tags, Status: string(EventStatusFinished),
		}); err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionUpdate,
			TargetRefCode: event.RefCode, Result: audit.ResultSuccess, Reason: "finish",
		}); err != nil {
			return err
		}
		finished = event
		return nil
	})
	if err != nil {
		return Event{}, s.recordWriteFailure(ctx, actor, audit.ActionUpdate, refCode, err)
	}
	return finished, nil
}

func (s *Service) VoidEvent(ctx context.Context, actor auth.Principal, refCode string) (Event, error) {
	var voided Event
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		event, err := s.repo.LockEventByRefCode(txCtx, refCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionUpdate, "event", event.ID, event.OwnerID); err != nil {
			return err
		}
		if event.Status == EventStatusVoided {
			return ErrEventAlreadyVoided
		}
		event, err = s.repo.VoidEvent(txCtx, event)
		if err != nil {
			return err
		}
		if _, err := s.references.UpdateProjection(txCtx, ref.ProjectionUpdate{
			OwnerID: event.OwnerID, ObjectType: ref.ObjectTypeEvent, ObjectID: event.ID,
			Title: eventProjectionTitle(event), Tags: event.Tags, Status: string(EventStatusVoided),
		}); err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionUpdate,
			TargetRefCode: event.RefCode, Result: audit.ResultSuccess, Reason: "void",
		}); err != nil {
			return err
		}
		voided = event
		return nil
	})
	if err != nil {
		return Event{}, s.recordWriteFailure(ctx, actor, audit.ActionUpdate, refCode, err)
	}
	return voided, nil
}

func (s *Service) recordWriteFailure(ctx context.Context, actor auth.Principal, action audit.Action, refCode string, operationErr error) error {
	result := audit.ResultFailed
	reason := "operation_failed"
	if errors.Is(operationErr, ErrEventAggregateNotFound) || errors.Is(operationErr, ErrEventNotFound) ||
		errors.Is(operationErr, auth.ErrForbidden) || errors.Is(operationErr, ref.ErrNotFound) {
		result = audit.ResultDenied
		reason = "not_found"
	}
	auditErr := s.audit.RecordStandalone(ctx, audit.Event{
		ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: action,
		TargetRefCode: refCode, Result: result, Reason: reason,
	})
	if auditErr != nil {
		return errors.Join(operationErr, auditErr)
	}
	return operationErr
}

func (s *Service) can(actor auth.Principal, action auth.Action, resourceType string, resourceID int64, ownerID int64) error {
	return s.authorizer.Can(actor, action, auth.Resource{Type: resourceType, ID: resourceID, OwnerID: ownerID})
}

func normalizeCreateEventAggregateInput(input CreateEventAggregateInput) (CreateEventAggregateInput, error) {
	input.Metadata = normalizeEventAggregateMetadata(input.Metadata)
	input.Tags = normalizedTags(input.Tags)
	if input.Metadata.Title == "" {
		return CreateEventAggregateInput{}, ErrInvalidEventAggregate
	}
	return input, nil
}

func normalizeCreateEventInput(input CreateEventInput) (CreateEventInput, error) {
	input.Metadata = normalizeEventMetadata(input.Metadata)
	input.Tags = normalizedTags(input.Tags)
	if input.Recurrence.Kind == "" {
		input.Recurrence.Kind = RecurrenceKindSingle
	}
	weekdays, validWeekdays := normalizedWeekdays(input.Recurrence.Weekdays)
	if !validWeekdays {
		return CreateEventInput{}, ErrInvalidEvent
	}
	input.Recurrence.Weekdays = weekdays

	if input.Metadata.Title == "" || input.StartsAt.IsZero() || input.DurationMinutes < 1 {
		return CreateEventInput{}, ErrInvalidEvent
	}
	switch input.Recurrence.Kind {
	case RecurrenceKindSingle:
		input.Recurrence.Weekdays = nil
		input.Recurrence.WeekCount = 0
	case RecurrenceKindWeekly:
		if input.Recurrence.WeekCount < 1 || input.Recurrence.WeekCount > maxRecurrenceWeekCount || len(input.Recurrence.Weekdays) == 0 {
			return CreateEventInput{}, ErrInvalidEvent
		}
	default:
		return CreateEventInput{}, ErrInvalidEvent
	}
	return input, nil
}

func expandEventInputs(input CreateEventInput) ([]CreateEventInput, error) {
	switch input.Recurrence.Kind {
	case RecurrenceKindSingle:
		return []CreateEventInput{eventInputAt(input, input.StartsAt)}, nil
	case RecurrenceKindWeekly:
		events := make([]CreateEventInput, 0, len(input.Recurrence.Weekdays)*input.Recurrence.WeekCount)
		weekStart := startOfWeek(input.StartsAt)
		for week := 0; week < input.Recurrence.WeekCount; week++ {
			for _, weekday := range input.Recurrence.Weekdays {
				startsAt := atWeekdayTime(weekStart, week, weekday, input.StartsAt)
				if startsAt.Before(input.StartsAt) {
					continue
				}
				events = append(events, eventInputAt(input, startsAt))
			}
		}
		if len(events) == 0 {
			return nil, ErrInvalidEvent
		}
		return events, nil
	default:
		return nil, ErrInvalidEvent
	}
}

func eventInputAt(input CreateEventInput, startsAt time.Time) CreateEventInput {
	return CreateEventInput{
		Metadata: input.Metadata, Tags: input.Tags,
		StartsAt: startsAt, DurationMinutes: input.DurationMinutes,
	}
}

func startOfWeek(t time.Time) time.Time {
	date := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	offset := (int(date.Weekday()) + 6) % 7
	return date.AddDate(0, 0, -offset)
}

func atWeekdayTime(weekStart time.Time, week int, weekday Weekday, template time.Time) time.Time {
	dayOffset := map[Weekday]int{
		WeekdayMonday: 0, WeekdayTuesday: 1, WeekdayWednesday: 2, WeekdayThursday: 3,
		WeekdayFriday: 4, WeekdaySaturday: 5, WeekdaySunday: 6,
	}[weekday]
	date := weekStart.AddDate(0, 0, week*7+dayOffset)
	return time.Date(date.Year(), date.Month(), date.Day(), template.Hour(), template.Minute(), template.Second(), template.Nanosecond(), template.Location())
}

func normalizeEventAggregateMetadata(metadata EventAggregateMetadata) EventAggregateMetadata {
	return EventAggregateMetadata{
		Title: strings.TrimSpace(metadata.Title), Description: strings.TrimSpace(metadata.Description),
		Location: strings.TrimSpace(metadata.Location), Timezone: strings.TrimSpace(metadata.Timezone),
	}
}

func normalizeEventMetadata(metadata EventMetadata) EventMetadata {
	return EventMetadata{
		Title: strings.TrimSpace(metadata.Title), Description: strings.TrimSpace(metadata.Description),
		Location: strings.TrimSpace(metadata.Location),
	}
}

func normalizedWeekdays(days []Weekday) ([]Weekday, bool) {
	seen := make(map[Weekday]struct{})
	for _, day := range days {
		day = Weekday(strings.ToLower(strings.TrimSpace(string(day))))
		if !validWeekday(day) {
			return nil, false
		}
		seen[day] = struct{}{}
	}
	weekOrder := []Weekday{
		WeekdayMonday, WeekdayTuesday, WeekdayWednesday, WeekdayThursday,
		WeekdayFriday, WeekdaySaturday, WeekdaySunday,
	}
	normalized := make([]Weekday, 0, len(seen))
	for _, day := range weekOrder {
		if _, exists := seen[day]; exists {
			normalized = append(normalized, day)
		}
	}
	return normalized, true
}

func validWeekday(day Weekday) bool {
	switch day {
	case WeekdayMonday, WeekdayTuesday, WeekdayWednesday, WeekdayThursday, WeekdayFriday, WeekdaySaturday, WeekdaySunday:
		return true
	default:
		return false
	}
}

func normalizedTags(names []string) []string {
	tags := make([]string, 0, len(names))
	seen := make(map[string]struct{})
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}
	return tags
}

func aggregateProjectionTitle(aggregate EventAggregate) string {
	return aggregate.Metadata.Title
}

func eventProjectionTitle(event Event) string {
	return event.Metadata.Title
}
