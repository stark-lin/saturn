// This file persists minimal Accounting ledgers through PostgreSQL.
package accounting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
)

type SQLRepository struct {
	database *sql.DB
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{database: database}
}

func (r *SQLRepository) ListAccounts(ctx context.Context, scope auth.Scope, query AccountQuery) (AccountPage, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return AccountPage{}, err
	}
	statement := `
SELECT a.id, a.owner_id, object_ref.id, object_ref.ref_code, object_ref.tags, a.name, a.type, a.currency,
       a.opening_balance_cents, a.balance_cents, object_ref.status, a.created_at, a.updated_at
FROM accounts AS a
JOIN object_refs AS object_ref
  ON object_ref.owner_id = a.owner_id
 AND object_ref.object_type = 'account'
 AND object_ref.object_id = a.id
ORDER BY a.updated_at DESC, object_ref.ref_code DESC
LIMIT $1 OFFSET $2`
	arguments := []any{query.Limit + 1, query.Offset}
	if !scope.All {
		statement = `
SELECT a.id, a.owner_id, object_ref.id, object_ref.ref_code, object_ref.tags, a.name, a.type, a.currency,
       a.opening_balance_cents, a.balance_cents, object_ref.status, a.created_at, a.updated_at
FROM accounts AS a
JOIN object_refs AS object_ref
  ON object_ref.owner_id = a.owner_id
 AND object_ref.object_type = 'account'
 AND object_ref.object_id = a.id
WHERE a.owner_id = $1
ORDER BY a.updated_at DESC, object_ref.ref_code DESC
LIMIT $2 OFFSET $3`
		arguments = []any{scope.OwnerID, query.Limit + 1, query.Offset}
	}
	rows, err := executor.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return AccountPage{}, err
	}
	defer rows.Close()

	accounts := make([]Account, 0, query.Limit+1)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return AccountPage{}, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return AccountPage{}, err
	}
	hasMore := len(accounts) > query.Limit
	if hasMore {
		accounts = accounts[:query.Limit]
	}
	return AccountPage{Accounts: accounts, Limit: query.Limit, Offset: query.Offset, HasMore: hasMore}, nil
}

func (r *SQLRepository) CreateAccount(ctx context.Context, ownerID int64, input CreateAccountInput) (Account, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Account{}, err
	}
	var account Account
	err = executor.QueryRowContext(ctx, `
INSERT INTO accounts (owner_id, name, type, currency, opening_balance_cents, balance_cents)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING id, owner_id, name, type, currency, opening_balance_cents, balance_cents, created_at, updated_at`,
		ownerID, input.Name, input.Type, input.Currency, input.OpeningBalanceCents).Scan(
		&account.ID, &account.OwnerID, &account.Name, &account.Type, &account.Currency,
		&account.OpeningBalanceCents, &account.BalanceCents, &account.CreatedAt, &account.UpdatedAt,
	)
	return account, err
}

func (r *SQLRepository) FindAccountByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Account, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Account{}, err
	}
	statement := accountByRefCodeSQL
	arguments := []any{refCode}
	if !scope.All {
		statement += ` AND a.owner_id = $2`
		arguments = append(arguments, scope.OwnerID)
	}
	account, err := scanAccount(executor.QueryRowContext(ctx, statement, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func (r *SQLRepository) LockAccountByRefCode(ctx context.Context, refCode string) (Account, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Account{}, err
	}
	account, err := scanAccount(executor.QueryRowContext(ctx, accountByRefCodeSQL+` FOR UPDATE OF a`, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	return account, err
}

func (r *SQLRepository) DeleteAccount(ctx context.Context, ownerID int64, accountID int64) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `DELETE FROM accounts WHERE owner_id = $1 AND id = $2`, ownerID, accountID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrAccountNotFound
	}
	return nil
}

func (r *SQLRepository) ListTransactions(ctx context.Context, scope auth.Scope, query TransactionQuery) (TransactionPage, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return TransactionPage{}, err
	}
	statement := transactionBaseSQL + `
WHERE ($1::text = '' OR account_ref.ref_code = $1)
  AND ($2::text = '' OR t.status = $2)
  AND ($3::text = '' OR transaction_ref.tags @> ARRAY[$3]::text[])
  AND ($4::date IS NULL OR t.occurred_on >= $4)
  AND ($5::date IS NULL OR t.occurred_on <= $5)
ORDER BY t.occurred_on DESC, t.id DESC
LIMIT $6 OFFSET $7`
	arguments := []any{
		query.AccountRefCode, query.Status, query.Tag, dateArgument(query.From), dateArgument(query.To),
		query.Limit + 1, query.Offset,
	}
	if !scope.All {
		statement = transactionBaseSQL + `
WHERE t.owner_id = $1
  AND ($2::text = '' OR account_ref.ref_code = $2)
  AND ($3::text = '' OR t.status = $3)
  AND ($4::text = '' OR transaction_ref.tags @> ARRAY[$4]::text[])
  AND ($5::date IS NULL OR t.occurred_on >= $5)
  AND ($6::date IS NULL OR t.occurred_on <= $6)
ORDER BY t.occurred_on DESC, t.id DESC
LIMIT $7 OFFSET $8`
		arguments = []any{
			scope.OwnerID, query.AccountRefCode, query.Status, query.Tag,
			dateArgument(query.From), dateArgument(query.To), query.Limit + 1, query.Offset,
		}
	}
	rows, err := executor.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return TransactionPage{}, err
	}
	defer rows.Close()

	transactions := make([]Transaction, 0, query.Limit+1)
	for rows.Next() {
		transaction, err := scanTransaction(rows)
		if err != nil {
			return TransactionPage{}, err
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return TransactionPage{}, err
	}
	hasMore := len(transactions) > query.Limit
	if hasMore {
		transactions = transactions[:query.Limit]
	}
	return TransactionPage{Transactions: transactions, Limit: query.Limit, Offset: query.Offset, HasMore: hasMore}, nil
}

func (r *SQLRepository) CreateTransaction(ctx context.Context, ownerID int64, accountID int64, input CreateTransactionInput) (Transaction, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Transaction{}, err
	}
	var transaction Transaction
	err = executor.QueryRowContext(ctx, `
INSERT INTO transactions (owner_id, account_id, occurred_on, kind, amount_cents, title, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, owner_id, account_id, occurred_on, kind, amount_cents, title, note, status, created_at, updated_at`,
		ownerID, accountID, input.OccurredOn, input.Kind, input.AmountCents, input.Title, input.Note).Scan(
		&transaction.ID, &transaction.OwnerID, &transaction.AccountID, &transaction.OccurredOn,
		&transaction.Kind, &transaction.AmountCents, &transaction.Title, &transaction.Note,
		&transaction.Status, &transaction.CreatedAt, &transaction.UpdatedAt,
	)
	return transaction, err
}

func (r *SQLRepository) FindTransactionByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Transaction, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Transaction{}, err
	}
	statement := transactionBaseSQL + `WHERE transaction_ref.ref_code = $1`
	arguments := []any{refCode}
	if !scope.All {
		statement += ` AND t.owner_id = $2`
		arguments = append(arguments, scope.OwnerID)
	}
	transaction, err := scanTransaction(executor.QueryRowContext(ctx, statement, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrTransactionNotFound
	}
	if err != nil {
		return Transaction{}, err
	}
	return transaction, nil
}

func (r *SQLRepository) LockTransactionAccountByRefCode(ctx context.Context, refCode string) (Transaction, Account, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Transaction{}, Account{}, err
	}
	row := executor.QueryRowContext(ctx, `
SELECT t.id, t.owner_id, transaction_ref.id, transaction_ref.ref_code, transaction_ref.tags, t.account_id, account_ref.ref_code,
       t.occurred_on, t.kind, t.amount_cents, t.title, t.note, t.status, t.created_at, t.updated_at,
       a.id, a.owner_id, account_ref.id, account_ref.ref_code, account_ref.tags, a.name, a.type, a.currency,
       a.opening_balance_cents, a.balance_cents, account_ref.status, a.created_at, a.updated_at
FROM transactions AS t
JOIN object_refs AS transaction_ref
  ON transaction_ref.owner_id = t.owner_id AND transaction_ref.object_type = 'transaction' AND transaction_ref.object_id = t.id
JOIN accounts AS a ON a.id = t.account_id AND a.owner_id = t.owner_id
JOIN object_refs AS account_ref
  ON account_ref.owner_id = a.owner_id AND account_ref.object_type = 'account' AND account_ref.object_id = a.id
WHERE transaction_ref.ref_code = $1
FOR UPDATE OF a`, refCode)
	var transaction Transaction
	var account Account
	err = row.Scan(
		&transaction.ID, &transaction.OwnerID, &transaction.ObjectRefID, &transaction.RefCode, pq.Array(&transaction.Tags), &transaction.AccountID, &transaction.AccountRefCode,
		&transaction.OccurredOn, &transaction.Kind, &transaction.AmountCents, &transaction.Title, &transaction.Note,
		&transaction.Status, &transaction.CreatedAt, &transaction.UpdatedAt,
		&account.ID, &account.OwnerID, &account.ObjectRefID, &account.RefCode, pq.Array(&account.Tags), &account.Name, &account.Type,
		&account.Currency, &account.OpeningBalanceCents, &account.BalanceCents, &account.Status, &account.CreatedAt, &account.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, Account{}, ErrTransactionNotFound
	}
	if err != nil {
		return Transaction{}, Account{}, err
	}
	transaction.Tags = nonNilTags(transaction.Tags)
	account.Tags = nonNilTags(account.Tags)
	return transaction, account, nil
}

func (r *SQLRepository) VoidTransaction(ctx context.Context, transaction Transaction) (Transaction, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Transaction{}, err
	}
	err = executor.QueryRowContext(ctx, `
UPDATE transactions
SET status = 'voided', updated_at = NOW()
WHERE owner_id = $1 AND id = $2 AND status = 'posted'
RETURNING status, updated_at`, transaction.OwnerID, transaction.ID).Scan(&transaction.Status, &transaction.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrTransactionAlreadyVoided
	}
	return transaction, err
}

func (r *SQLRepository) ListTransactionsForAccount(ctx context.Context, ownerID int64, accountID int64) ([]Transaction, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := executor.QueryContext(ctx, transactionBaseSQL+`
WHERE t.owner_id = $1
  AND t.account_id = $2
ORDER BY t.id`, ownerID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transactions := make([]Transaction, 0)
	for rows.Next() {
		transaction, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}
	return transactions, rows.Err()
}

func (r *SQLRepository) RecalculateAccountBalance(ctx context.Context, ownerID int64, accountID int64) (Account, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Account{}, err
	}
	var account Account
	err = executor.QueryRowContext(ctx, `
UPDATE accounts AS a
SET balance_cents = a.opening_balance_cents + COALESCE((
        SELECT SUM(txn.amount_cents)
        FROM transactions AS txn
        WHERE txn.owner_id = a.owner_id
          AND txn.account_id = a.id
          AND txn.status = 'posted'
    ), 0),
    updated_at = NOW()
WHERE a.owner_id = $1 AND a.id = $2
RETURNING a.id, a.owner_id, a.name, a.type, a.currency,
          a.opening_balance_cents, a.balance_cents, a.created_at, a.updated_at`,
		ownerID, accountID).Scan(
		&account.ID, &account.OwnerID, &account.Name, &account.Type, &account.Currency,
		&account.OpeningBalanceCents, &account.BalanceCents, &account.CreatedAt, &account.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	return account, err
}

func (r *SQLRepository) executor(ctx context.Context) (platformdb.Executor, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("accounting database is required")
	}
	return platformdb.ExecutorFromContext(ctx, r.database), nil
}

func dateArgument(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (Account, error) {
	var account Account
	err := row.Scan(
		&account.ID, &account.OwnerID, &account.ObjectRefID, &account.RefCode, pq.Array(&account.Tags), &account.Name, &account.Type,
		&account.Currency, &account.OpeningBalanceCents, &account.BalanceCents, &account.Status,
		&account.CreatedAt, &account.UpdatedAt,
	)
	account.Tags = nonNilTags(account.Tags)
	return account, err
}

func scanTransaction(row rowScanner) (Transaction, error) {
	var transaction Transaction
	err := row.Scan(
		&transaction.ID, &transaction.OwnerID, &transaction.ObjectRefID, &transaction.RefCode,
		pq.Array(&transaction.Tags), &transaction.AccountID, &transaction.AccountRefCode, &transaction.OccurredOn, &transaction.Kind,
		&transaction.AmountCents, &transaction.Title, &transaction.Note, &transaction.Status,
		&transaction.CreatedAt, &transaction.UpdatedAt,
	)
	transaction.Tags = nonNilTags(transaction.Tags)
	return transaction, err
}

const accountByRefCodeSQL = `
SELECT a.id, a.owner_id, object_ref.id, object_ref.ref_code, object_ref.tags, a.name, a.type, a.currency,
       a.opening_balance_cents, a.balance_cents, object_ref.status, a.created_at, a.updated_at
FROM accounts AS a
JOIN object_refs AS object_ref
  ON object_ref.owner_id = a.owner_id
 AND object_ref.object_type = 'account'
 AND object_ref.object_id = a.id
WHERE object_ref.ref_code = $1`

const transactionBaseSQL = `
SELECT t.id, t.owner_id, transaction_ref.id, transaction_ref.ref_code, transaction_ref.tags, t.account_id, account_ref.ref_code,
       t.occurred_on, t.kind, t.amount_cents, t.title, t.note, t.status, t.created_at, t.updated_at
FROM transactions AS t
JOIN object_refs AS transaction_ref
  ON transaction_ref.owner_id = t.owner_id
 AND transaction_ref.object_type = 'transaction'
 AND transaction_ref.object_id = t.id
JOIN object_refs AS account_ref
  ON account_ref.owner_id = t.owner_id
 AND account_ref.object_type = 'account'
 AND account_ref.object_id = t.account_id
`

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
