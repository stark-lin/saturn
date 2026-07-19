// This file exposes administrator-session and API-key REST endpoints.
package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/stark-lin/saturn/internal/platform/httpx"
)

type Handler struct {
	service *Service
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UpdateAdministratorRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
}

type ChangeAdministratorPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type CreateAPIKeyRequest struct {
	Name      string      `json:"name"`
	Scopes    []ScopeName `json:"scopes"`
	ExpiresAt *time.Time  `json:"expires_at"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request LoginRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid login request")
		return
	}
	result, err := h.service.Login(r.Context(), request.Username, request.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "authentication_unavailable", "Authentication is unavailable")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": principal})
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request UpdateAdministratorRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid administrator update request")
		return
	}
	administrator, err := h.service.UpdateAdministrator(r.Context(), principal, UpdateAdministratorInput{
		Username: request.Username, Email: request.Email,
	})
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": administrator})
}

func (h *Handler) ChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request ChangeAdministratorPasswordRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid password update request")
		return
	}
	err := h.service.ChangeAdministratorPassword(r.Context(), principal, ChangeAdministratorPasswordInput{
		CurrentPassword: request.CurrentPassword, NewPassword: request.NewPassword,
	})
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"password_updated": true})
}

func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request CreateAPIKeyRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid API key request")
		return
	}
	result, err := h.service.CreateAPIKey(r.Context(), principal, CreateAPIKeyInput{
		Name: request.Name, Scopes: request.Scopes, ExpiresAt: request.ExpiresAt,
	})
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	keys, err := h.service.ListAPIKeys(r.Context(), principal)
	if h.writeServiceError(w, err) {
		return
	}
	if keys == nil {
		keys = []APIKey{}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"api_keys": keys})
}

func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	key, err := h.service.RevokeAPIKey(r.Context(), principal, r.PathValue("refcode"))
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, key)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	if !principal.IsAdministrator() {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Administrator access is required")
		return
	}
	token, err := BearerToken(r)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	if err := h.service.Logout(r.Context(), token); errors.Is(err, ErrUnauthenticated) {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	} else if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "authentication_unavailable", "Authentication is unavailable")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
}

func authenticatedPrincipal(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
	case errors.Is(err, ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
	case errors.Is(err, ErrInvalidAdministrator), errors.Is(err, ErrInvalidAPIKey):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid authentication request")
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Administrator access is required")
	case errors.Is(err, ErrAdministratorNotFound), errors.Is(err, ErrAPIKeyNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Authentication resource not found")
	case errors.Is(err, ErrAdministratorConflict), errors.Is(err, ErrAPIKeyConflict):
		httpx.WriteError(w, http.StatusConflict, "conflict", "Administrator or API key name already exists")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "authentication_unavailable", "Authentication is unavailable")
	}
	return true
}
