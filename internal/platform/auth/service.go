// This file implements authentication service orchestration.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidUser        = errors.New("invalid user")
	ErrUserConflict       = errors.New("user conflict")
	ErrUserNotFound       = errors.New("user not found")
)

type AuditRecorder interface {
	RecordAuthentication(ctx context.Context, actorUserID int64, action string, result string, reason string) error
}

type Service struct {
	repo     Repository
	sessions SessionStore
	tokens   *TokenManager
	audit    AuditRecorder
}

type LoginResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      Principal `json:"user"`
}

type CreateUserInput struct {
	Username string
	Email    string
	Password string
	Role     Role
}

type UpdateOwnUserInput struct {
	Username *string
	Email    *string
}

type ChangeOwnPasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

func NewService(repo Repository, sessions SessionStore, tokens *TokenManager, auditRecorder ...AuditRecorder) *Service {
	var recorder AuditRecorder
	if len(auditRecorder) > 0 {
		recorder = auditRecorder[0]
	}
	return &Service{repo: repo, sessions: sessions, tokens: tokens, audit: recorder}
}

func (s *Service) Login(ctx context.Context, username string, password string) (LoginResult, error) {
	if s.repo == nil || s.sessions == nil || s.tokens == nil {
		return LoginResult{}, fmt.Errorf("authentication service is not configured")
	}
	user, err := s.repo.FindUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil || !VerifyPassword(user.PasswordHash, password) {
		if auditErr := s.recordAuthentication(ctx, 0, "LOGIN", "DENIED", "invalid_credentials"); auditErr != nil {
			return LoginResult{}, auditErr
		}
		return LoginResult{}, ErrInvalidCredentials
	}
	principal := principalForUser(user)
	token, err := s.tokens.Issue(principal)
	if err != nil {
		_ = s.recordAuthentication(ctx, principal.ID, "LOGIN", "FAILED", "token_issue_failed")
		return LoginResult{}, err
	}
	if err := s.sessions.Save(ctx, Session{
		ID:        token.ID,
		UserID:    principal.ID,
		ExpiresAt: token.ExpiresAt,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		_ = s.recordAuthentication(ctx, principal.ID, "LOGIN", "FAILED", "session_save_failed")
		return LoginResult{}, err
	}
	if err := s.recordAuthentication(ctx, principal.ID, "LOGIN", "SUCCESS", ""); err != nil {
		_ = s.sessions.Delete(ctx, token.ID)
		return LoginResult{}, err
	}
	return LoginResult{Token: token.Value, ExpiresAt: token.ExpiresAt, User: principal}, nil
}

func (s *Service) CreateUser(ctx context.Context, actor Principal, input CreateUserInput) (Principal, error) {
	if s.repo == nil {
		return Principal{}, fmt.Errorf("authentication service is not configured")
	}
	if actor.IsZero() {
		return Principal{}, ErrUnauthenticated
	}
	if !actor.IsSuperuser() {
		_ = s.recordAuthentication(ctx, actor.ID, "CREATE", "DENIED", "forbidden")
		return Principal{}, ErrForbidden
	}
	input, err := normalizeCreateUserInput(input)
	if err != nil {
		return Principal{}, err
	}
	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return Principal{}, err
	}
	user, err := s.repo.CreateUser(ctx, CreateUserRecord{
		Username:     input.Username,
		Email:        input.Email,
		Role:         input.Role,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return Principal{}, err
	}
	if err := s.recordAuthentication(ctx, actor.ID, "CREATE", "SUCCESS", "create_user"); err != nil {
		return Principal{}, err
	}
	return principalForUser(user), nil
}

func (s *Service) UpdateOwnUser(ctx context.Context, actor Principal, input UpdateOwnUserInput) (Principal, error) {
	if s.repo == nil {
		return Principal{}, fmt.Errorf("authentication service is not configured")
	}
	if actor.IsZero() {
		return Principal{}, ErrUnauthenticated
	}
	currentUser, err := s.repo.FindUserByID(ctx, actor.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUserNotFound
	}
	if err != nil {
		return Principal{}, err
	}
	username := currentUser.Username
	if input.Username != nil {
		username = strings.TrimSpace(*input.Username)
		if username == "" {
			return Principal{}, ErrInvalidUser
		}
	}
	email := currentUser.Email
	if input.Email != nil {
		email = strings.TrimSpace(*input.Email)
	}
	if input.Username == nil && input.Email == nil {
		return principalForUser(currentUser), nil
	}
	updatedUser, err := s.repo.UpdateUserProfile(ctx, actor.ID, username, email)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUserNotFound
	}
	if err != nil {
		return Principal{}, err
	}
	if err := s.recordAuthentication(ctx, actor.ID, "UPDATE", "SUCCESS", "update_own_user"); err != nil {
		return Principal{}, err
	}
	return principalForUser(updatedUser), nil
}

func (s *Service) ChangeOwnPassword(ctx context.Context, actor Principal, input ChangeOwnPasswordInput) error {
	if s.repo == nil {
		return fmt.Errorf("authentication service is not configured")
	}
	if actor.IsZero() {
		return ErrUnauthenticated
	}
	if strings.TrimSpace(input.NewPassword) == "" {
		return ErrInvalidUser
	}
	user, err := s.repo.FindUserByID(ctx, actor.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if !VerifyPassword(user.PasswordHash, input.CurrentPassword) {
		_ = s.recordAuthentication(ctx, actor.ID, "UPDATE", "DENIED", "invalid_credentials")
		return ErrInvalidCredentials
	}
	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	if _, err := s.repo.UpdateUserPassword(ctx, actor.ID, passwordHash); errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	} else if err != nil {
		return err
	}
	return s.recordAuthentication(ctx, actor.ID, "UPDATE", "SUCCESS", "change_own_password")
}

func (s *Service) ResetUserPassword(ctx context.Context, actor Principal, targetUserID int64, password string) error {
	if s.repo == nil {
		return fmt.Errorf("authentication service is not configured")
	}
	if actor.IsZero() {
		return ErrUnauthenticated
	}
	if !actor.IsSuperuser() {
		_ = s.recordAuthentication(ctx, actor.ID, "UPDATE", "DENIED", "forbidden")
		return ErrForbidden
	}
	if targetUserID < 1 || strings.TrimSpace(password) == "" {
		return ErrInvalidUser
	}
	targetUser, err := s.repo.FindUserByID(ctx, targetUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if targetUser.Role == RoleSuperuser && targetUser.ID != actor.ID {
		_ = s.recordAuthentication(ctx, actor.ID, "UPDATE", "DENIED", "target_superuser")
		return ErrForbidden
	}
	passwordHash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := s.repo.UpdateUserPassword(ctx, targetUserID, passwordHash); errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	} else if err != nil {
		return err
	}
	return s.recordAuthentication(ctx, actor.ID, "UPDATE", "SUCCESS", "reset_user_password")
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (Principal, error) {
	if s.repo == nil || s.sessions == nil || s.tokens == nil {
		return Principal{}, ErrUnauthenticated
	}
	claims, err := s.tokens.Verify(rawToken)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	active, err := s.sessions.Active(ctx, claims.TokenID)
	if err != nil || !active {
		return Principal{}, ErrUnauthenticated
	}
	user, err := s.repo.FindUserByID(ctx, claims.UserID)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return principalForUser(user), nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if s.sessions == nil || s.tokens == nil {
		return nil
	}
	claims, err := s.tokens.Verify(rawToken)
	if err != nil {
		return ErrUnauthenticated
	}
	if err := s.sessions.Delete(ctx, claims.TokenID); err != nil {
		_ = s.recordAuthentication(ctx, claims.UserID, "LOGOUT", "FAILED", "session_delete_failed")
		return err
	}
	return s.recordAuthentication(ctx, claims.UserID, "LOGOUT", "SUCCESS", "")
}

func (s *Service) recordAuthentication(ctx context.Context, actorUserID int64, action string, result string, reason string) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.RecordAuthentication(ctx, actorUserID, action, result, reason)
}

func EnsureDevelopmentAdmin(ctx context.Context, repo UserInitializer) error {
	if repo == nil {
		return errors.New("auth user initializer is required")
	}
	passwordHash, err := HashPassword("admin")
	if err != nil {
		return fmt.Errorf("hash development admin password: %w", err)
	}
	if err := repo.CreateUserIfMissing(ctx, "admin", RoleSuperuser, passwordHash); err != nil {
		return fmt.Errorf("create development admin: %w", err)
	}
	return nil
}

func normalizeCreateUserInput(input CreateUserInput) (CreateUserInput, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	if input.Username == "" || strings.TrimSpace(input.Password) == "" {
		return CreateUserInput{}, ErrInvalidUser
	}
	if input.Role == "" {
		input.Role = RoleUser
	}
	switch input.Role {
	case RoleUser:
	default:
		return CreateUserInput{}, ErrInvalidUser
	}
	return input, nil
}

func principalForUser(user User) Principal {
	return Principal{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}
}
