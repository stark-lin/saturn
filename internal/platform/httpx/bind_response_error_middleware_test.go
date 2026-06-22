// This file tests shared HTTP binding, response, error, and middleware contracts.
package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindJSONRequiresBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Body = nil
	var input struct {
		Name string `json:"name"`
	}

	if err := BindJSON(request, &input); err == nil {
		t.Fatal("BindJSON with nil body error = nil, want error")
	}
}

func TestBindJSONRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Saturn","extra":true}`))
	var input struct {
		Name string `json:"name"`
	}

	if err := BindJSON(request, &input); err == nil {
		t.Fatal("BindJSON with unknown field error = nil, want error")
	}
}

func TestBindJSONDecodesValidBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Saturn"}`))
	var input struct {
		Name string `json:"name"`
	}

	if err := BindJSON(request, &input); err != nil {
		t.Fatalf("BindJSON valid body: %v", err)
	}
	if input.Name != "Saturn" {
		t.Fatalf("input name = %q, want Saturn", input.Name)
	}
}

func TestWriteJSONWritesStatusContentTypeAndBody(t *testing.T) {
	response := httptest.NewRecorder()

	WriteJSON(response, http.StatusCreated, map[string]string{"name": "Saturn"})

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["name"] != "Saturn" {
		t.Fatalf("body = %#v", body)
	}
}

func TestWriteErrorWritesSharedErrorShape(t *testing.T) {
	response := httptest.NewRecorder()

	WriteError(response, http.StatusBadRequest, "invalid_request", "Invalid request")

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	var body ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "invalid_request" || body.Error.Message != "Invalid request" {
		t.Fatalf("error body = %#v", body)
	}
}

func TestRequestIDUsesIncomingHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "request-123")
	response := httptest.NewRecorder()
	var contextRequestID string

	RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		contextRequestID, _ = r.Context().Value(requestIDKey).(string)
	})).ServeHTTP(response, request)

	if contextRequestID != "request-123" {
		t.Fatalf("context request id = %q, want request-123", contextRequestID)
	}
	if got := response.Header().Get("X-Request-ID"); got != "request-123" {
		t.Fatalf("response request id = %q, want request-123", got)
	}
}

func TestRequestIDGeneratesMissingHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	var contextRequestID string

	RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		contextRequestID, _ = r.Context().Value(requestIDKey).(string)
	})).ServeHTTP(response, request)

	if contextRequestID == "" {
		t.Fatal("context request id is empty")
	}
	if got := response.Header().Get("X-Request-ID"); got != contextRequestID {
		t.Fatalf("response request id = %q, want context request id %q", got, contextRequestID)
	}
}

func TestRecoverLogsPanicAndWritesInternalError(t *testing.T) {
	log := &fakeRecoverLogger{}
	handler := Recover(log)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if len(log.messages) != 1 || log.messages[0] != "panic recovered" {
		t.Fatalf("logged messages = %#v", log.messages)
	}
	var body ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Fatalf("error body = %#v", body)
	}
}

type fakeRecoverLogger struct {
	messages []string
	args     [][]any
}

func (l *fakeRecoverLogger) Error(msg string, args ...any) {
	l.messages = append(l.messages, msg)
	l.args = append(l.args, args)
}
