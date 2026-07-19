// This file tests the single-administrator and API-key authentication model.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	platformdb "github.com/stark-lin/saturn/internal/platform/db"
)

func TestEnsureDevelopmentAdminCreatesHashedSingletonCredential(t *testing.T) {
	initializer := &recordingInitializer{}
	references := &fakeIdentityReferences{nextSequence: 0x2A}
	if err := EnsureDevelopmentAdmin(context.Background(), initializer, platformdb.NoopTransactionRunner{}, references); err != nil {
		t.Fatalf("ensure development administrator: %v", err)
	}
	if initializer.refCode != "USR-0000002A" || !VerifyPassword(initializer.passwordHash, "admin") {
		t.Fatalf("initializer = %#v, want usable hashed admin credential", initializer)
	}
	if len(references.registrations) != 1 || references.registrations[0].Kind != IdentityReferenceUser || references.registrations[0].RefCode != initializer.refCode {
		t.Fatalf("identity registrations = %#v, want claimed user reference", references.registrations)
	}
}

func TestServiceLoginAuthenticateAndLogoutAdministrator(t *testing.T) {
	service := newTestService(t, nil)
	result, err := service.Login(context.Background(), "admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.User.RefCode != AdministratorRefCode || result.User.Kind != PrincipalKindAdministrator || result.Token == "" {
		t.Fatalf("login result = %#v", result)
	}
	principal, err := service.Authenticate(context.Background(), result.Token)
	if err != nil || !principal.IsAdministrator() {
		t.Fatalf("authenticate principal = %#v, err = %v", principal, err)
	}
	if err := service.Logout(context.Background(), result.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.Authenticate(context.Background(), result.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("authenticate revoked token error = %v", err)
	}
}

func TestServiceCreatesAuthenticatesAndRevokesAPIKey(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	service := newTestService(t, recorder)
	service.credential = func() (string, error) {
		return "sat_sk_test-secret-value", nil
	}
	administrator := testAdministrator()
	result, err := service.CreateAPIKey(context.Background(), administrator, CreateAPIKeyInput{
		Name: "saturn-mcp", Scopes: []ScopeName{ScopeDataWrite, ScopeDataRead},
	})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	if result.APIKey != "sat_sk_test-secret-value" || result.RefCode != "KEY-4F8A2C10" {
		t.Fatalf("create api key result = %#v", result)
	}
	principal, err := service.Authenticate(context.Background(), result.APIKey)
	if err != nil {
		t.Fatalf("authenticate api key: %v", err)
	}
	if principal.RefCode != result.RefCode || principal.Kind != PrincipalKindAPIKey || principal.ID != 1 || !principal.Allows(ScopeDataWrite) {
		t.Fatalf("api key principal = %#v", principal)
	}
	keys, err := service.ListAPIKeys(context.Background(), administrator)
	if err != nil || len(keys) != 1 || keys[0].KeyHash != "" || keys[0].Status != APIKeyStatusActive {
		t.Fatalf("listed api keys = %#v, err = %v", keys, err)
	}
	revoked, err := service.RevokeAPIKey(context.Background(), administrator, result.RefCode)
	if err != nil || revoked.Status != APIKeyStatusRevoked {
		t.Fatalf("revoked key = %#v, err = %v", revoked, err)
	}
	if _, err := service.Authenticate(context.Background(), result.APIKey); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("authenticate revoked key error = %v", err)
	}
	if len(recorder.actions) != 2 || recorder.actions[0].actorRefCode != AdministratorRefCode || recorder.actions[0].targetRefCode != result.RefCode {
		t.Fatalf("audit actions = %#v", recorder.actions)
	}
	references := service.references.(*fakeIdentityReferences)
	if len(references.registrations) != 1 || references.registrations[0].Kind != IdentityReferenceAPIKey || references.registrations[0].ObjectID != 1 {
		t.Fatalf("identity registrations = %#v", references.registrations)
	}
	if len(references.projections) != 1 || references.projections[0].Status != string(APIKeyStatusRevoked) {
		t.Fatalf("identity projections = %#v", references.projections)
	}
}

func TestServiceRejectsInvalidOrExpiredAPIKeyConfiguration(t *testing.T) {
	service := newTestService(t, nil)
	service.credential = func() (string, error) { return "sat_sk_secret", nil }
	expired := service.now().Add(-time.Minute)
	for _, input := range []CreateAPIKeyInput{
		{Name: "Invalid Name", Scopes: []ScopeName{ScopeDataRead}},
		{Name: "missing-scope"},
		{Name: "unknown-scope", Scopes: []ScopeName{"admin"}},
		{Name: "expired", Scopes: []ScopeName{ScopeDataRead}, ExpiresAt: &expired},
	} {
		if _, err := service.CreateAPIKey(context.Background(), testAdministrator(), input); !errors.Is(err, ErrInvalidAPIKey) {
			t.Fatalf("input %#v error = %v, want invalid api key", input, err)
		}
	}
}

func TestServiceAPIKeyNameCannotBeReusedAfterRevocation(t *testing.T) {
	service := newTestService(t, nil)
	sequence := 0
	service.credential = func() (string, error) {
		sequence++
		if sequence == 1 {
			return "sat_sk_first-secret-value", nil
		}
		return "sat_sk_second-secret-value", nil
	}
	first, err := service.CreateAPIKey(context.Background(), testAdministrator(), CreateAPIKeyInput{Name: "backup", Scopes: []ScopeName{ScopeDataRead}})
	if err != nil {
		t.Fatalf("create first key: %v", err)
	}
	if _, err := service.RevokeAPIKey(context.Background(), testAdministrator(), first.RefCode); err != nil {
		t.Fatalf("revoke first key: %v", err)
	}
	if _, err := service.CreateAPIKey(context.Background(), testAdministrator(), CreateAPIKeyInput{Name: "backup", Scopes: []ScopeName{ScopeDataRead}}); !errors.Is(err, ErrAPIKeyConflict) {
		t.Fatalf("reuse revoked name error = %v, want conflict", err)
	}
}

func TestServiceExpiredAPIKeyCannotAuthenticateAndListsAsExpired(t *testing.T) {
	service := newTestService(t, nil)
	service.credential = func() (string, error) {
		return "sat_sk_expiring-secret-value", nil
	}
	expiresAt := service.now().Add(time.Minute)
	created, err := service.CreateAPIKey(context.Background(), testAdministrator(), CreateAPIKeyInput{
		Name: "expiring", Scopes: []ScopeName{ScopeDataRead}, ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatalf("create expiring API key: %v", err)
	}

	repo := service.repo.(*fakeRepository)
	key := repo.apiKeys[created.RefCode]
	expiredAt := service.now().Add(-time.Minute)
	key.ExpiresAt = &expiredAt
	repo.apiKeys[created.RefCode] = key

	if _, err := service.Authenticate(context.Background(), created.APIKey); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired API key authentication error = %v", err)
	}
	keys, err := service.ListAPIKeys(context.Background(), testAdministrator())
	if err != nil || len(keys) != 1 || keys[0].Status != APIKeyStatusExpired {
		t.Fatalf("expired API key list = %#v, error = %v", keys, err)
	}
}

func TestServiceUpdatesOnlyAdministratorAccount(t *testing.T) {
	service := newTestService(t, nil)
	email := " owner@example.com "
	updated, err := service.UpdateAdministrator(context.Background(), testAdministrator(), UpdateAdministratorInput{Email: &email})
	if err != nil || updated.Email != "owner@example.com" {
		t.Fatalf("updated administrator = %#v, err = %v", updated, err)
	}
	apiKey := Principal{ID: 1, RefCode: "KEY-00000001", Kind: PrincipalKindAPIKey, Scopes: []ScopeName{ScopeDataWrite}}
	if _, err := service.UpdateAdministrator(context.Background(), apiKey, UpdateAdministratorInput{Email: &email}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("api key update administrator error = %v", err)
	}
}

func TestTokenManagerRejectsExpiredAndTamperedTokens(t *testing.T) {
	manager, err := NewTokenManager("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	now := time.Date(2026, time.July, 19, 2, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	token, err := manager.Issue(Principal{ID: 1, RefCode: "USR-0000002A", Kind: PrincipalKindAdministrator})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if _, err := manager.Verify("x" + token.Value[1:]); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered token error = %v", err)
	}
	claims, err := manager.Verify(token.Value)
	if err != nil || claims.RefCode != "USR-0000002A" {
		t.Fatalf("claimed administrator token = %#v, error = %v", claims, err)
	}
	now = token.ExpiresAt
	if _, err := manager.Verify(token.Value); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired token error = %v", err)
	}
}

func newTestService(t *testing.T, recorder AuditRecorder) *Service {
	t.Helper()
	passwordHash, err := HashPassword("admin")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repo := &fakeRepository{
		administrator: User{ID: 1, RefCode: AdministratorRefCode, PasswordHash: passwordHash},
		apiKeys:       make(map[string]APIKey),
	}
	tokens, err := NewTokenManager("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	service := NewService(repo, &fakeSessions{active: make(map[string]bool)}, tokens, recorder)
	service.references = &fakeIdentityReferences{nextSequence: 0x4F8A2C10}
	service.now = func() time.Time { return time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC) }
	return service
}

func testAdministrator() Principal {
	return Principal{ID: 1, RefCode: AdministratorRefCode, Kind: PrincipalKindAdministrator}
}

type fakeRepository struct {
	administrator User
	apiKeys       map[string]APIKey
	usedRefCode   string
}

func (r *fakeRepository) FindAdministratorByRefCode(_ context.Context, refCode string) (User, error) {
	if r.administrator.RefCode != refCode {
		return User{}, sql.ErrNoRows
	}
	return r.administrator, nil
}

func (r *fakeRepository) FindAdministrator(context.Context) (User, error) {
	return r.administrator, nil
}

func (r *fakeRepository) UpdateAdministratorEmail(_ context.Context, email string) (User, error) {
	r.administrator.Email = email
	return r.administrator, nil
}

func (r *fakeRepository) UpdateAdministratorPassword(_ context.Context, passwordHash string) (User, error) {
	r.administrator.PasswordHash = passwordHash
	return r.administrator, nil
}

func (r *fakeRepository) CreateAPIKey(_ context.Context, input CreateAPIKeyRecord) (APIKey, error) {
	for _, existing := range r.apiKeys {
		if existing.Name == input.Name || existing.RefCode == input.RefCode || existing.KeyHash == input.KeyHash {
			return APIKey{}, ErrAPIKeyConflict
		}
	}
	key := APIKey{ID: int64(len(r.apiKeys) + 1), RefCode: input.RefCode, Name: input.Name, KeyPrefix: input.KeyPrefix, KeyHash: input.KeyHash, Scopes: input.Scopes, CreatedAt: time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC), ExpiresAt: input.ExpiresAt}
	r.apiKeys[key.RefCode] = key
	return key, nil
}

func (r *fakeRepository) UseAPIKey(_ context.Context, hash string) (APIKey, error) {
	for _, key := range r.apiKeys {
		if key.KeyHash == hash && key.RevokedAt == nil && (key.ExpiresAt == nil || key.ExpiresAt.After(time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC))) {
			r.usedRefCode = key.RefCode
			return key, nil
		}
	}
	return APIKey{}, sql.ErrNoRows
}

func (r *fakeRepository) FindAPIKeyByRefCode(_ context.Context, refCode string) (APIKey, error) {
	key, ok := r.apiKeys[refCode]
	if !ok {
		return APIKey{}, sql.ErrNoRows
	}
	return key, nil
}

func (r *fakeRepository) ListAPIKeys(context.Context) ([]APIKey, error) {
	keys := make([]APIKey, 0, len(r.apiKeys))
	for _, key := range r.apiKeys {
		keys = append(keys, key)
	}
	return keys, nil
}

func (r *fakeRepository) RevokeAPIKey(_ context.Context, refCode string) (APIKey, error) {
	key, ok := r.apiKeys[refCode]
	if !ok {
		return APIKey{}, sql.ErrNoRows
	}
	now := time.Date(2026, time.July, 19, 11, 0, 0, 0, time.UTC)
	key.RevokedAt = &now
	r.apiKeys[refCode] = key
	return key, nil
}

type fakeSessions struct{ active map[string]bool }

func (s *fakeSessions) Save(_ context.Context, session Session) error {
	s.active[session.ID] = true
	return nil
}
func (s *fakeSessions) Active(_ context.Context, id string) (bool, error) { return s.active[id], nil }
func (s *fakeSessions) Delete(_ context.Context, id string) error         { delete(s.active, id); return nil }

type recordedActorAction struct{ actorRefCode, action, targetRefCode, result, reason string }
type fakeAuditRecorder struct {
	authentication []recordedActorAction
	actions        []recordedActorAction
	err            error
}

func (r *fakeAuditRecorder) RecordAuthentication(_ context.Context, actorRefCode, action, result, reason string) error {
	r.authentication = append(r.authentication, recordedActorAction{actorRefCode: actorRefCode, action: action, result: result, reason: reason})
	return r.err
}

func (r *fakeAuditRecorder) RecordActorAction(_ context.Context, actorRefCode, action, targetRefCode, result, reason string) error {
	r.actions = append(r.actions, recordedActorAction{actorRefCode: actorRefCode, action: action, targetRefCode: targetRefCode, result: result, reason: reason})
	return r.err
}

type recordingInitializer struct {
	administrator User
	refCode       string
	passwordHash  string
}

func (r *recordingInitializer) FindAdministrator(context.Context) (User, error) {
	if r.administrator.ID == 0 {
		return User{}, sql.ErrNoRows
	}
	return r.administrator, nil
}

func (r *recordingInitializer) CreateAdministrator(_ context.Context, refCode string, passwordHash string) (User, error) {
	r.refCode, r.passwordHash = refCode, passwordHash
	r.administrator = User{ID: 1, RefCode: refCode, PasswordHash: passwordHash}
	return r.administrator, nil
}

type fakeIdentityReferences struct {
	nextSequence  int64
	registrations []IdentityReferenceRegistration
	projections   []IdentityReferenceProjection
}

func (r *fakeIdentityReferences) ClaimIdentityCode(_ context.Context, kind IdentityReferenceKind) (string, error) {
	sequence := r.nextSequence
	r.nextSequence++
	switch kind {
	case IdentityReferenceUser:
		return fmt.Sprintf("USR-%08X", sequence), nil
	case IdentityReferenceAPIKey:
		return fmt.Sprintf("KEY-%08X", sequence), nil
	default:
		return "", errors.New("unsupported identity reference kind")
	}
}

func (r *fakeIdentityReferences) RegisterIdentity(_ context.Context, registration IdentityReferenceRegistration) error {
	r.registrations = append(r.registrations, registration)
	return nil
}

func (r *fakeIdentityReferences) UpdateIdentityProjection(_ context.Context, projection IdentityReferenceProjection) error {
	r.projections = append(r.projections, projection)
	return nil
}
