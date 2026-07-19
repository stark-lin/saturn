// This file tests administrator and API-key HTTP authentication contracts.
package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerLoginReturnsAdministratorPrincipal(t *testing.T) {
	service := newTestService(t, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"admin"}`))
	response := httptest.NewRecorder()
	NewHandler(service).Login(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", response.Code, response.Body.String())
	}
	var result LoginResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if result.User.RefCode != AdministratorRefCode || result.User.Kind != PrincipalKindAdministrator {
		t.Fatalf("login user = %#v", result.User)
	}
	if strings.Contains(response.Body.String(), "username") {
		t.Fatalf("login response still exposes username: %s", response.Body.String())
	}
}

func TestHandlerLoginRejectsRemovedUsernameField(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"admin"}`))
	response := httptest.NewRecorder()
	NewHandler(newTestService(t, nil)).Login(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("login status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}

func TestHandlerLoginReportsInvalidPasswordOnly(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"wrong"}`))
	response := httptest.NewRecorder()
	NewHandler(newTestService(t, nil)).Login(response, request)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"message":"Invalid password"`) {
		t.Fatalf("login response = %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "username") {
		t.Fatalf("login error still mentions username: %s", response.Body.String())
	}
}

func TestHandlerCreatesListsAndRevokesAPIKey(t *testing.T) {
	service := newTestService(t, nil)
	service.credential = func() (string, error) { return "sat_sk_once-secret-value", nil }
	handler := NewHandler(service)
	create := authenticatedAuthRequest(http.MethodPost, "/api/auth/api-keys", `{"name":"saturn-mcp","scopes":["data:read"]}`, testAdministrator())
	createResponse := httptest.NewRecorder()
	handler.CreateAPIKey(createResponse, create)
	if createResponse.Code != http.StatusCreated || !strings.Contains(createResponse.Body.String(), `"api_key":"sat_sk_once-secret-value"`) {
		t.Fatalf("create response = %d %s", createResponse.Code, createResponse.Body.String())
	}
	listResponse := httptest.NewRecorder()
	handler.ListAPIKeys(listResponse, authenticatedAuthRequest(http.MethodGet, "/api/auth/api-keys", "", testAdministrator()))
	if listResponse.Code != http.StatusOK || strings.Contains(listResponse.Body.String(), "sat_sk_once-secret-value") || strings.Contains(listResponse.Body.String(), "key_hash") {
		t.Fatalf("list response = %d %s", listResponse.Code, listResponse.Body.String())
	}
	revoke := authenticatedAuthRequest(http.MethodPost, "/api/auth/api-keys/KEY-4F8A2C10/revoke", `{}`, testAdministrator())
	revoke.SetPathValue("refcode", "KEY-4F8A2C10")
	revokeResponse := httptest.NewRecorder()
	handler.RevokeAPIKey(revokeResponse, revoke)
	if revokeResponse.Code != http.StatusOK || !strings.Contains(revokeResponse.Body.String(), `"status":"REVOKED"`) {
		t.Fatalf("revoke response = %d %s", revokeResponse.Code, revokeResponse.Body.String())
	}
}

func TestRequireScopeEnforcesAPIKeyScopes(t *testing.T) {
	called := false
	handler := RequireScope(ScopeDataWrite, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	readOnly := Principal{ID: 1, RefCode: "KEY-00000001", Kind: PrincipalKindAPIKey, Scopes: []ScopeName{ScopeDataRead}}
	request := httptest.NewRequest(http.MethodPost, "/api/notes", nil)
	request = request.WithContext(ContextWithPrincipal(request.Context(), readOnly))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || called {
		t.Fatalf("scope response = %d, called = %v", response.Code, called)
	}
}

func TestAuthenticateBearerRejectsMissingCredential(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	response := httptest.NewRecorder()
	AuthenticateBearer(newTestService(t, nil), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler called")
	})).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing credential status = %d", response.Code)
	}
}

func authenticatedAuthRequest(method, target, body string, principal Principal) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	return request.WithContext(ContextWithPrincipal(request.Context(), principal))
}
