// This file exposes authentication REST endpoints.
package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/stark-lin/go-proj/internal/platform/httpx"
)

type Handler struct {
	service *Service
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

type UpdateOwnUserRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
}

type ChangeOwnPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type ResetUserPasswordRequest struct {
	Password string `json:"password"`
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
	var request UpdateOwnUserRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid account update request")
		return
	}
	user, err := h.service.UpdateOwnUser(r.Context(), principal, UpdateOwnUserInput{
		Username: request.Username,
		Email:    request.Email,
	})
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (h *Handler) ChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request ChangeOwnPasswordRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid password update request")
		return
	}
	err := h.service.ChangeOwnPassword(r.Context(), principal, ChangeOwnPasswordInput{
		CurrentPassword: request.CurrentPassword,
		NewPassword:     request.NewPassword,
	})
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"password_updated": true})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request CreateUserRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid user creation request")
		return
	}
	user, err := h.service.CreateUser(r.Context(), principal, CreateUserInput{
		Username: request.Username,
		Email:    request.Email,
		Password: request.Password,
		Role:     request.Role,
	})
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	targetUserID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || targetUserID < 1 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid user id")
		return
	}
	var request ResetUserPasswordRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid password reset request")
		return
	}
	if err := h.service.ResetUserPassword(r.Context(), principal, targetUserID, request.Password); h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"password_updated": true})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
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
	case errors.Is(err, ErrInvalidUser):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid account request")
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Superuser access is required")
	case errors.Is(err, ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "User not found")
	case errors.Is(err, ErrUserConflict):
		httpx.WriteError(w, http.StatusConflict, "conflict", "Username or email already exists")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "authentication_unavailable", "Authentication is unavailable")
	}
	return true
}
