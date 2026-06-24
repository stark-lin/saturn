// This file exposes authenticated LLM session and request API handlers.
package llm

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/httpx"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	limit, offset, err := bindPagination(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid LLM query")
		return
	}
	page, err := h.service.ListSessions(r.Context(), principal, limit, offset)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sessionsResponse(page))
}

func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request CreateSessionRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid LLM session request")
		return
	}
	session, err := h.service.CreateSession(r.Context(), principal, CreateSessionInput{Title: request.Title, Tags: request.Tags})
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/llm/sessions/"+session.RefCode)
	httpx.WriteJSON(w, http.StatusCreated, SessionResponse{Session: sessionView(session)})
}

func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := sessionResourceRequest(w, r)
	if !ok {
		return
	}
	limit, offset, err := bindPagination(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid LLM query")
		return
	}
	detail, err := h.service.GetSession(r.Context(), principal, refCode, limit, offset)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sessionDetailResponse(detail))
}

func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := sessionResourceRequest(w, r)
	if !ok {
		return
	}
	if h.writeServiceError(w, h.service.DeleteSession(r.Context(), principal, refCode)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := sessionResourceRequest(w, r)
	if !ok {
		return
	}
	var request CreateRequestRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid LLM request")
		return
	}
	requestModel, err := h.service.CreateRequest(r.Context(), principal, refCode, CreateRequestInput{
		Prompt: request.Prompt, References: request.References, Model: request.Model, MaxTokens: request.MaxTokens,
		Tags: request.Tags,
	})
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/llm/requests/"+requestModel.RefCode)
	httpx.WriteJSON(w, http.StatusAccepted, RequestResponse{Request: requestView(requestModel)})
}

func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := requestResourceRequest(w, r)
	if !ok {
		return
	}
	request, err := h.service.GetRequest(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, RequestResponse{Request: requestView(request)})
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
	case errors.Is(err, ErrInvalidSession), errors.Is(err, ErrInvalidRequest), errors.Is(err, ErrInvalidQuery):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid LLM request")
	case errors.Is(err, ErrSessionNotFound), errors.Is(err, ErrRequestNotFound),
		errors.Is(err, ErrReferenceNotFound), errors.Is(err, auth.ErrForbidden), errors.Is(err, ref.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "LLM resource not found")
	case errors.Is(err, ErrRequestAlreadyFinal):
		httpx.WriteError(w, http.StatusConflict, "conflict", "LLM request is already final")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "llm_unavailable", "LLM service is unavailable")
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

func sessionResourceRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, string, bool) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return auth.Principal{}, "", false
	}
	refCode := ref.NormalizeCode(r.PathValue("ref_code"))
	if !ref.ValidCode(refCode) || !strings.HasPrefix(refCode, "LLM-") {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return auth.Principal{}, "", false
	}
	return principal, refCode, true
}

func requestResourceRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, string, bool) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return auth.Principal{}, "", false
	}
	refCode := ref.NormalizeCode(r.PathValue("ref_code"))
	if !ref.ValidCode(refCode) || !strings.HasPrefix(refCode, "LLM-") {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return auth.Principal{}, "", false
	}
	return principal, refCode, true
}

func bindPagination(values url.Values) (int, int, error) {
	for key := range values {
		if key != "limit" && key != "offset" {
			return 0, 0, ErrInvalidQuery
		}
	}
	limit := DefaultLimit
	offset := 0
	if value := values.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, 0, ErrInvalidQuery
		}
		limit = parsed
	}
	if value := values.Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, 0, ErrInvalidQuery
		}
		offset = parsed
	}
	return normalizePagination(limit, offset)
}

func sessionsResponse(page SessionPage) SessionsResponse {
	sessions := make([]SessionView, 0, len(page.Sessions))
	for _, session := range page.Sessions {
		sessions = append(sessions, sessionView(session))
	}
	return SessionsResponse{Sessions: sessions, Limit: page.Limit, Offset: page.Offset, HasMore: page.HasMore}
}

func sessionDetailResponse(detail SessionDetail) SessionDetailResponse {
	requests := make([]RequestView, 0, len(detail.Requests))
	for _, request := range detail.Requests {
		requests = append(requests, requestView(request))
	}
	return SessionDetailResponse{Session: sessionView(detail.Session), Requests: requests}
}

func sessionView(session Session) SessionView {
	tags := session.Tags
	if tags == nil {
		tags = []string{}
	}
	return SessionView{
		RefCode: session.RefCode, Title: session.Title, Status: session.Status,
		Tags:      tags,
		CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
	}
}

func requestView(request Request) RequestView {
	references := make([]RequestReferenceView, 0, len(request.References))
	for _, reference := range request.References {
		tags := reference.Tags
		if tags == nil {
			tags = []string{}
		}
		references = append(references, RequestReferenceView{
			RefCode: reference.RefCode, Module: reference.Module, ObjectType: reference.ObjectType,
			Title: reference.Title, Status: reference.Status, Tags: tags,
		})
	}
	tags := request.Tags
	if tags == nil {
		tags = []string{}
	}
	return RequestView{
		RefCode: request.RefCode, Prompt: request.Prompt, Model: request.Model, MaxTokens: request.MaxTokens,
		References: references, ResponseStatus: request.ResponseStatus, ResponseContent: request.ResponseContent,
		ResponseErrorCode: request.ResponseErrorCode, ResponseErrorMessage: request.ResponseErrorMessage,
		Tags: tags, CreatedAt: request.CreatedAt, UpdatedAt: request.UpdatedAt, CompletedAt: request.CompletedAt,
	}
}
