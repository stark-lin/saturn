// This file tests the owner-only global-search reference metadata endpoint.
package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

func TestHandlerMetadataReturnsReferenceJSON(t *testing.T) {
	resolver := &fakeMetadataResolver{metadata: ref.Metadata{
		RefCode:    "NTE-00000001",
		Module:     ref.ModuleNotes,
		ObjectType: ref.ObjectTypeNote,
		Title:      "Release notes",
		Tags:       []string{"backend"},
		Status:     "draft",
	}}
	handler := NewHandler(resolver)
	request := httptest.NewRequest(http.MethodGet, "/api/platform/search?ref_code=nte-00000001", nil)
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.Metadata(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("metadata status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var metadata ref.Metadata
	if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
		t.Fatalf("decode metadata response: %v", err)
	}
	if metadata.Module != ref.ModuleNotes || metadata.Status != "draft" || len(metadata.Tags) != 1 || resolver.code != "nte-00000001" {
		t.Fatalf("metadata = %#v, resolver code = %q", metadata, resolver.code)
	}
}

func TestHandlerObjectRefMetadataReturnsReferenceJSON(t *testing.T) {
	resolver := &fakeMetadataResolver{metadata: ref.Metadata{
		RefCode:    "FIL-00000001",
		Module:     ref.ModuleFiles,
		ObjectType: ref.ObjectTypeFile,
		Title:      "Receipt",
		Tags:       []string{"tax"},
		Status:     "active",
	}}
	handler := NewHandler(resolver)
	request := httptest.NewRequest(http.MethodGet, "/api/platform/object-refs/fil-00000001", nil)
	request.SetPathValue("ref_code", "fil-00000001")
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.ObjectRefMetadata(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("object ref metadata status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var metadata ref.Metadata
	if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
		t.Fatalf("decode object ref metadata response: %v", err)
	}
	if metadata.RefCode != "FIL-00000001" || resolver.code != "FIL-00000001" {
		t.Fatalf("metadata = %#v, resolver code = %q", metadata, resolver.code)
	}
}

func TestHandlerObjectRefMetadataRejectsInvalidReferenceCode(t *testing.T) {
	handler := NewHandler(&fakeMetadataResolver{})
	request := httptest.NewRequest(http.MethodGet, "/api/platform/object-refs/not-a-code", nil)
	request.SetPathValue("ref_code", "not-a-code")
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.ObjectRefMetadata(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid object ref code status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestHandlerMetadataHidesUnauthorizedOrMissingReferences(t *testing.T) {
	handler := NewHandler(&fakeMetadataResolver{err: ref.ErrNotFound})
	request := httptest.NewRequest(http.MethodGet, "/api/platform/search?ref_code=FIL-00000001", nil)
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 8, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.Metadata(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestHandlerMetadataRejectsMissingReferenceCode(t *testing.T) {
	handler := NewHandler(&fakeMetadataResolver{})
	request := httptest.NewRequest(http.MethodGet, "/api/platform/search", nil)
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.Metadata(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing ref code status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestHandlerSearchObjectRefsReturnsJSONList(t *testing.T) {
	resolver := &fakeMetadataResolver{search: []ref.Metadata{{
		RefCode:    "NTE-00000001",
		Module:     ref.ModuleNotes,
		ObjectType: ref.ObjectTypeNote,
		Title:      "Release notes",
		Tags:       []string{"backend", "release"},
		Status:     "draft",
	}}}
	handler := NewHandler(resolver)
	body := `{
		"modules":["notes","files"],
		"object_types":["note"],
		"statuses":["draft"],
		"tags":["backend","release"],
		"created_at":{"from":"2026-05-01T00:00:00Z","to":"2026-06-01T00:00:00Z"},
		"updated_at":{"from":"2026-05-02T00:00:00Z","to":"2026-06-02T00:00:00Z"},
		"sort":{"field":"updated_at","direction":"desc"},
		"limit":40
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/platform/object-refs/search", strings.NewReader(body))
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.SearchObjectRefs(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("search object refs status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var objects []ref.Metadata
	if err := json.NewDecoder(response.Body).Decode(&objects); err != nil {
		t.Fatalf("decode object ref search response: %v", err)
	}
	if len(objects) != 1 || objects[0].RefCode != "NTE-00000001" {
		t.Fatalf("search objects = %#v", objects)
	}
	if resolver.actorID != 7 || resolver.searchQuery.Limit != 40 {
		t.Fatalf("search actor = %d query = %#v", resolver.actorID, resolver.searchQuery)
	}
	if len(resolver.searchQuery.Modules) != 2 || resolver.searchQuery.Modules[0] != ref.ModuleNotes || resolver.searchQuery.Modules[1] != ref.ModuleFiles {
		t.Fatalf("search modules = %#v", resolver.searchQuery.Modules)
	}
	if resolver.searchQuery.CreatedFrom == nil || resolver.searchQuery.CreatedTo == nil || resolver.searchQuery.UpdatedFrom == nil || resolver.searchQuery.UpdatedTo == nil {
		t.Fatalf("search ranges were not parsed: %#v", resolver.searchQuery)
	}
	if resolver.searchQuery.Sort.Field != ref.MetadataSearchSortUpdatedAt || resolver.searchQuery.Sort.Direction != ref.MetadataSearchSortDescending {
		t.Fatalf("search sort = %#v", resolver.searchQuery.Sort)
	}
}

func TestHandlerSearchObjectRefsReturnsEmptyArray(t *testing.T) {
	handler := NewHandler(&fakeMetadataResolver{})
	request := httptest.NewRequest(http.MethodPost, "/api/platform/object-refs/search", strings.NewReader(`{}`))
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.SearchObjectRefs(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("empty search status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var objects []ref.Metadata
	if err := json.NewDecoder(response.Body).Decode(&objects); err != nil {
		t.Fatalf("decode empty search response: %v", err)
	}
	if objects == nil || len(objects) != 0 {
		t.Fatalf("empty search objects = %#v", objects)
	}
}

func TestHandlerSearchObjectRefsRejectsInvalidRequest(t *testing.T) {
	for _, body := range []string{
		`{"created_at":{"from":"not-a-time"}}`,
		`{"unexpected":true}`,
	} {
		handler := NewHandler(&fakeMetadataResolver{})
		request := httptest.NewRequest(http.MethodPost, "/api/platform/object-refs/search", strings.NewReader(body))
		request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
		response := httptest.NewRecorder()

		handler.SearchObjectRefs(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid search body %s status = %d, want %d", body, response.Code, http.StatusBadRequest)
		}
	}

	handler := NewHandler(&fakeMetadataResolver{err: ref.ErrInvalidMetadataSearchQuery})
	request := httptest.NewRequest(http.MethodPost, "/api/platform/object-refs/search", strings.NewReader(`{"modules":["unknown"]}`))
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.SearchObjectRefs(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid search query status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestHandlerRecentObjectsReturnsMetadataEnvelope(t *testing.T) {
	resolver := &fakeMetadataResolver{recent: []ref.Metadata{{
		RefCode:    "NTE-00000001",
		Module:     ref.ModuleNotes,
		ObjectType: ref.ObjectTypeNote,
		Title:      "Release notes",
		Tags:       []string{"backend"},
		Status:     "draft",
	}}}
	handler := NewHandler(resolver)
	request := httptest.NewRequest(http.MethodGet, "/api/platform/recent-objects?limit=4", nil)
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.RecentObjects(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("recent objects status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var result struct {
		Objects []ref.Metadata `json:"objects"`
		Limit   int            `json:"limit"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode recent objects response: %v", err)
	}
	if result.Limit != 4 || len(result.Objects) != 1 || len(result.Objects[0].Tags) != 1 || resolver.limit != 4 || resolver.actorID != 7 {
		t.Fatalf("recent objects result = %#v, resolver limit = %d, actor = %d", result, resolver.limit, resolver.actorID)
	}
}

func TestHandlerRecentObjectsReturnsEmptyArray(t *testing.T) {
	handler := NewHandler(&fakeMetadataResolver{})
	request := httptest.NewRequest(http.MethodGet, "/api/platform/recent-objects", nil)
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.RecentObjects(response, request)

	var result struct {
		Objects []ref.Metadata `json:"objects"`
		Limit   int            `json:"limit"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode empty recent objects response: %v", err)
	}
	if result.Objects == nil || len(result.Objects) != 0 || result.Limit != ref.DefaultRecentMetadataLimit {
		t.Fatalf("empty recent objects result = %#v", result)
	}
}

func TestHandlerRecentObjectsRejectsInvalidLimit(t *testing.T) {
	for _, target := range []string{
		"/api/platform/recent-objects?limit=51",
		"/api/platform/recent-objects?limit=",
		"/api/platform/recent-objects?sort=created_at",
	} {
		handler := NewHandler(&fakeMetadataResolver{})
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
		response := httptest.NewRecorder()

		handler.RecentObjects(response, request)

		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid recent objects query %q status = %d, want %d", target, response.Code, http.StatusBadRequest)
		}
	}
}

type fakeMetadataResolver struct {
	metadata    ref.Metadata
	recent      []ref.Metadata
	search      []ref.Metadata
	err         error
	code        string
	actorID     int64
	limit       int
	searchQuery ref.MetadataSearchQuery
}

func (r *fakeMetadataResolver) ResolveMetadata(_ context.Context, _ auth.Principal, code string) (ref.Metadata, error) {
	r.code = code
	return r.metadata, r.err
}

func (r *fakeMetadataResolver) ListRecentMetadata(_ context.Context, actor auth.Principal, limit int) ([]ref.Metadata, error) {
	r.actorID = actor.ID
	r.limit = limit
	return r.recent, r.err
}

func (r *fakeMetadataResolver) SearchMetadata(_ context.Context, actor auth.Principal, query ref.MetadataSearchQuery) ([]ref.Metadata, error) {
	r.actorID = actor.ID
	r.searchQuery = query
	return r.search, r.err
}
