// This file exposes authenticated Calendar event aggregate API handlers.
package calendar

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/httpx"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

func (h *Handler) ListEventAggregates(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	query, err := bindEventAggregateQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid event aggregate query")
		return
	}
	page, err := h.service.ListEventAggregates(r.Context(), principal, query)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, eventAggregatesResponse(page))
}

func (h *Handler) CreateEventAggregate(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request CreateEventAggregateRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid event aggregate request")
		return
	}
	detail, err := h.service.CreateEventAggregate(r.Context(), principal, request.input())
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/calendar/aggregates/"+detail.Aggregate.RefCode)
	httpx.WriteJSON(w, http.StatusCreated, eventAggregateResponse(detail))
}

func (h *Handler) GetEventAggregate(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := calendarResourceRequest(w, r)
	if !ok {
		return
	}
	detail, err := h.service.GetEventAggregate(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, eventAggregateResponse(detail))
}

func (h *Handler) DeleteEventAggregate(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := calendarResourceRequest(w, r)
	if !ok {
		return
	}
	if h.writeServiceError(w, h.service.DeleteEventAggregate(r.Context(), principal, refCode)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	principal, aggregateRefCode, ok := calendarResourceRequest(w, r)
	if !ok {
		return
	}
	var request CreateEventRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid event request")
		return
	}
	startsAt, err := parseTimestamp(request.StartsAt)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid event request")
		return
	}
	detail, err := h.service.CreateEvent(r.Context(), principal, aggregateRefCode, request.input(startsAt))
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/calendar/aggregates/"+detail.Aggregate.RefCode)
	httpx.WriteJSON(w, http.StatusCreated, eventAggregateResponse(detail))
}

func (h *Handler) CalendarView(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	query, err := bindCalendarViewQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid calendar view query")
		return
	}
	view, err := h.service.CalendarView(r.Context(), principal, query)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, calendarViewResponse(view))
}

func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := calendarResourceRequest(w, r)
	if !ok {
		return
	}
	event, err := h.service.GetEvent(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, eventResponse(event))
}

func (h *Handler) FinishEvent(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := calendarResourceRequest(w, r)
	if !ok {
		return
	}
	event, err := h.service.FinishEvent(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, eventResponse(event))
}

func (h *Handler) VoidEvent(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := calendarResourceRequest(w, r)
	if !ok {
		return
	}
	event, err := h.service.VoidEvent(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, eventResponse(event))
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
	case errors.Is(err, ErrInvalidEventAggregate), errors.Is(err, ErrInvalidEvent), errors.Is(err, ErrInvalidQuery):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid calendar request")
	case errors.Is(err, ErrEventAlreadyFinished):
		httpx.WriteError(w, http.StatusConflict, "conflict", "Event is already finished")
	case errors.Is(err, ErrEventAlreadyVoided):
		httpx.WriteError(w, http.StatusConflict, "conflict", "Event is already voided")
	case errors.Is(err, ErrEventAggregateNotFound), errors.Is(err, ErrEventNotFound),
		errors.Is(err, auth.ErrForbidden), errors.Is(err, ref.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Calendar resource not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "calendar_unavailable", "Calendar service is unavailable")
	}
	return true
}

func authenticatedPrincipal(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return auth.Principal{}, false
	}
	return principal, true
}

func calendarResourceRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, string, bool) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return auth.Principal{}, "", false
	}
	refCode := ref.NormalizeCode(r.PathValue("ref_code"))
	if !ref.ValidCode(refCode) || !ref.CodeMatchesObjectType(refCode, ref.ObjectTypeEvent) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return auth.Principal{}, "", false
	}
	return principal, refCode, true
}

func bindEventAggregateQuery(values url.Values) (EventAggregateQuery, error) {
	for key := range values {
		if key != "limit" && key != "offset" {
			return EventAggregateQuery{}, ErrInvalidQuery
		}
	}
	query := EventAggregateQuery{Limit: DefaultLimit}
	if err := bindPagination(values, &query.Limit, &query.Offset); err != nil {
		return EventAggregateQuery{}, err
	}
	return normalizeEventAggregateQuery(query)
}

func bindCalendarViewQuery(values url.Values) (CalendarViewQuery, error) {
	for key := range values {
		switch key {
		case "from", "to", "limit", "offset":
		default:
			return CalendarViewQuery{}, ErrInvalidQuery
		}
	}
	query := CalendarViewQuery{Limit: DefaultLimit}
	var err error
	if query.From, err = parseTimestamp(values.Get("from")); err != nil {
		return CalendarViewQuery{}, ErrInvalidQuery
	}
	if query.To, err = parseTimestamp(values.Get("to")); err != nil {
		return CalendarViewQuery{}, ErrInvalidQuery
	}
	if err := bindPagination(values, &query.Limit, &query.Offset); err != nil {
		return CalendarViewQuery{}, err
	}
	return normalizeCalendarViewQuery(query)
}

func parseTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, ErrInvalidQuery
	}
	timestamp, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return timestamp, nil
	}
	return time.Parse(time.DateOnly, value)
}

func bindPagination(values url.Values, limit *int, offset *int) error {
	if value := values.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > MaxLimit {
			return ErrInvalidQuery
		}
		*limit = parsed
	}
	if value := values.Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return ErrInvalidQuery
		}
		*offset = parsed
	}
	return nil
}

func normalizeEventAggregateQuery(query EventAggregateQuery) (EventAggregateQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	if query.Limit < 1 || query.Limit > MaxLimit || query.Offset < 0 {
		return EventAggregateQuery{}, ErrInvalidQuery
	}
	return query, nil
}

func normalizeCalendarViewQuery(query CalendarViewQuery) (CalendarViewQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	if query.From.IsZero() || query.To.IsZero() || query.From.After(query.To) ||
		query.Limit < 1 || query.Limit > MaxLimit || query.Offset < 0 {
		return CalendarViewQuery{}, ErrInvalidQuery
	}
	return query, nil
}
