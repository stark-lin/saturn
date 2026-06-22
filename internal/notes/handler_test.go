// This file tests the owner-only Notes HTTP contract.
package notes

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

func TestHandlerCreateRejectsServerOwnedFields(t *testing.T) {
	handler := NewHandler(&fakeNoteService{})
	request := authenticatedRequest(http.MethodPost, "/api/notes", `{"markdown":"Title\n\nBody","status":"draft"}`)
	response := httptest.NewRecorder()

	handler.Create(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestHandlerCreateReturnsLocationAndCurrentNote(t *testing.T) {
	service := &fakeNoteService{note: Note{RefCode: "NTE-00000001", Title: "Title", Markdown: "Title\n\nBody", Status: NoteDraft}}
	handler := NewHandler(service)
	request := authenticatedRequest(http.MethodPost, "/api/notes", `{"markdown":"Title\n\nBody"}`)
	response := httptest.NewRecorder()

	handler.Create(response, request)

	if response.Code != http.StatusCreated || response.Header().Get("Location") != "/api/notes/NTE-00000001" {
		t.Fatalf("create response = %d location %q", response.Code, response.Header().Get("Location"))
	}
	var body NoteResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if body.Note.RefCode != "NTE-00000001" || body.Note.Status != NoteDraft {
		t.Fatalf("note response = %#v", body.Note)
	}
}

func TestHandlerListRejectsDeferredQueryParameters(t *testing.T) {
	handler := NewHandler(&fakeNoteService{})
	request := authenticatedRequest(http.MethodGet, "/api/notes?status=draft", "")
	response := httptest.NewRecorder()

	handler.List(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("list status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestHandlerGetMapsNonOwnedNoteToNotFound(t *testing.T) {
	handler := NewHandler(&fakeNoteService{err: ErrNoteNotFound})
	request := authenticatedRequest(http.MethodGet, "/api/notes/NTE-00000001", "")
	request.SetPathValue("ref_code", "NTE-00000001")
	response := httptest.NewRecorder()

	handler.Get(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("get status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestHandlerUpdateReturnsCurrentNote(t *testing.T) {
	service := &fakeNoteService{note: Note{RefCode: "NTE-00000001", Title: "New", Markdown: "New\n\nBody", Status: NoteDraft}}
	handler := NewHandler(service)
	request := authenticatedRequest(http.MethodPut, "/api/notes/NTE-00000001", `{"markdown":"New\n\nBody"}`)
	request.SetPathValue("ref_code", "NTE-00000001")
	response := httptest.NewRecorder()

	handler.Update(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", response.Code, http.StatusOK)
	}
	if service.updateRefCode != "NTE-00000001" || service.updateMarkdown != "New\n\nBody" {
		t.Fatalf("update call = ref %q markdown %q", service.updateRefCode, service.updateMarkdown)
	}
	var body NoteResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if body.Note.RefCode != "NTE-00000001" || body.Note.Title != "New" {
		t.Fatalf("update response = %#v", body.Note)
	}
}

func TestHandlerDeleteReturnsNoContentAndRejectsInvalidRef(t *testing.T) {
	service := &fakeNoteService{}
	handler := NewHandler(service)
	request := authenticatedRequest(http.MethodDelete, "/api/notes/NTE-00000001", "")
	request.SetPathValue("ref_code", "NTE-00000001")
	response := httptest.NewRecorder()

	handler.Delete(response, request)

	if response.Code != http.StatusNoContent || service.deleteRefCode != "NTE-00000001" {
		t.Fatalf("delete response = %d delete ref = %q", response.Code, service.deleteRefCode)
	}

	invalidRequest := authenticatedRequest(http.MethodDelete, "/api/notes/FIL-00000001", "")
	invalidRequest.SetPathValue("ref_code", "FIL-00000001")
	invalidResponse := httptest.NewRecorder()
	handler.Delete(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid ref status = %d, want %d", invalidResponse.Code, http.StatusBadRequest)
	}
}

type fakeNoteService struct {
	note           Note
	updateRefCode  string
	updateMarkdown string
	deleteRefCode  string
	err            error
}

func (s *fakeNoteService) ListNotes(_ context.Context, _ auth.Principal, query Query) (Page, error) {
	return Page{Notes: []Note{s.note}, Limit: query.Limit, Offset: query.Offset}, s.err
}

func (s *fakeNoteService) CreateNote(_ context.Context, _ auth.Principal, _ string) (Note, error) {
	return s.note, s.err
}

func (s *fakeNoteService) GetNote(_ context.Context, _ auth.Principal, _ string) (Note, error) {
	return s.note, s.err
}

func (s *fakeNoteService) UpdateNote(_ context.Context, _ auth.Principal, refCode string, markdown string) (Note, error) {
	s.updateRefCode = refCode
	s.updateMarkdown = markdown
	return s.note, s.err
}

func (s *fakeNoteService) DeleteNote(_ context.Context, _ auth.Principal, refCode string) error {
	s.deleteRefCode = refCode
	return s.err
}

func authenticatedRequest(method string, target string, body string) *http.Request {
	var buffer *bytes.Buffer
	if body == "" {
		buffer = bytes.NewBuffer(nil)
	} else {
		buffer = bytes.NewBufferString(body)
	}
	request := httptest.NewRequest(method, target, buffer)
	return request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
}
