// This file tests the Calendar HTTP contract.
package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

func TestHandlerCreatesEmptyEventAggregate(t *testing.T) {
	service := &fakeEventService{detail: EventAggregateDetail{
		Aggregate: EventAggregate{
			RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Sprint"}, Tags: []string{"work"},
		},
	}}
	handler := NewHandler(service)
	request := authenticatedCalendarRequest(http.MethodPost, "/api/calendar/aggregates",
		`{"metadata":{"title":"Sprint"},"tags":["work"]}`)
	response := httptest.NewRecorder()

	handler.CreateEventAggregate(response, request)

	if response.Code != http.StatusCreated || response.Header().Get("Location") != "/api/calendar/aggregates/CAL-00000001" {
		t.Fatalf("create response = %d location %q", response.Code, response.Header().Get("Location"))
	}
	if len(service.createInput.Tags) != 1 || service.createInput.Tags[0] != "work" {
		t.Fatalf("create input = %#v", service.createInput)
	}
	var body EventAggregateResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != 0 {
		t.Fatalf("event response = %#v", body.Events)
	}
}

func TestHandlerCreatesEventUnderAggregateWithDurationAndTags(t *testing.T) {
	startsAt := time.Date(2026, time.June, 1, 9, 30, 0, 0, time.UTC)
	service := &fakeEventService{detail: EventAggregateDetail{
		Aggregate: EventAggregate{
			RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Sprint"}, Tags: []string{"work"},
		},
		Events: []Event{{
			RefCode: "CAL-00000002", AggregateRefCode: "CAL-00000001", StartsAt: startsAt,
			DurationMinutes: 45, Metadata: EventMetadata{Title: "Planning"}, Status: EventStatusScheduled,
			Tags: []string{"meeting"},
		}},
	}}
	handler := NewHandler(service)
	request := authenticatedCalendarRequest(http.MethodPost, "/api/calendar/aggregates/CAL-00000001/events",
		`{"metadata":{"title":"Planning"},"tags":["meeting"],"starts_at":"2026-06-01T09:30:00Z","duration_minutes":45,"recurrence":{"kind":"single"}}`)
	request.SetPathValue("ref_code", "CAL-00000001")
	response := httptest.NewRecorder()

	handler.CreateEvent(response, request)

	if response.Code != http.StatusCreated || response.Header().Get("Location") != "/api/calendar/aggregates/CAL-00000001" {
		t.Fatalf("create response = %d location %q", response.Code, response.Header().Get("Location"))
	}
	if service.createEventAggregateRef != "CAL-00000001" || service.createEventInput.DurationMinutes != 45 ||
		len(service.createEventInput.Tags) != 1 || service.createEventInput.Tags[0] != "meeting" {
		t.Fatalf("create event input = %q %#v", service.createEventAggregateRef, service.createEventInput)
	}
	var body EventAggregateResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].DurationMinutes != 45 || body.Events[0].Tags[0] != "meeting" {
		t.Fatalf("event response = %#v", body.Events)
	}
}

func TestHandlerMapsRepeatVoidToConflictAndRejectsUnknownViewQuery(t *testing.T) {
	handler := NewHandler(&fakeEventService{err: ErrEventAlreadyVoided})
	request := authenticatedCalendarRequest(http.MethodPost, "/api/calendar/events/CAL-00000002/void", "")
	request.SetPathValue("ref_code", "CAL-00000002")
	response := httptest.NewRecorder()

	handler.VoidEvent(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("void response status = %d, want %d", response.Code, http.StatusConflict)
	}

	viewRequest := authenticatedCalendarRequest(http.MethodGet, "/api/calendar/view?from=2026-06-01&to=2026-06-02&status=voided", "")
	rejected := httptest.NewRecorder()
	handler.CalendarView(rejected, viewRequest)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("unknown query response = %d, want %d", rejected.Code, http.StatusBadRequest)
	}
}

func TestHandlerFinishesEvent(t *testing.T) {
	service := &fakeEventService{event: Event{RefCode: "CAL-00000002", Status: EventStatusFinished}}
	handler := NewHandler(service)
	request := authenticatedCalendarRequest(http.MethodPost, "/api/calendar/events/CAL-00000002/finish", "")
	request.SetPathValue("ref_code", "CAL-00000002")
	response := httptest.NewRecorder()

	handler.FinishEvent(response, request)

	if response.Code != http.StatusOK || !service.finishCalled {
		t.Fatalf("finish response = %d finish called = %v", response.Code, service.finishCalled)
	}
	var body EventResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Event.Status != EventStatusFinished {
		t.Fatalf("finish event response = %#v", body.Event)
	}
}

func TestHandlerListsGetsDeletesAndViewsCalendarResources(t *testing.T) {
	startsAt := time.Date(2026, time.June, 1, 9, 30, 0, 0, time.UTC)
	service := &fakeEventService{
		detail: EventAggregateDetail{
			Aggregate: EventAggregate{
				RefCode: "CAL-00000001", Metadata: EventAggregateMetadata{Title: "Sprint"}, Tags: []string{"work"},
			},
			Events: []Event{{
				RefCode: "CAL-00000002", AggregateRefCode: "CAL-00000001", StartsAt: startsAt,
				DurationMinutes: 45, Metadata: EventMetadata{Title: "Planning"}, Status: EventStatusScheduled,
			}},
		},
		event: Event{
			RefCode: "CAL-00000002", AggregateRefCode: "CAL-00000001", StartsAt: startsAt,
			DurationMinutes: 45, Metadata: EventMetadata{Title: "Planning"}, Status: EventStatusScheduled,
		},
		view: CalendarView{
			From: startsAt.Add(-time.Hour), To: startsAt.Add(time.Hour),
			Events: []Event{{
				RefCode: "CAL-00000002", AggregateRefCode: "CAL-00000001", StartsAt: startsAt,
				DurationMinutes: 45, Metadata: EventMetadata{Title: "Planning"}, Status: EventStatusScheduled,
			}},
			Limit: 10,
		},
	}
	handler := NewHandler(service)

	listRequest := authenticatedCalendarRequest(http.MethodGet, "/api/calendar/aggregates?limit=10&offset=1", "")
	listResponse := httptest.NewRecorder()
	handler.ListEventAggregates(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list aggregates status = %d", listResponse.Code)
	}
	if service.listQuery.Limit != 10 || service.listQuery.Offset != 1 {
		t.Fatalf("list query = %#v", service.listQuery)
	}
	var listBody EventAggregatesResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode aggregates response: %v", err)
	}
	if len(listBody.Aggregates) != 1 || listBody.Aggregates[0].RefCode != "CAL-00000001" {
		t.Fatalf("aggregates response = %#v", listBody.Aggregates)
	}

	getAggregateRequest := authenticatedCalendarRequest(http.MethodGet, "/api/calendar/aggregates/CAL-00000001", "")
	getAggregateRequest.SetPathValue("ref_code", "CAL-00000001")
	getAggregateResponse := httptest.NewRecorder()
	handler.GetEventAggregate(getAggregateResponse, getAggregateRequest)
	if getAggregateResponse.Code != http.StatusOK {
		t.Fatalf("get aggregate status = %d", getAggregateResponse.Code)
	}

	viewRequest := authenticatedCalendarRequest(http.MethodGet, "/api/calendar/view?from=2026-06-01T09:00:00Z&to=2026-06-01T11:00:00Z&limit=10", "")
	viewResponse := httptest.NewRecorder()
	handler.CalendarView(viewResponse, viewRequest)
	if viewResponse.Code != http.StatusOK {
		t.Fatalf("calendar view status = %d", viewResponse.Code)
	}
	if service.viewQuery.From.IsZero() || service.viewQuery.To.IsZero() || service.viewQuery.Limit != 10 {
		t.Fatalf("view query = %#v", service.viewQuery)
	}
	var viewBody CalendarViewResponse
	if err := json.NewDecoder(viewResponse.Body).Decode(&viewBody); err != nil {
		t.Fatalf("decode view response: %v", err)
	}
	if len(viewBody.Events) != 1 || viewBody.Events[0].RefCode != "CAL-00000002" {
		t.Fatalf("view response = %#v", viewBody.Events)
	}

	getEventRequest := authenticatedCalendarRequest(http.MethodGet, "/api/calendar/events/CAL-00000002", "")
	getEventRequest.SetPathValue("ref_code", "CAL-00000002")
	getEventResponse := httptest.NewRecorder()
	handler.GetEvent(getEventResponse, getEventRequest)
	if getEventResponse.Code != http.StatusOK {
		t.Fatalf("get event status = %d", getEventResponse.Code)
	}

	deleteRequest := authenticatedCalendarRequest(http.MethodDelete, "/api/calendar/aggregates/CAL-00000001", "")
	deleteRequest.SetPathValue("ref_code", "CAL-00000001")
	deleteResponse := httptest.NewRecorder()
	handler.DeleteEventAggregate(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || service.deleteRefCode != "CAL-00000001" {
		t.Fatalf("delete response = %d ref = %q", deleteResponse.Code, service.deleteRefCode)
	}
}

func TestHandlerRejectsInvalidCalendarRefsAndUnauthenticatedList(t *testing.T) {
	handler := NewHandler(&fakeEventService{})
	invalidRequest := authenticatedCalendarRequest(http.MethodGet, "/api/calendar/events/NTE-00000001", "")
	invalidRequest.SetPathValue("ref_code", "NTE-00000001")
	invalidResponse := httptest.NewRecorder()
	handler.GetEvent(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid ref status = %d, want %d", invalidResponse.Code, http.StatusBadRequest)
	}

	unauthenticatedRequest := httptest.NewRequest(http.MethodGet, "/api/calendar/aggregates", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ListEventAggregates(unauthenticatedResponse, unauthenticatedRequest)
	if unauthenticatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", unauthenticatedResponse.Code, http.StatusUnauthorized)
	}
}

type fakeEventService struct {
	detail                  EventAggregateDetail
	event                   Event
	view                    CalendarView
	createInput             CreateEventAggregateInput
	createEventAggregateRef string
	createEventInput        CreateEventInput
	listQuery               EventAggregateQuery
	viewQuery               CalendarViewQuery
	deleteRefCode           string
	finishCalled            bool
	err                     error
}

func (s *fakeEventService) ListEventAggregates(_ context.Context, _ auth.Principal, query EventAggregateQuery) (EventAggregatePage, error) {
	s.listQuery = query
	return EventAggregatePage{Aggregates: []EventAggregate{s.detail.Aggregate}, Limit: query.Limit, Offset: query.Offset}, s.err
}

func (s *fakeEventService) CreateEventAggregate(_ context.Context, _ auth.Principal, input CreateEventAggregateInput) (EventAggregateDetail, error) {
	s.createInput = input
	return s.detail, s.err
}

func (s *fakeEventService) CreateEvent(_ context.Context, _ auth.Principal, aggregateRefCode string, input CreateEventInput) (EventAggregateDetail, error) {
	s.createEventAggregateRef = aggregateRefCode
	s.createEventInput = input
	return s.detail, s.err
}

func (s *fakeEventService) GetEventAggregate(_ context.Context, _ auth.Principal, _ string) (EventAggregateDetail, error) {
	return s.detail, s.err
}

func (s *fakeEventService) DeleteEventAggregate(_ context.Context, _ auth.Principal, refCode string) error {
	s.deleteRefCode = refCode
	return s.err
}

func (s *fakeEventService) CalendarView(_ context.Context, _ auth.Principal, query CalendarViewQuery) (CalendarView, error) {
	s.viewQuery = query
	return s.view, s.err
}

func (s *fakeEventService) GetEvent(_ context.Context, _ auth.Principal, _ string) (Event, error) {
	return s.event, s.err
}

func (s *fakeEventService) FinishEvent(_ context.Context, _ auth.Principal, _ string) (Event, error) {
	s.finishCalled = true
	return s.event, s.err
}

func (s *fakeEventService) VoidEvent(_ context.Context, _ auth.Principal, _ string) (Event, error) {
	return s.event, s.err
}

func authenticatedCalendarRequest(method string, target string, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	return request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
}
