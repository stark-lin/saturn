// This file defines authentication persistence boundaries.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	authsqlc "github.com/stark-lin/go-proj/internal/platform/auth/sqlc"

	"github.com/jackc/pgx/v5/pgconn"
)

type Repository interface {
	FindUserByUsername(ctx context.Context, username string) (User, error)
	FindUserByID(ctx context.Context, id int64) (User, error)
	CreateUser(ctx context.Context, input CreateUserRecord) (User, error)
	UpdateUserProfile(ctx context.Context, id int64, username string, email string) (User, error)
	UpdateUserPassword(ctx context.Context, id int64, passwordHash string) (User, error)
}

type UserInitializer interface {
	CreateUserIfMissing(ctx context.Context, username string, role Role, passwordHash string) error
}

type CreateUserRecord struct {
	Username     string
	Email        string
	Role         Role
	PasswordHash string
}

type SQLRepository struct {
	database *sql.DB
	queries  *authsqlc.Queries
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{
		database: database,
		queries:  authsqlc.New(database),
	}
}

func (r *SQLRepository) FindUserByUsername(ctx context.Context, username string) (User, error) {
	if r == nil || r.database == nil {
		return User{}, fmt.Errorf("auth database is required")
	}
	row, err := r.queries.FindUserByUsername(ctx, username)
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.Username, row.Email, row.Role, row.PasswordHash), nil
}

func (r *SQLRepository) FindUserByID(ctx context.Context, id int64) (User, error) {
	if r == nil || r.database == nil {
		return User{}, fmt.Errorf("auth database is required")
	}
	row, err := r.queries.FindUserByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.Username, row.Email, row.Role, row.PasswordHash), nil
}

func (r *SQLRepository) CreateUser(ctx context.Context, input CreateUserRecord) (User, error) {
	if r == nil || r.database == nil {
		return User{}, fmt.Errorf("auth database is required")
	}
	row, err := r.queries.CreateUser(ctx, authsqlc.CreateUserParams{
		Username:     input.Username,
		Email:        nullableString(input.Email),
		Role:         string(input.Role),
		PasswordHash: input.PasswordHash,
	})
	if isUniqueViolation(err) {
		return User{}, ErrUserConflict
	}
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.Username, row.Email, row.Role, row.PasswordHash), nil
}

func (r *SQLRepository) CreateUserIfMissing(ctx context.Context, username string, role Role, passwordHash string) error {
	if r == nil || r.database == nil {
		return fmt.Errorf("auth database is required")
	}
	return r.queries.CreateUserIfMissing(ctx, authsqlc.CreateUserIfMissingParams{
		Username:     username,
		Role:         string(role),
		PasswordHash: passwordHash,
	})
}

func (r *SQLRepository) UpdateUserProfile(ctx context.Context, id int64, username string, email string) (User, error) {
	if r == nil || r.database == nil {
		return User{}, fmt.Errorf("auth database is required")
	}
	row, err := r.queries.UpdateUserProfile(ctx, authsqlc.UpdateUserProfileParams{
		ID:       id,
		Username: username,
		Email:    nullableString(email),
	})
	if isUniqueViolation(err) {
		return User{}, ErrUserConflict
	}
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.Username, row.Email, row.Role, row.PasswordHash), nil
}

func (r *SQLRepository) UpdateUserPassword(ctx context.Context, id int64, passwordHash string) (User, error) {
	if r == nil || r.database == nil {
		return User{}, fmt.Errorf("auth database is required")
	}
	row, err := r.queries.UpdateUserPassword(ctx, authsqlc.UpdateUserPasswordParams{
		ID:           id,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return User{}, err
	}
	return userFromDatabaseFields(row.ID, row.Username, row.Email, row.Role, row.PasswordHash), nil
}

func userFromDatabaseFields(id int64, username string, email string, role string, passwordHash string) User {
	return User{
		ID:           id,
		Username:     username,
		Email:        email,
		Role:         Role(role),
		PasswordHash: passwordHash,
	}
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
