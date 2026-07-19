// This file defines authentication persistence boundaries.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	authsqlc "github.com/stark-lin/saturn/internal/platform/auth/sqlc"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"

	"github.com/jackc/pgx/v5/pgconn"
)

type Repository interface {
	FindAdministratorByRefCode(ctx context.Context, refCode string) (User, error)
	FindAdministrator(ctx context.Context) (User, error)
	UpdateAdministratorEmail(ctx context.Context, email string) (User, error)
	UpdateAdministratorPassword(ctx context.Context, passwordHash string) (User, error)
	CreateAPIKey(ctx context.Context, input CreateAPIKeyRecord) (APIKey, error)
	UseAPIKey(ctx context.Context, keyHash string) (APIKey, error)
	FindAPIKeyByRefCode(ctx context.Context, refCode string) (APIKey, error)
	ListAPIKeys(ctx context.Context) ([]APIKey, error)
	RevokeAPIKey(ctx context.Context, refCode string) (APIKey, error)
}

type AdministratorInitializer interface {
	FindAdministrator(ctx context.Context) (User, error)
	CreateAdministrator(ctx context.Context, refCode string, passwordHash string) (User, error)
}

type CreateAPIKeyRecord struct {
	RefCode   string
	Name      string
	KeyPrefix string
	KeyHash   string
	Scopes    []ScopeName
	ExpiresAt *time.Time
}

type SQLRepository struct {
	database *sql.DB
	queries  *authsqlc.Queries
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{database: database, queries: authsqlc.New(database)}
}

func (r *SQLRepository) FindAdministratorByRefCode(ctx context.Context, refCode string) (User, error) {
	if err := r.validate(); err != nil {
		return User{}, err
	}
	row, err := r.queriesFor(ctx).FindAdministratorByRefCode(ctx, refCode)
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.RefCode, row.Email, row.PasswordHash), nil
}

func (r *SQLRepository) FindAdministrator(ctx context.Context) (User, error) {
	if err := r.validate(); err != nil {
		return User{}, err
	}
	row, err := r.queriesFor(ctx).FindAdministrator(ctx)
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.RefCode, row.Email, row.PasswordHash), nil
}

func (r *SQLRepository) CreateAdministrator(ctx context.Context, refCode string, passwordHash string) (User, error) {
	if err := r.validate(); err != nil {
		return User{}, err
	}
	row, err := r.queriesFor(ctx).CreateAdministrator(ctx, authsqlc.CreateAdministratorParams{
		RefCode: refCode, PasswordHash: passwordHash,
	})
	if isUniqueViolation(err) {
		return User{}, ErrAdministratorConflict
	}
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.RefCode, row.Email, row.PasswordHash), nil
}

func (r *SQLRepository) UpdateAdministratorEmail(ctx context.Context, email string) (User, error) {
	if err := r.validate(); err != nil {
		return User{}, err
	}
	row, err := r.queriesFor(ctx).UpdateAdministratorEmail(ctx, nullableString(email))
	if isUniqueViolation(err) {
		return User{}, ErrAdministratorConflict
	}
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.RefCode, row.Email, row.PasswordHash), nil
}

func (r *SQLRepository) UpdateAdministratorPassword(ctx context.Context, passwordHash string) (User, error) {
	if err := r.validate(); err != nil {
		return User{}, err
	}
	row, err := r.queriesFor(ctx).UpdateAdministratorPassword(ctx, passwordHash)
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.RefCode, row.Email, row.PasswordHash), nil
}

func (r *SQLRepository) CreateAPIKey(ctx context.Context, input CreateAPIKeyRecord) (APIKey, error) {
	if err := r.validate(); err != nil {
		return APIKey{}, err
	}
	row, err := r.queriesFor(ctx).CreateAPIKey(ctx, authsqlc.CreateAPIKeyParams{
		RefCode: input.RefCode, Name: input.Name, KeyPrefix: input.KeyPrefix, KeyHash: input.KeyHash,
		Scopes: scopeStrings(input.Scopes), ExpiresAt: nullableTime(input.ExpiresAt),
	})
	if isUniqueViolation(err) {
		return APIKey{}, ErrAPIKeyConflict
	}
	if err != nil {
		return APIKey{}, err
	}
	return apiKeyFromDatabaseFields(row.ID, row.RefCode, row.Name, row.KeyPrefix, row.KeyHash, row.Scopes, row.CreatedAt, row.LastUsedAt, row.ExpiresAt, row.RevokedAt), nil
}

func (r *SQLRepository) UseAPIKey(ctx context.Context, keyHash string) (APIKey, error) {
	if err := r.validate(); err != nil {
		return APIKey{}, err
	}
	row, err := r.queriesFor(ctx).UseAPIKey(ctx, keyHash)
	if err != nil {
		return APIKey{}, err
	}
	return apiKeyFromDatabaseFields(row.ID, row.RefCode, row.Name, row.KeyPrefix, row.KeyHash, row.Scopes, row.CreatedAt, row.LastUsedAt, row.ExpiresAt, row.RevokedAt), nil
}

func (r *SQLRepository) FindAPIKeyByRefCode(ctx context.Context, refCode string) (APIKey, error) {
	if err := r.validate(); err != nil {
		return APIKey{}, err
	}
	row, err := r.queriesFor(ctx).FindAPIKeyByRefCode(ctx, refCode)
	if err != nil {
		return APIKey{}, err
	}
	return apiKeyFromDatabaseFields(row.ID, row.RefCode, row.Name, row.KeyPrefix, row.KeyHash, row.Scopes, row.CreatedAt, row.LastUsedAt, row.ExpiresAt, row.RevokedAt), nil
}

func (r *SQLRepository) ListAPIKeys(ctx context.Context) ([]APIKey, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	rows, err := r.queriesFor(ctx).ListAPIKeys(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]APIKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, apiKeyFromDatabaseFields(row.ID, row.RefCode, row.Name, row.KeyPrefix, row.KeyHash, row.Scopes, row.CreatedAt, row.LastUsedAt, row.ExpiresAt, row.RevokedAt))
	}
	return keys, nil
}

func (r *SQLRepository) RevokeAPIKey(ctx context.Context, refCode string) (APIKey, error) {
	if err := r.validate(); err != nil {
		return APIKey{}, err
	}
	row, err := r.queriesFor(ctx).RevokeAPIKey(ctx, refCode)
	if err != nil {
		return APIKey{}, err
	}
	return apiKeyFromDatabaseFields(row.ID, row.RefCode, row.Name, row.KeyPrefix, row.KeyHash, row.Scopes, row.CreatedAt, row.LastUsedAt, row.ExpiresAt, row.RevokedAt), nil
}

func (r *SQLRepository) validate() error {
	if r == nil || r.database == nil || r.queries == nil {
		return fmt.Errorf("auth database is required")
	}
	return nil
}

func (r *SQLRepository) queriesFor(ctx context.Context) *authsqlc.Queries {
	return authsqlc.New(platformdb.ExecutorFromContext(ctx, r.database))
}

func userFromDatabaseFields(id int64, refCode string, email string, passwordHash string) User {
	return User{ID: id, RefCode: refCode, Email: email, PasswordHash: passwordHash}
}

func apiKeyFromDatabaseFields(id int64, refCode string, name string, keyPrefix string, keyHash string, scopes []string, createdAt time.Time, lastUsedAt sql.NullTime, expiresAt sql.NullTime, revokedAt sql.NullTime) APIKey {
	key := APIKey{
		ID: id, RefCode: refCode, Name: name, KeyPrefix: keyPrefix, KeyHash: keyHash,
		Scopes: scopeNames(scopes), CreatedAt: createdAt,
		LastUsedAt: timePointer(lastUsedAt), ExpiresAt: timePointer(expiresAt), RevokedAt: timePointer(revokedAt),
	}
	key.Status = key.EffectiveStatus(time.Now().UTC())
	return key
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	normalized := value.Time.UTC()
	return &normalized
}

func scopeStrings(scopes []ScopeName) []string {
	values := make([]string, len(scopes))
	for index, scope := range scopes {
		values[index] = string(scope)
	}
	return values
}

func scopeNames(scopes []string) []ScopeName {
	values := make([]ScopeName, len(scopes))
	for index, scope := range scopes {
		values[index] = ScopeName(scope)
	}
	return values
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
