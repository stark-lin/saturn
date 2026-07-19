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

const maxRecurrenceCount = 520

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
	if err := s.can(actor, auth.ActionRead, "event_aggregate", 0, 0); err != nil {
		return EventAggregatePage{}, err
	}
	return s.repo.ListEventAggregates(ctx, auth.ScopeForPrincipal(actor), query)
}

func (s *Service) CreateEventAggregate(ctx context.Context, actor auth.Principal, input CreateEventAggregateInput) (EventAggregateDetail, error) {
	return s.createEventAggregateWithEvents(ctx, actor, input, nil)
}

func (s *Service) ImportEventAggregate(ctx context.Context, actor auth.Principal, input ImportEventAggregateInput) (EventAggregateDetail, error) {
	if err := s.can(actor, auth.ActionCreate, "event_aggregate", 0, 0); err != nil {
		return EventAggregateDetail{}, err
	}
	parsed, err := parseICSImport(input)
	if err != nil {
		return EventAggregateDetail{}, wrapInvalidICSImport(err)
	}
	return s.createEventAggregateWithEvents(ctx, actor, CreateEventAggregateInput{
		Metadata: EventAggregateMetadata{
			Title:       input.Title,
			Description: parsed.Description,
			Timezone:    parsed.Timezone,
		},
	}, parsed.Events)
}

func (s *Service) createEventAggregateWithEvents(
	ctx context.Context,
	actor auth.Principal,
	input CreateEventAggregateInput,
	eventInputs []CreateEventInput,
) (EventAggregateDetail, error) {
	if err := s.can(actor, auth.ActionCreate, "event_aggregate", 0, 0); err != nil {
		return EventAggregateDetail{}, err
	}
	input, err := normalizeCreateEventAggregateInput(input)
	if err != nil {
		return EventAggregateDetail{}, err
	}
	if len(eventInputs) > maxICSImportedEvents {
		return EventAggregateDetail{}, ErrInvalidICSImport
	}
	normalizedEvents := make([]CreateEventInput, 0, len(eventInputs))
	for _, eventInput := range eventInputs {
		normalized, err := normalizeCreateEventInput(eventInput)
		if err != nil || normalized.Recurrence.Kind != RecurrenceKindNone {
			return EventAggregateDetail{}, ErrInvalidICSImport
		}
		normalizedEvents = append(normalizedEvents, normalized)
	}

	aggregateRefCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeEventAggregate)
	if err != nil {
		return EventAggregateDetail{}, err
	}
	eventRefCodes := make([]string, 0, len(normalizedEvents))
	for range normalizedEvents {
		eventRefCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeEvent)
		if err != nil {
			return EventAggregateDetail{}, err
		}
		eventRefCodes = append(eventRefCodes, eventRefCode)
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
		created.Events = make([]Event, 0, len(normalizedEvents))

		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorRefCode: actor.ActorRefCode(), Action: audit.ActionCreate,
			TargetRefCode: aggregate.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		for index, eventInput := range normalizedEvents {
			event, err := s.repo.CreateEvent(txCtx, actor.ID, aggregate.ID, eventInput)
			if err != nil {
				return err
			}
			eventRef, err := s.references.Register(txCtx, ref.Registration{
				OwnerID: actor.ID, RefCode: eventRefCodes[index], ObjectType: ref.ObjectTypeEvent,
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
				ActorRefCode: actor.ActorRefCode(), Action: audit.ActionCreate,
				TargetRefCode: event.RefCode, Result: audit.ResultSuccess,
			}); err != nil {
				return err
			}
			created.Events = append(created.Events, event)
		}
		return nil
	})
	if err != nil {
		return EventAggregateDetail{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, aggregateRefCode, err)
	}
	return created, nil
}

func (s *Service) CreateEvent(ctx context.Context, actor auth.Principal, aggregateRefCode string, input CreateEventInput) (EventAggregateDetail, error) {
	if err := s.can(actor, auth.ActionCreate, "event", 0, 0); err != nil {
		return EventAggregateDetail{}, err
	}
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
				ActorRefCode: actor.ActorRefCode(), Action: audit.ActionCreate,
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
	if err := s.can(actor, auth.ActionRead, "event_aggregate", 0, 0); err != nil {
		return EventAggregateDetail{}, err
	}
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
	if err := s.can(actor, auth.ActionDelete, "event_aggregate", 0, 0); err != nil {
		return err
	}
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
				ActorRefCode: actor.ActorRefCode(), Action: audit.ActionDelete,
				TargetRefCode: event.RefCode, Result: audit.ResultSuccess, Reason: "cascade_event_aggregate",
			}); err != nil {
				return err
			}
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorRefCode: actor.ActorRefCode(), Action: audit.ActionDelete,
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
	if err := s.can(actor, auth.ActionRead, "event", 0, 0); err != nil {
		return CalendarView{}, err
	}
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
	if err := s.can(actor, auth.ActionRead, "event", 0, 0); err != nil {
		return Event{}, err
	}
	return s.repo.FindEventByRefCode(ctx, auth.ScopeForPrincipal(actor), refCode)
}

func (s *Service) FinishEvent(ctx context.Context, actor auth.Principal, refCode string) (Event, error) {
	if err := s.can(actor, auth.ActionUpdate, "event", 0, 0); err != nil {
		return Event{}, err
	}
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
			ActorRefCode: actor.ActorRefCode(), Action: audit.ActionUpdate,
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
	if err := s.can(actor, auth.ActionUpdate, "event", 0, 0); err != nil {
		return Event{}, err
	}
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
			ActorRefCode: actor.ActorRefCode(), Action: audit.ActionUpdate,
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
		ActorRefCode: actor.ActorRefCode(), Action: action,
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
		input.Recurrence.Kind = RecurrenceKindNone
	}

	if input.Metadata.Title == "" || input.StartsAt.IsZero() || input.EndsAt.IsZero() || !input.EndsAt.After(input.StartsAt) {
		return CreateEventInput{}, ErrInvalidEvent
	}
	switch input.Recurrence.Kind {
	case RecurrenceKindNone:
		if input.Recurrence.Count < 0 || input.Recurrence.Count > 1 {
			return CreateEventInput{}, ErrInvalidEvent
		}
		input.Recurrence.Count = 1
	case RecurrenceKindWeek, RecurrenceKindMonth, RecurrenceKindYear:
		if input.Recurrence.Count < 1 || input.Recurrence.Count > maxRecurrenceCount {
			return CreateEventInput{}, ErrInvalidEvent
		}
	default:
		return CreateEventInput{}, ErrInvalidEvent
	}
	return input, nil
}

func expandEventInputs(input CreateEventInput) ([]CreateEventInput, error) {
	switch input.Recurrence.Kind {
	case RecurrenceKindNone:
		return []CreateEventInput{eventInputAt(input, input.StartsAt)}, nil
	case RecurrenceKindWeek, RecurrenceKindMonth, RecurrenceKindYear:
		events := make([]CreateEventInput, 0, input.Recurrence.Count)
		for occurrence := 0; occurrence < input.Recurrence.Count; occurrence++ {
			startsAt := recurringEventStartsAt(input.StartsAt, input.Recurrence.Kind, occurrence)
			eventInput := eventInputAt(input, startsAt)
			if !eventInput.EndsAt.After(eventInput.StartsAt) {
				return nil, ErrInvalidEvent
			}
			events = append(events, eventInput)
		}
		return events, nil
	default:
		return nil, ErrInvalidEvent
	}
}

func recurringEventStartsAt(startsAt time.Time, kind RecurrenceKind, occurrence int) time.Time {
	switch kind {
	case RecurrenceKindWeek:
		return startsAt.AddDate(0, 0, occurrence*7)
	case RecurrenceKindMonth:
		return clampedCalendarDate(startsAt, 0, occurrence)
	case RecurrenceKindYear:
		return clampedCalendarDate(startsAt, occurrence, 0)
	default:
		return startsAt
	}
}

func clampedCalendarDate(template time.Time, yearOffset int, monthOffset int) time.Time {
	targetMonth := time.Date(
		template.Year()+yearOffset, template.Month()+time.Month(monthOffset), 1,
		template.Hour(), template.Minute(), template.Second(), template.Nanosecond(), template.Location(),
	)
	lastDay := time.Date(targetMonth.Year(), targetMonth.Month()+1, 0, 0, 0, 0, 0, targetMonth.Location()).Day()
	day := template.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(
		targetMonth.Year(), targetMonth.Month(), day,
		template.Hour(), template.Minute(), template.Second(), template.Nanosecond(), template.Location(),
	)
}

func eventInputAt(input CreateEventInput, startsAt time.Time) CreateEventInput {
	return CreateEventInput{
		Metadata: input.Metadata, Tags: input.Tags,
		StartsAt: startsAt, EndsAt: recurringEventEndsAt(input, startsAt),
	}
}

func recurringEventEndsAt(input CreateEventInput, startsAt time.Time) time.Time {
	if startsAt.Equal(input.StartsAt) {
		return input.EndsAt
	}
	templateEndsAt := input.EndsAt.In(input.StartsAt.Location())
	dayOffset := calendarDayOffset(input.StartsAt, templateEndsAt)
	endDate := startsAt.AddDate(0, 0, dayOffset)
	return time.Date(
		endDate.Year(), endDate.Month(), endDate.Day(),
		templateEndsAt.Hour(), templateEndsAt.Minute(), templateEndsAt.Second(), templateEndsAt.Nanosecond(),
		startsAt.Location(),
	)
}

func calendarDayOffset(startsAt time.Time, endsAt time.Time) int {
	startDate := time.Date(startsAt.Year(), startsAt.Month(), startsAt.Day(), 0, 0, 0, 0, time.UTC)
	endDate := time.Date(endsAt.Year(), endsAt.Month(), endsAt.Day(), 0, 0, 0, 0, time.UTC)
	return int(endDate.Sub(startDate) / (24 * time.Hour))
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
