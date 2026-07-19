// This file implements administrator sessions and API key lifecycle orchestration.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	platformdb "github.com/stark-lin/saturn/internal/platform/db"
)

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrInvalidAdministrator  = errors.New("invalid administrator")
	ErrAdministratorConflict = errors.New("administrator conflict")
	ErrAdministratorNotFound = errors.New("administrator not found")
	ErrInvalidAPIKey         = errors.New("invalid api key")
	ErrAPIKeyConflict        = errors.New("api key conflict")
	ErrAPIKeyNotFound        = errors.New("api key not found")
)

var apiKeyNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type AuditRecorder interface {
	RecordAuthentication(ctx context.Context, actorRefCode string, action string, result string, reason string) error
	RecordActorAction(ctx context.Context, actorRefCode string, action string, targetRefCode string, result string, reason string) error
}

type Service struct {
	repo         Repository
	sessions     SessionStore
	tokens       *TokenManager
	audit        AuditRecorder
	references   IdentityReferenceRegistry
	transactions platformdb.TransactionRunner
	now          func() time.Time
	credential   func() (string, error)
}

func NewServiceWithTransactions(repo Repository, sessions SessionStore, tokens *TokenManager, transactions platformdb.TransactionRunner, references IdentityReferenceRegistry, auditRecorder AuditRecorder) *Service {
	service := NewService(repo, sessions, tokens, auditRecorder)
	service.transactions = transactions
	service.references = references
	return service
}

type LoginResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      Principal `json:"user"`
}

type UpdateAdministratorInput struct {
	Email *string
}

type ChangeAdministratorPasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

type CreateAPIKeyInput struct {
	Name      string
	Scopes    []ScopeName
	ExpiresAt *time.Time
}

type CreateAPIKeyResult struct {
	RefCode   string      `json:"refcode"`
	Name      string      `json:"name"`
	APIKey    string      `json:"api_key"`
	KeyPrefix string      `json:"key_prefix"`
	Scopes    []ScopeName `json:"scopes"`
	ExpiresAt *time.Time  `json:"expires_at"`
}

func NewService(repo Repository, sessions SessionStore, tokens *TokenManager, auditRecorder ...AuditRecorder) *Service {
	var recorder AuditRecorder
	if len(auditRecorder) > 0 {
		recorder = auditRecorder[0]
	}
	return &Service{
		repo: repo, sessions: sessions, tokens: tokens, audit: recorder,
		now: time.Now, credential: generateAPIKeyCredential,
	}
}

func (s *Service) Login(ctx context.Context, password string) (LoginResult, error) {
	if s.repo == nil || s.sessions == nil || s.tokens == nil {
		return LoginResult{}, fmt.Errorf("authentication service is not configured")
	}
	user, err := s.repo.FindAdministrator(ctx)
	if err != nil || !VerifyPassword(user.PasswordHash, password) {
		if auditErr := s.recordAuthentication(ctx, SystemActorRefCode, "LOGIN", "DENIED", "invalid_credentials"); auditErr != nil {
			return LoginResult{}, auditErr
		}
		return LoginResult{}, ErrInvalidCredentials
	}
	principal := principalForAdministrator(user)
	token, err := s.tokens.Issue(principal)
	if err != nil {
		_ = s.recordAuthentication(ctx, principal.ActorRefCode(), "LOGIN", "FAILED", "token_issue_failed")
		return LoginResult{}, err
	}
	if err := s.sessions.Save(ctx, Session{
		ID: token.ID, AdministratorRefCode: principal.ActorRefCode(),
		ExpiresAt: token.ExpiresAt, CreatedAt: s.now().UTC(),
	}); err != nil {
		_ = s.recordAuthentication(ctx, principal.ActorRefCode(), "LOGIN", "FAILED", "session_save_failed")
		return LoginResult{}, err
	}
	if err := s.recordAuthentication(ctx, principal.ActorRefCode(), "LOGIN", "SUCCESS", ""); err != nil {
		_ = s.sessions.Delete(ctx, token.ID)
		return LoginResult{}, err
	}
	return LoginResult{Token: token.Value, ExpiresAt: token.ExpiresAt, User: principal}, nil
}

func (s *Service) UpdateAdministrator(ctx context.Context, actor Principal, input UpdateAdministratorInput) (Principal, error) {
	if err := requireAdministrator(actor); err != nil {
		return Principal{}, err
	}
	current, err := s.repo.FindAdministrator(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrAdministratorNotFound
	}
	if err != nil {
		return Principal{}, err
	}
	email := current.Email
	if input.Email != nil {
		email = strings.TrimSpace(*input.Email)
	}
	if input.Email == nil {
		return principalForAdministrator(current), nil
	}
	var updated User
	err = s.withinMutation(ctx, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.repo.UpdateAdministratorEmail(txCtx, email)
		if updateErr != nil {
			return updateErr
		}
		return s.recordActorAction(txCtx, actor.ActorRefCode(), "UPDATE", current.RefCode, "SUCCESS", "update_administrator")
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrAdministratorNotFound
	}
	if err != nil {
		return Principal{}, err
	}
	return principalForAdministrator(updated), nil
}

func (s *Service) ChangeAdministratorPassword(ctx context.Context, actor Principal, input ChangeAdministratorPasswordInput) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	if strings.TrimSpace(input.NewPassword) == "" {
		return ErrInvalidAdministrator
	}
	user, err := s.repo.FindAdministrator(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAdministratorNotFound
	}
	if err != nil {
		return err
	}
	if !VerifyPassword(user.PasswordHash, input.CurrentPassword) {
		_ = s.recordActorAction(ctx, actor.ActorRefCode(), "UPDATE", user.RefCode, "DENIED", "invalid_credentials")
		return ErrInvalidCredentials
	}
	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	err = s.withinMutation(ctx, func(txCtx context.Context) error {
		if _, updateErr := s.repo.UpdateAdministratorPassword(txCtx, passwordHash); updateErr != nil {
			return updateErr
		}
		return s.recordActorAction(txCtx, actor.ActorRefCode(), "UPDATE", user.RefCode, "SUCCESS", "change_administrator_password")
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAdministratorNotFound
	}
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) CreateAPIKey(ctx context.Context, actor Principal, input CreateAPIKeyInput) (CreateAPIKeyResult, error) {
	if err := requireAdministrator(actor); err != nil {
		return CreateAPIKeyResult{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Scopes = normalizeScopes(input.Scopes)
	if !apiKeyNamePattern.MatchString(input.Name) || len(input.Scopes) == 0 || !validScopes(input.Scopes) {
		return CreateAPIKeyResult{}, ErrInvalidAPIKey
	}
	now := s.now().UTC()
	if input.ExpiresAt != nil {
		expiresAt := input.ExpiresAt.UTC()
		if !expiresAt.After(now) {
			return CreateAPIKeyResult{}, ErrInvalidAPIKey
		}
		input.ExpiresAt = &expiresAt
	}
	if s.references == nil {
		return CreateAPIKeyResult{}, fmt.Errorf("identity reference registry is required")
	}
	refCode, err := s.references.ClaimIdentityCode(ctx, IdentityReferenceAPIKey)
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	plaintext, err := s.credential()
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	if !ValidAPIKeyRefCode(refCode) || !strings.HasPrefix(plaintext, apiKeySecretPrefix) || len(plaintext) <= len(apiKeySecretPrefix)+8 {
		return CreateAPIKeyResult{}, ErrInvalidAPIKey
	}
	digest := sha256.Sum256([]byte(plaintext))
	keyPrefix := plaintext
	if len(keyPrefix) > len(apiKeySecretPrefix)+8 {
		keyPrefix = keyPrefix[:len(apiKeySecretPrefix)+8]
	}
	var created APIKey
	err = s.withinMutation(ctx, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.repo.CreateAPIKey(txCtx, CreateAPIKeyRecord{
			RefCode: refCode, Name: input.Name, KeyPrefix: keyPrefix,
			KeyHash: hex.EncodeToString(digest[:]), Scopes: input.Scopes, ExpiresAt: input.ExpiresAt,
		})
		if createErr != nil {
			return createErr
		}
		if createErr = s.references.RegisterIdentity(txCtx, IdentityReferenceRegistration{
			RefCode: created.RefCode, Kind: IdentityReferenceAPIKey, ObjectID: created.ID,
			Title: created.Name, Status: string(APIKeyStatusActive),
		}); createErr != nil {
			return createErr
		}
		return s.recordActorAction(txCtx, actor.ActorRefCode(), "CREATE", created.RefCode, "SUCCESS", "create_api_key")
	})
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	return CreateAPIKeyResult{
		RefCode: created.RefCode, Name: created.Name, APIKey: plaintext,
		KeyPrefix: created.KeyPrefix, Scopes: created.Scopes, ExpiresAt: created.ExpiresAt,
	}, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, actor Principal) ([]APIKey, error) {
	if err := requireAdministrator(actor); err != nil {
		return nil, err
	}
	keys, err := s.repo.ListAPIKeys(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	for index := range keys {
		keys[index].Status = keys[index].EffectiveStatus(now)
		keys[index].KeyHash = ""
	}
	return keys, nil
}

func (s *Service) RevokeAPIKey(ctx context.Context, actor Principal, refCode string) (APIKey, error) {
	if err := requireAdministrator(actor); err != nil {
		return APIKey{}, err
	}
	refCode = strings.ToUpper(strings.TrimSpace(refCode))
	if !ValidAPIKeyRefCode(refCode) {
		return APIKey{}, ErrInvalidAPIKey
	}
	var key APIKey
	err := s.withinMutation(ctx, func(txCtx context.Context) error {
		var revokeErr error
		key, revokeErr = s.repo.RevokeAPIKey(txCtx, refCode)
		if revokeErr != nil {
			return revokeErr
		}
		if s.references == nil {
			return fmt.Errorf("identity reference registry is required")
		}
		if revokeErr = s.references.UpdateIdentityProjection(txCtx, IdentityReferenceProjection{
			Kind: IdentityReferenceAPIKey, ObjectID: key.ID,
			Title: key.Name, Status: string(APIKeyStatusRevoked),
		}); revokeErr != nil {
			return revokeErr
		}
		return s.recordActorAction(txCtx, actor.ActorRefCode(), "UPDATE", key.RefCode, "SUCCESS", "revoke_api_key")
	})
	if errors.Is(err, sql.ErrNoRows) {
		return APIKey{}, ErrAPIKeyNotFound
	}
	if err != nil {
		return APIKey{}, err
	}
	key.Status = APIKeyStatusRevoked
	key.KeyHash = ""
	return key, nil
}

func (s *Service) Authenticate(ctx context.Context, rawCredential string) (Principal, error) {
	if s.repo == nil {
		return Principal{}, ErrUnauthenticated
	}
	if strings.HasPrefix(rawCredential, apiKeySecretPrefix) {
		return s.authenticateAPIKey(ctx, rawCredential)
	}
	return s.authenticateAdministratorSession(ctx, rawCredential)
}

func (s *Service) authenticateAdministratorSession(ctx context.Context, rawToken string) (Principal, error) {
	if s.sessions == nil || s.tokens == nil {
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
	user, err := s.repo.FindAdministratorByRefCode(ctx, claims.RefCode)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return principalForAdministrator(user), nil
}

func (s *Service) authenticateAPIKey(ctx context.Context, plaintext string) (Principal, error) {
	digest := sha256.Sum256([]byte(plaintext))
	key, err := s.repo.UseAPIKey(ctx, hex.EncodeToString(digest[:]))
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	administrator, err := s.repo.FindAdministrator(ctx)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{
		ID: administrator.ID, RefCode: key.RefCode, Kind: PrincipalKindAPIKey,
		Name: key.Name, Scopes: slices.Clone(key.Scopes),
	}, nil
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
		_ = s.recordAuthentication(ctx, claims.RefCode, "LOGOUT", "FAILED", "session_delete_failed")
		return err
	}
	return s.recordAuthentication(ctx, claims.RefCode, "LOGOUT", "SUCCESS", "")
}

func (s *Service) recordAuthentication(ctx context.Context, actorRefCode string, action string, result string, reason string) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.RecordAuthentication(ctx, actorRefCode, action, result, reason)
}

func (s *Service) recordActorAction(ctx context.Context, actorRefCode string, action string, targetRefCode string, result string, reason string) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.RecordActorAction(ctx, actorRefCode, action, targetRefCode, result, reason)
}

func (s *Service) withinMutation(ctx context.Context, operation func(context.Context) error) error {
	if s.transactions == nil {
		return operation(ctx)
	}
	return s.transactions.WithinTransaction(ctx, operation)
}

func EnsureDevelopmentAdmin(ctx context.Context, repo AdministratorInitializer, transactions platformdb.TransactionRunner, references IdentityReferenceRegistry) error {
	if repo == nil || references == nil {
		return errors.New("auth administrator initializer and identity reference registry are required")
	}
	if _, err := repo.FindAdministrator(ctx); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find development administrator: %w", err)
	}
	passwordHash, err := HashPassword("admin")
	if err != nil {
		return fmt.Errorf("hash development administrator password: %w", err)
	}
	create := func(txCtx context.Context) error {
		refCode, claimErr := references.ClaimIdentityCode(txCtx, IdentityReferenceUser)
		if claimErr != nil {
			return claimErr
		}
		user, createErr := repo.CreateAdministrator(txCtx, refCode, passwordHash)
		if createErr != nil {
			return createErr
		}
		return references.RegisterIdentity(txCtx, IdentityReferenceRegistration{
			RefCode: user.RefCode, Kind: IdentityReferenceUser, ObjectID: user.ID,
			Title: "Administrator", Status: "active",
		})
	}
	if transactions != nil {
		err = transactions.WithinTransaction(ctx, create)
	} else {
		err = create(ctx)
	}
	if err != nil {
		return fmt.Errorf("create development administrator: %w", err)
	}
	return nil
}

func principalForAdministrator(user User) Principal {
	return Principal{
		ID: user.ID, RefCode: user.RefCode, Kind: PrincipalKindAdministrator,
		Email: user.Email,
	}
}

func requireAdministrator(actor Principal) error {
	if actor.IsZero() {
		return ErrUnauthenticated
	}
	if !actor.IsAdministrator() || !ValidUserRefCode(actor.ActorRefCode()) {
		return ErrForbidden
	}
	return nil
}

func normalizeScopes(scopes []ScopeName) []ScopeName {
	unique := make(map[ScopeName]struct{}, len(scopes))
	for _, scope := range scopes {
		value := ScopeName(strings.TrimSpace(string(scope)))
		if value != "" {
			unique[value] = struct{}{}
		}
	}
	result := make([]ScopeName, 0, len(unique))
	for scope := range unique {
		result = append(result, scope)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func validScopes(scopes []ScopeName) bool {
	for _, scope := range scopes {
		if !slices.Contains(SupportedAPIKeyScopes, scope) {
			return false
		}
	}
	return true
}

func generateAPIKeyCredential() (string, error) {
	var secretBytes [32]byte
	if _, err := rand.Read(secretBytes[:]); err != nil {
		return "", fmt.Errorf("generate api key secret: %w", err)
	}
	return apiKeySecretPrefix + base64.RawURLEncoding.EncodeToString(secretBytes[:]), nil
}

const SystemActorRefCode = "SYS-00000000"
