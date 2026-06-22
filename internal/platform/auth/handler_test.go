// This file tests authentication HTTP entry points and bearer middleware.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerLoginReturnsTokenAndCurrentUser(t *testing.T) {
	service := newTestService(t, "admin")
	handler := NewHandler(service)
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"admin"}`))
	response := httptest.NewRecorder()

	handler.Login(response, loginRequest)

	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var result LoginResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if result.Token == "" || result.User.Username != "admin" {
		t.Fatalf("login response = %#v, want token and admin", result)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+result.Token)
	meResponse := httptest.NewRecorder()
	AuthenticateBearer(service, http.HandlerFunc(handler.Me)).ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d: %s", meResponse.Code, http.StatusOK, meResponse.Body.String())
	}
}

func TestAuthenticateBearerRejectsMissingToken(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	response := httptest.NewRecorder()

	AuthenticateBearer(newTestService(t, "admin"), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler must not be called without authentication")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestHandlerLogoutReportsAuditWriteFailure(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	service := newTestServiceWithAudit(t, "admin", recorder)
	result, err := service.Login(context.Background(), "admin", "admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	recorder.err = errors.New("audit unavailable")
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.Header.Set("Authorization", "Bearer "+result.Token)
	response := httptest.NewRecorder()

	NewHandler(service).Logout(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("logout status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestHandlerCreatesUserAndUpdatesOwnAccount(t *testing.T) {
	handler := NewHandler(newTestService(t, "admin"))

	createRequest := authenticatedAuthRequest(http.MethodPost, "/api/auth/users",
		`{"username":"alice","email":"alice@example.com","password":"secret"}`, Principal{ID: 1, Username: "admin", Role: RoleSuperuser})
	createResponse := httptest.NewRecorder()
	handler.CreateUser(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	var createBody struct {
		User Principal `json:"user"`
	}
	if err := json.NewDecoder(createResponse.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create user response: %v", err)
	}
	if createBody.User.Username != "alice" || createBody.User.Role != RoleUser {
		t.Fatalf("create user response = %#v", createBody.User)
	}

	updateRequest := authenticatedAuthRequest(http.MethodPatch, "/api/auth/me",
		`{"username":"admin2","email":"admin2@example.com"}`, Principal{ID: 1, Username: "admin", Role: RoleSuperuser})
	updateResponse := httptest.NewRecorder()
	handler.UpdateMe(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update me status = %d, want %d: %s", updateResponse.Code, http.StatusOK, updateResponse.Body.String())
	}
	var updateBody struct {
		User Principal `json:"user"`
	}
	if err := json.NewDecoder(updateResponse.Body).Decode(&updateBody); err != nil {
		t.Fatalf("decode update user response: %v", err)
	}
	if updateBody.User.Username != "admin2" || updateBody.User.Email != "admin2@example.com" {
		t.Fatalf("update user response = %#v", updateBody.User)
	}
}

func TestHandlerChangesAndResetsPasswords(t *testing.T) {
	service := newTestService(t, "admin")
	handler := NewHandler(service)
	_, err := service.CreateUser(context.Background(), Principal{ID: 1, Username: "admin", Role: RoleSuperuser}, CreateUserInput{
		Username: "alice",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	changeRequest := authenticatedAuthRequest(http.MethodPatch, "/api/auth/me/password",
		`{"current_password":"admin","new_password":"admin-new"}`, Principal{ID: 1, Username: "admin", Role: RoleSuperuser})
	changeResponse := httptest.NewRecorder()
	handler.ChangeOwnPassword(changeResponse, changeRequest)
	if changeResponse.Code != http.StatusOK {
		t.Fatalf("change password status = %d, want %d: %s", changeResponse.Code, http.StatusOK, changeResponse.Body.String())
	}

	resetRequest := authenticatedAuthRequest(http.MethodPatch, "/api/auth/users/2/password",
		`{"password":"alice-new"}`, Principal{ID: 1, Username: "admin", Role: RoleSuperuser})
	resetRequest.SetPathValue("id", "2")
	resetResponse := httptest.NewRecorder()
	handler.ResetUserPassword(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("reset password status = %d, want %d: %s", resetResponse.Code, http.StatusOK, resetResponse.Body.String())
	}
	if _, err := service.Login(context.Background(), "alice", "alice-new"); err != nil {
		t.Fatalf("login with reset password: %v", err)
	}
}

func TestHandlerRejectsForbiddenUserCreation(t *testing.T) {
	handler := NewHandler(newTestService(t, "admin"))
	request := authenticatedAuthRequest(http.MethodPost, "/api/auth/users",
		`{"username":"alice","password":"secret"}`, Principal{ID: 2, Username: "bob", Role: RoleUser})
	response := httptest.NewRecorder()

	handler.CreateUser(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("create user status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestHandlerRejectsSuperuserCreation(t *testing.T) {
	handler := NewHandler(newTestService(t, "admin"))
	request := authenticatedAuthRequest(http.MethodPost, "/api/auth/users",
		`{"username":"root","password":"secret","role":"superuser"}`, Principal{ID: 1, Username: "admin", Role: RoleSuperuser})
	response := httptest.NewRecorder()

	handler.CreateUser(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("create superuser status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func authenticatedAuthRequest(method string, target string, body string, principal Principal) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	return request.WithContext(ContextWithPrincipal(request.Context(), principal))
}
