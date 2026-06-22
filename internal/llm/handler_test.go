// This file tests LLM HTTP handler contracts.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

func TestListSessionsRespondsWithPaginationEnvelope(t *testing.T) {
	service := &fakeSessionService{page: SessionPage{
		Sessions: []Session{{RefCode: "LLM-00000001", Title: "Planning", Status: SessionStatusActive, Tags: nil}},
		Limit:    5,
		Offset:   2,
		HasMore:  true,
	}}
	handler := NewHandler(service)
	request := authenticatedLLMRequest(http.MethodGet, "/api/llm/sessions?limit=5&offset=2", "")
	response := httptest.NewRecorder()

	handler.ListSessions(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.listLimit != 5 || service.listOffset != 2 {
		t.Fatalf("pagination = %d/%d", service.listLimit, service.listOffset)
	}
	var body SessionsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Sessions) != 1 || body.Sessions[0].RefCode != "LLM-00000001" || body.Sessions[0].Tags == nil && len(body.Sessions[0].Tags) != 0 {
		t.Fatalf("body = %#v", body)
	}
	if !body.HasMore || body.Limit != 5 || body.Offset != 2 {
		t.Fatalf("pagination body = %#v", body)
	}
}

func TestCreateSessionRespondsWithSessionResource(t *testing.T) {
	service := &fakeSessionService{}
	handler := NewHandler(service)
	request := authenticatedLLMRequest(http.MethodPost, "/api/llm/sessions", `{"title":"Planning","tags":["llm"]}`)
	response := httptest.NewRecorder()

	handler.CreateSession(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if response.Header().Get("Location") != "/api/llm/sessions/LLM-00000001" {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}
	if service.createSessionInput.Title != "Planning" || len(service.createSessionInput.Tags) != 1 {
		t.Fatalf("create input = %#v", service.createSessionInput)
	}
}

func TestGetSessionRespondsWithRequests(t *testing.T) {
	service := &fakeSessionService{detail: SessionDetail{
		Session: Session{RefCode: "LLM-00000001", Title: "Planning", Status: SessionStatusActive},
		Requests: []Request{{
			RefCode: "LLM-00000002", Prompt: "Summarize", Model: "test-model", MaxTokens: 100,
			ResponseStatus: ResponseStatusSuccess, ResponseContent: "answer",
		}},
	}}
	handler := NewHandler(service)
	request := authenticatedLLMRequest(http.MethodGet, "/api/llm/sessions/LLM-00000001?limit=10&offset=1", "")
	request.SetPathValue("ref_code", "LLM-00000001")
	response := httptest.NewRecorder()

	handler.GetSession(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.sessionRefCode != "LLM-00000001" || service.getSessionLimit != 10 || service.getSessionOffset != 1 {
		t.Fatalf("session call = %q %d/%d", service.sessionRefCode, service.getSessionLimit, service.getSessionOffset)
	}
	var body SessionDetailResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Requests) != 1 || body.Requests[0].ResponseContent != "answer" {
		t.Fatalf("body = %#v", body)
	}
}

func TestCreateRequestRespondsWithRequestResource(t *testing.T) {
	service := &fakeSessionService{
		request: Request{
			RefCode: "LLM-00000002", Prompt: "Summarize", Model: "test-model", MaxTokens: 100,
			References:     []RequestReference{{RefCode: "ACC-00000001", Module: "accounting", ObjectType: "account", Title: "Cash", Status: "active", Tags: []string{"finance"}}},
			ResponseStatus: ResponseStatusQueued,
			Tags:           []string{"review"},
			CreatedAt:      time.Unix(100, 0).UTC(),
			UpdatedAt:      time.Unix(102, 0).UTC(),
		},
	}
	handler := NewHandler(service)
	request := authenticatedLLMRequest(http.MethodPost, "/api/llm/sessions/LLM-00000001/requests",
		`{"prompt":"Summarize","references":["ACC-00000001"],"tags":["review"]}`)
	request.SetPathValue("ref_code", "LLM-00000001")
	response := httptest.NewRecorder()

	handler.CreateRequest(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if response.Header().Get("Location") != "/api/llm/requests/LLM-00000002" {
		t.Fatalf("Location = %q", response.Header().Get("Location"))
	}
	var body RequestResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Request.RefCode != "LLM-00000002" {
		t.Fatalf("request ref = %#v", body.Request)
	}
	if body.Request.ResponseStatus != ResponseStatusQueued || body.Request.ResponseContent != "" {
		t.Fatalf("request response = %#v", body.Request)
	}
	if len(body.Request.Tags) != 1 || body.Request.Tags[0] != "review" {
		t.Fatalf("request tags = %#v", body.Request.Tags)
	}
	if len(body.Request.References) != 1 || len(body.Request.References[0].Tags) != 1 {
		t.Fatalf("reference tags = %#v", body.Request.References)
	}
	if service.sessionRefCode != "LLM-00000001" {
		t.Fatalf("session ref_code = %q", service.sessionRefCode)
	}
	if len(service.createRequestInput.Tags) != 1 || service.createRequestInput.Tags[0] != "review" {
		t.Fatalf("request input tags = %#v", service.createRequestInput.Tags)
	}
}

func TestGetRequestRespondsWithRequestResource(t *testing.T) {
	service := &fakeSessionService{request: Request{
		RefCode: "LLM-00000002", Prompt: "Summarize", Model: "test-model", MaxTokens: 100,
		ResponseStatus: ResponseStatusSuccess, ResponseContent: "answer",
	}}
	handler := NewHandler(service)
	request := authenticatedLLMRequest(http.MethodGet, "/api/llm/requests/LLM-00000002", "")
	request.SetPathValue("ref_code", "LLM-00000002")
	response := httptest.NewRecorder()

	handler.GetRequest(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if service.requestRefCode != "LLM-00000002" {
		t.Fatalf("request ref code = %q", service.requestRefCode)
	}
}

func TestDeleteSessionRespondsNoContent(t *testing.T) {
	service := &fakeSessionService{}
	handler := NewHandler(service)
	request := authenticatedLLMRequest(http.MethodDelete, "/api/llm/sessions/LLM-00000001", "")
	request.SetPathValue("ref_code", "LLM-00000001")
	response := httptest.NewRecorder()

	handler.DeleteSession(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if service.sessionRefCode != "LLM-00000001" {
		t.Fatalf("deleted ref code = %q", service.sessionRefCode)
	}
}

func TestHandlerRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		call func(*Handler, http.ResponseWriter, *http.Request)
		req  *http.Request
	}{
		{
			name: "missing principal",
			call: (*Handler).ListSessions,
			req:  httptest.NewRequest(http.MethodGet, "/api/llm/sessions", nil),
		},
		{
			name: "unsupported pagination",
			call: (*Handler).ListSessions,
			req:  authenticatedLLMRequest(http.MethodGet, "/api/llm/sessions?sort=created_at", ""),
		},
		{
			name: "invalid session ref code",
			call: (*Handler).GetSession,
			req:  requestWithPathValue(authenticatedLLMRequest(http.MethodGet, "/api/llm/sessions/bad", ""), "ref_code", "bad"),
		},
		{
			name: "invalid request ref code",
			call: (*Handler).GetRequest,
			req:  requestWithPathValue(authenticatedLLMRequest(http.MethodGet, "/api/llm/requests/bad", ""), "ref_code", "bad"),
		},
		{
			name: "invalid create session json",
			call: (*Handler).CreateSession,
			req:  authenticatedLLMRequest(http.MethodPost, "/api/llm/sessions", `{"title":`),
		},
		{
			name: "invalid create request json",
			call: (*Handler).CreateRequest,
			req:  requestWithPathValue(authenticatedLLMRequest(http.MethodPost, "/api/llm/sessions/LLM-00000001/requests", `{"prompt":`), "ref_code", "LLM-00000001"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(&fakeSessionService{})
			response := httptest.NewRecorder()

			tt.call(handler, response, tt.req)

			if response.Code != http.StatusBadRequest && response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want client error: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestHandlerMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "unauthenticated", err: auth.ErrUnauthenticated, wantStatus: http.StatusUnauthorized},
		{name: "invalid request", err: ErrInvalidRequest, wantStatus: http.StatusBadRequest},
		{name: "not found", err: ErrReferenceNotFound, wantStatus: http.StatusNotFound},
		{name: "conflict", err: ErrRequestAlreadyFinal, wantStatus: http.StatusConflict},
		{name: "unexpected", err: errors.New("database down"), wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(&fakeSessionService{err: tt.err})
			request := authenticatedLLMRequest(http.MethodPost, "/api/llm/sessions", `{"title":"Planning"}`)
			response := httptest.NewRecorder()

			handler.CreateSession(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body.String())
			}
		})
	}
}

func authenticatedLLMRequest(method string, target string, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	principal := auth.Principal{ID: 1, Username: "alice", Role: auth.RoleUser}
	return request.WithContext(auth.ContextWithPrincipal(request.Context(), principal))
}

func requestWithPathValue(request *http.Request, key string, value string) *http.Request {
	request.SetPathValue(key, value)
	return request
}

type fakeSessionService struct {
	request            Request
	page               SessionPage
	detail             SessionDetail
	err                error
	sessionRefCode     string
	requestRefCode     string
	createSessionInput CreateSessionInput
	createRequestInput CreateRequestInput
	listLimit          int
	listOffset         int
	getSessionLimit    int
	getSessionOffset   int
}

func (s *fakeSessionService) ListSessions(_ context.Context, _ auth.Principal, limit int, offset int) (SessionPage, error) {
	s.listLimit = limit
	s.listOffset = offset
	if s.err != nil {
		return SessionPage{}, s.err
	}
	if len(s.page.Sessions) > 0 || s.page.Limit != 0 || s.page.Offset != 0 || s.page.HasMore {
		return s.page, nil
	}
	return SessionPage{Limit: limit, Offset: offset}, nil
}

func (s *fakeSessionService) CreateSession(_ context.Context, _ auth.Principal, input CreateSessionInput) (Session, error) {
	s.createSessionInput = input
	if s.err != nil {
		return Session{}, s.err
	}
	return Session{RefCode: "LLM-00000001", Title: input.Title, Status: SessionStatusActive}, nil
}

func (s *fakeSessionService) GetSession(_ context.Context, _ auth.Principal, refCode string, limit int, offset int) (SessionDetail, error) {
	s.sessionRefCode = refCode
	s.getSessionLimit = limit
	s.getSessionOffset = offset
	if s.err != nil {
		return SessionDetail{}, s.err
	}
	if s.detail.Session.RefCode != "" || len(s.detail.Requests) > 0 {
		return s.detail, nil
	}
	return SessionDetail{Session: Session{RefCode: refCode, Title: "Session", Status: SessionStatusActive}}, nil
}

func (s *fakeSessionService) CreateRequest(_ context.Context, _ auth.Principal, sessionRefCode string, input CreateRequestInput) (Request, error) {
	s.sessionRefCode = sessionRefCode
	s.createRequestInput = input
	if s.err != nil {
		return Request{}, s.err
	}
	return s.request, nil
}

func (s *fakeSessionService) GetRequest(_ context.Context, _ auth.Principal, refCode string) (Request, error) {
	s.requestRefCode = refCode
	if s.err != nil {
		return Request{}, s.err
	}
	return s.request, nil
}

func (s *fakeSessionService) DeleteSession(_ context.Context, _ auth.Principal, refCode string) error {
	s.sessionRefCode = refCode
	return s.err
}
