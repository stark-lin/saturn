// This file defines the Accounting HTTP handler dependencies.
package accounting

import (
	"context"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

type LedgerService interface {
	ListAccounts(ctx context.Context, actor auth.Principal, query AccountQuery) (AccountPage, error)
	CreateAccount(ctx context.Context, actor auth.Principal, input CreateAccountInput) (Account, error)
	GetAccount(ctx context.Context, actor auth.Principal, refCode string) (Account, error)
	DeleteAccount(ctx context.Context, actor auth.Principal, refCode string) error
	ListTransactions(ctx context.Context, actor auth.Principal, query TransactionQuery) (TransactionPage, error)
	CreateTransaction(ctx context.Context, actor auth.Principal, input CreateTransactionInput) (Transaction, error)
	GetTransaction(ctx context.Context, actor auth.Principal, refCode string) (Transaction, error)
	VoidTransaction(ctx context.Context, actor auth.Principal, refCode string) (Transaction, error)
}

type Handler struct {
	service LedgerService
}

func NewHandler(service LedgerService) *Handler {
	return &Handler{service: service}
}
