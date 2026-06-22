// This file defines transaction boundary helpers for repository code.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type TransactionRunner interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
}

type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type NoopTransactionRunner struct{}

func (NoopTransactionRunner) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type SQLTransactionRunner struct {
	DB *sql.DB
}

type transactionContextKey struct{}

func (r SQLTransactionRunner) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if currentTransaction(ctx) != nil {
		return fn(ctx)
	}
	if r.DB == nil {
		return fmt.Errorf("database is required")
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, transactionContextKey{}, tx)
	if err := fn(txCtx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func ExecutorFromContext(ctx context.Context, database *sql.DB) Executor {
	if tx := currentTransaction(ctx); tx != nil {
		return tx
	}
	return database
}

func TransactionExecutorFromContext(ctx context.Context) (Executor, bool) {
	tx := currentTransaction(ctx)
	if tx == nil {
		return nil, false
	}
	return tx, true
}

func currentTransaction(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(transactionContextKey{}).(*sql.Tx)
	return tx
}
