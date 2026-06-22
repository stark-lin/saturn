// This file tests authentication login, token, and session behavior.
package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestEnsureDevelopmentAdminCreatesUsableSuperuser(t *testing.T) {
	initializer := &recordingInitializer{}

	if err := EnsureDevelopmentAdmin(context.Background(), initializer); err != nil {
		t.Fatalf("ensure development admin: %v", err)
	}
	if initializer.username != "admin" || initializer.role != RoleSuperuser {
		t.Fatalf("created user = %q/%q, want admin/superuser", initializer.username, initializer.role)
	}
	if !VerifyPassword(initializer.passwordHash, "admin") {
		t.Fatal("expected stored development password hash to verify admin password")
	}
	if initializer.passwordHash == "admin" {
		t.Fatal("expected development password to be hashed")
	}
}

func TestServiceLoginAuthenticateAndLogout(t *testing.T) {
	service := newTestService(t, "admin")
	ctx := context.Background()

	result, err := service.Login(ctx, "admin", "admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.User.Username != "admin" || result.User.Role != RoleSuperuser || result.Token == "" {
		t.Fatalf("login result = %#v, want admin superuser token", result)
	}

	principal, err := service.Authenticate(ctx, result.Token)
	if err != nil {
		t.Fatalf("authenticate issued token: %v", err)
	}
	if principal.Username != "admin" || !principal.IsSuperuser() {
		t.Fatalf("principal = %#v, want admin superuser", principal)
	}

	if err := service.Logout(ctx, result.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.Authenticate(ctx, result.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("authenticate revoked token error = %v, want unauthenticated", err)
	}
}

func TestServiceLoginRejectsWrongPassword(t *testing.T) {
	service := newTestService(t, "admin")

	_, err := service.Login(context.Background(), "admin", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("login error = %v, want invalid credentials", err)
	}
}

func TestServiceAuditsAuthenticationOutcomes(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	service := newTestServiceWithAudit(t, "admin", recorder)

	result, err := service.Login(context.Background(), "admin", "admin")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := service.Login(context.Background(), "admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	if err := service.Logout(context.Background(), result.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	want := []string{"LOGIN/SUCCESS/1", "LOGIN/DENIED/0/invalid_credentials", "LOGOUT/SUCCESS/1"}
	if len(recorder.calls) != len(want) {
		t.Fatalf("audit calls = %#v, want %#v", recorder.calls, want)
	}
	for i := range want {
		if recorder.calls[i] != want[i] {
			t.Fatalf("audit calls = %#v, want %#v", recorder.calls, want)
		}
	}
}

func TestServiceCreateUserRequiresSuperuserAndHashesPassword(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	service := newTestServiceWithAudit(t, "admin", recorder)
	ctx := context.Background()

	createdUser, err := service.CreateUser(ctx, Principal{ID: 1, Username: "admin", Role: RoleSuperuser}, CreateUserInput{
		Username: " alice ",
		Email:    " alice@example.com ",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if createdUser.Username != "alice" || createdUser.Email != "alice@example.com" || createdUser.Role != RoleUser {
		t.Fatalf("created user = %#v", createdUser)
	}
	storedUser, err := service.repo.FindUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("find created user: %v", err)
	}
	if storedUser.PasswordHash == "secret" || !VerifyPassword(storedUser.PasswordHash, "secret") {
		t.Fatalf("stored password hash is not usable")
	}
	if len(recorder.calls) != 1 || recorder.calls[0] != "CREATE/SUCCESS/1/create_user" {
		t.Fatalf("audit calls = %#v", recorder.calls)
	}

	_, err = service.CreateUser(ctx, Principal{ID: 2, Username: "bob", Role: RoleUser}, CreateUserInput{
		Username: "blocked",
		Password: "secret",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-superuser create error = %v, want forbidden", err)
	}
}

func TestServiceCreateUserRejectsSuperuserRole(t *testing.T) {
	service := newTestService(t, "admin")
	ctx := context.Background()

	_, err := service.CreateUser(ctx, Principal{ID: 1, Username: "admin", Role: RoleSuperuser}, CreateUserInput{
		Username: "root",
		Password: "secret",
		Role:     RoleSuperuser,
	})
	if !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("create superuser error = %v, want invalid user", err)
	}
}

func TestServiceUpdateOwnUserAndChangeOwnPassword(t *testing.T) {
	service := newTestService(t, "admin")
	ctx := context.Background()
	actor := Principal{ID: 1, Username: "admin", Role: RoleSuperuser}
	username := " owner "
	email := " owner@example.com "

	updatedUser, err := service.UpdateOwnUser(ctx, actor, UpdateOwnUserInput{
		Username: &username,
		Email:    &email,
	})
	if err != nil {
		t.Fatalf("update own user: %v", err)
	}
	if updatedUser.Username != "owner" || updatedUser.Email != "owner@example.com" {
		t.Fatalf("updated user = %#v", updatedUser)
	}

	if err := service.ChangeOwnPassword(ctx, Principal{ID: 1, Username: "owner", Role: RoleSuperuser}, ChangeOwnPasswordInput{
		CurrentPassword: "admin",
		NewPassword:     "new-secret",
	}); err != nil {
		t.Fatalf("change own password: %v", err)
	}
	if _, err := service.Login(ctx, "owner", "new-secret"); err != nil {
		t.Fatalf("login with changed password: %v", err)
	}
}

func TestServiceChangeOwnPasswordRejectsWrongCurrentPassword(t *testing.T) {
	service := newTestService(t, "admin")

	err := service.ChangeOwnPassword(context.Background(), Principal{ID: 1, Username: "admin", Role: RoleSuperuser}, ChangeOwnPasswordInput{
		CurrentPassword: "wrong",
		NewPassword:     "new-secret",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("change password error = %v, want invalid credentials", err)
	}
}

func TestServiceOrdinaryUserCanChangeOwnPassword(t *testing.T) {
	service := newTestService(t, "admin")
	ctx := context.Background()
	_, err := service.CreateUser(ctx, Principal{ID: 1, Username: "admin", Role: RoleSuperuser}, CreateUserInput{
		Username: "alice",
		Password: "secret",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := service.ChangeOwnPassword(ctx, Principal{ID: 2, Username: "alice", Role: RoleUser}, ChangeOwnPasswordInput{
		CurrentPassword: "secret",
		NewPassword:     "user-new-secret",
	}); err != nil {
		t.Fatalf("ordinary user change own password: %v", err)
	}
	if _, err := service.Login(ctx, "alice", "user-new-secret"); err != nil {
		t.Fatalf("login with ordinary user changed password: %v", err)
	}
}

func TestServiceResetUserPasswordRules(t *testing.T) {
	service := newTestService(t, "admin")
	ctx := context.Background()
	_, err := service.CreateUser(ctx, Principal{ID: 1, Username: "admin", Role: RoleSuperuser}, CreateUserInput{
		Username: "alice",
		Password: "secret",
		Role:     RoleUser,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	passwordHash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("hash legacy superuser password: %v", err)
	}
	repo := service.repo.(*fakeRepository)
	repo.users["root"] = User{ID: 3, Username: "root", Role: RoleSuperuser, PasswordHash: passwordHash}
	repo.nextID = 4

	if err := service.ResetUserPassword(ctx, Principal{ID: 1, Username: "admin", Role: RoleSuperuser}, 2, "reset-secret"); err != nil {
		t.Fatalf("reset user password: %v", err)
	}
	if _, err := service.Login(ctx, "alice", "reset-secret"); err != nil {
		t.Fatalf("login with reset password: %v", err)
	}
	if err := service.ResetUserPassword(ctx, Principal{ID: 2, Username: "alice", Role: RoleUser}, 1, "blocked"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ordinary user reset error = %v, want forbidden", err)
	}
	if err := service.ResetUserPassword(ctx, Principal{ID: 1, Username: "admin", Role: RoleSuperuser}, 3, "blocked"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other superuser reset error = %v, want forbidden", err)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager, err := NewTokenManager("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	now := time.Date(2026, time.May, 24, 2, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	token, err := manager.Issue(Principal{ID: 1, Username: "admin", Role: RoleSuperuser})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	now = token.ExpiresAt
	if _, err := manager.Verify(token.Value); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("verify expired token error = %v, want invalid token", err)
	}
}

func TestTokenManagerRejectsTamperedToken(t *testing.T) {
	manager, err := NewTokenManager("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	token, err := manager.Issue(Principal{ID: 1, Username: "admin", Role: RoleSuperuser})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	tampered := "x" + token.Value[1:]
	if _, err := manager.Verify(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("verify tampered token error = %v, want invalid token", err)
	}
}

func newTestService(t *testing.T, username string) *Service {
	return newTestServiceWithAudit(t, username, nil)
}

func newTestServiceWithAudit(t *testing.T, username string, recorder AuditRecorder) *Service {
	t.Helper()
	passwordHash, err := HashPassword("admin")
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}
	repo := &fakeRepository{nextID: 2, users: map[string]User{
		username: {ID: 1, Username: username, Role: RoleSuperuser, PasswordHash: passwordHash},
	}}
	tokens, err := NewTokenManager("test-secret", time.Hour)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	return NewService(repo, &fakeSessions{active: make(map[string]bool)}, tokens, recorder)
}

type fakeAuditRecorder struct {
	calls []string
	err   error
}

func (r *fakeAuditRecorder) RecordAuthentication(_ context.Context, actorUserID int64, action string, result string, reason string) error {
	call := action + "/" + result + "/" + fmt.Sprint(actorUserID)
	if reason != "" {
		call += "/" + reason
	}
	r.calls = append(r.calls, call)
	return r.err
}

type fakeRepository struct {
	users  map[string]User
	nextID int64
}

func (r *fakeRepository) FindUserByUsername(_ context.Context, username string) (User, error) {
	user, ok := r.users[username]
	if !ok {
		return User{}, errors.New("not found")
	}
	return user, nil
}

func (r *fakeRepository) FindUserByID(_ context.Context, id int64) (User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}
	return User{}, errors.New("not found")
}

func (r *fakeRepository) CreateUser(_ context.Context, input CreateUserRecord) (User, error) {
	if _, ok := r.users[input.Username]; ok {
		return User{}, ErrUserConflict
	}
	for _, user := range r.users {
		if input.Email != "" && user.Email == input.Email {
			return User{}, ErrUserConflict
		}
	}
	if r.nextID == 0 {
		r.nextID = 1
	}
	user := User{
		ID:           r.nextID,
		Username:     input.Username,
		Email:        input.Email,
		Role:         input.Role,
		PasswordHash: input.PasswordHash,
	}
	r.nextID++
	r.users[user.Username] = user
	return user, nil
}

func (r *fakeRepository) UpdateUserProfile(_ context.Context, id int64, username string, email string) (User, error) {
	var current User
	var currentKey string
	for key, user := range r.users {
		if user.ID == id {
			current = user
			currentKey = key
			break
		}
	}
	if current.ID == 0 {
		return User{}, errors.New("not found")
	}
	for key, user := range r.users {
		if key != currentKey && user.Username == username {
			return User{}, ErrUserConflict
		}
		if key != currentKey && email != "" && user.Email == email {
			return User{}, ErrUserConflict
		}
	}
	delete(r.users, currentKey)
	current.Username = username
	current.Email = email
	r.users[current.Username] = current
	return current, nil
}

func (r *fakeRepository) UpdateUserPassword(_ context.Context, id int64, passwordHash string) (User, error) {
	for key, user := range r.users {
		if user.ID == id {
			user.PasswordHash = passwordHash
			r.users[key] = user
			return user, nil
		}
	}
	return User{}, errors.New("not found")
}

type fakeSessions struct {
	active map[string]bool
}

func (s *fakeSessions) Save(_ context.Context, session Session) error {
	s.active[session.ID] = true
	return nil
}

func (s *fakeSessions) Active(_ context.Context, sessionID string) (bool, error) {
	return s.active[sessionID], nil
}

func (s *fakeSessions) Delete(_ context.Context, sessionID string) error {
	delete(s.active, sessionID)
	return nil
}

type recordingInitializer struct {
	username     string
	role         Role
	passwordHash string
}

func (r *recordingInitializer) CreateUserIfMissing(_ context.Context, username string, role Role, passwordHash string) error {
	r.username = username
	r.role = role
	r.passwordHash = passwordHash
	return nil
}
