// This file defines minimal Accounting data access boundaries.
package accounting

import (
	"context"
	"errors"

	"github.com/stark-lin/saturn/internal/platform/auth"
)

var (
	ErrRepositoryUnavailable    = errors.New("accounting repository is not wired")
	ErrAccountNotFound          = errors.New("account not found")
	ErrTransactionNotFound      = errors.New("transaction not found")
	ErrTransactionAlreadyVoided = errors.New("transaction is already voided")
)

type Repository interface {
	ListAccounts(ctx context.Context, scope auth.Scope, query AccountQuery) (AccountPage, error)
	CreateAccount(ctx context.Context, ownerID int64, input CreateAccountInput) (Account, error)
	FindAccountByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Account, error)
	LockAccountByRefCode(ctx context.Context, refCode string) (Account, error)
	DeleteAccount(ctx context.Context, ownerID int64, accountID int64) error

	ListTransactions(ctx context.Context, scope auth.Scope, query TransactionQuery) (TransactionPage, error)
	CreateTransaction(ctx context.Context, ownerID int64, accountID int64, input CreateTransactionInput) (Transaction, error)
	FindTransactionByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Transaction, error)
	LockTransactionAccountByRefCode(ctx context.Context, refCode string) (Transaction, Account, error)
	VoidTransaction(ctx context.Context, transaction Transaction) (Transaction, error)
	ListTransactionsForAccount(ctx context.Context, ownerID int64, accountID int64) ([]Transaction, error)
	RecalculateAccountBalance(ctx context.Context, ownerID int64, accountID int64) (Account, error)
}
