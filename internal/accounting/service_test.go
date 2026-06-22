// This file tests minimal Accounting ledger service invariants.
package accounting

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/audit"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

func TestNewModuleBuildsAccountingDependencies(t *testing.T) {
	module := NewModule(nil, nil, nil, nil)
	if module.Service == nil || module.Handler == nil {
		t.Fatal("expected accounting service and handler")
	}
}

func TestServiceCreatesAccountAndTransactionUsingAccountingRefsAndCachedBalance(t *testing.T) {
	service, repo, references, audits := newTestService()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}

	account, err := service.CreateAccount(context.Background(), actor, CreateAccountInput{
		Name: "Wallet", Type: AccountTypeCash, Currency: "aud", OpeningBalanceCents: 1000,
		Tags: []string{" cash ", "daily", "cash"},
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	repo.storeAccount(account)
	if account.RefCode != "ACC-00000001" || account.BalanceCents != 1000 || account.Currency != "AUD" {
		t.Fatalf("created account = %#v", account)
	}
	if len(account.Tags) != 2 || account.Tags[0] != "cash" || account.Tags[1] != "daily" {
		t.Fatalf("account tags = %#v", account.Tags)
	}

	transaction, err := service.CreateTransaction(context.Background(), actor, CreateTransactionInput{
		AccountRefCode: account.RefCode,
		OccurredOn:     time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC),
		Kind:           TransactionKindExpense,
		AmountCents:    -250,
		Title:          "Lunch",
		Note:           " Receipt FIL-00000008\n",
		Tags:           []string{" food ", "food", "work"},
	})
	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	if transaction.RefCode != "ACC-00000002" || transaction.AccountRefCode != account.RefCode {
		t.Fatalf("created transaction = %#v", transaction)
	}
	if transaction.Note != " Receipt FIL-00000008\n" {
		t.Fatalf("transaction note = %q, want original content", transaction.Note)
	}
	if repo.accounts[account.ID].BalanceCents != 750 || repo.lockedAccountRef != account.RefCode {
		t.Fatalf("account after transaction = %#v; locked = %q", repo.accounts[account.ID], repo.lockedAccountRef)
	}
	if len(references.registrations[0].Tags) != 2 ||
		references.registrations[0].Tags[0] != "cash" ||
		references.registrations[0].Tags[1] != "daily" ||
		len(references.registrations[1].Tags) != 2 ||
		references.registrations[1].Tags[0] != "food" ||
		references.registrations[1].Tags[1] != "work" {
		t.Fatalf("registration tags = %#v", references.registrations)
	}
	if references.registrations[0].ObjectType != ref.ObjectTypeAccount ||
		references.registrations[1].ObjectType != ref.ObjectTypeTransaction {
		t.Fatalf("reference registrations = %#v", references.registrations)
	}
	if len(audits.successes) != 2 || audits.successes[1].TargetRefCode != transaction.RefCode {
		t.Fatalf("audit successes = %#v", audits.successes)
	}
}

func TestServiceVoidsTransactionAndExcludesItFromBalance(t *testing.T) {
	service, repo, references, audits := newTestService()
	account := Account{ID: 1, OwnerID: 7, RefCode: "ACC-00000001", Name: "Wallet", OpeningBalanceCents: 1000, BalanceCents: 750}
	repo.storeAccount(account)
	repo.transactions[2] = Transaction{
		ID: 2, OwnerID: 7, RefCode: "ACC-00000002", AccountID: account.ID, AccountRefCode: account.RefCode,
		OccurredOn: time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC), Kind: TransactionKindExpense,
		AmountCents: -250, Title: "Lunch", Status: TransactionStatusPosted,
	}

	transaction, err := service.VoidTransaction(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, "ACC-00000002")
	if err != nil {
		t.Fatalf("void transaction: %v", err)
	}
	if transaction.Status != TransactionStatusVoided || repo.accounts[account.ID].BalanceCents != 1000 {
		t.Fatalf("voided transaction = %#v; account = %#v", transaction, repo.accounts[account.ID])
	}
	if references.updates[0].ObjectType != ref.ObjectTypeTransaction || references.updates[0].Status != "voided" {
		t.Fatalf("reference updates = %#v", references.updates)
	}
	if audits.successes[0].Reason != "void" {
		t.Fatalf("void audit = %#v", audits.successes[0])
	}

	if _, err := service.VoidTransaction(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, "ACC-00000002"); !errors.Is(err, ErrTransactionAlreadyVoided) {
		t.Fatalf("second void error = %v, want already voided", err)
	}
}

func TestServiceDeletesLedgerAndAllTransactionReferences(t *testing.T) {
	service, repo, references, audits := newTestService()
	account := Account{ID: 1, OwnerID: 7, RefCode: "ACC-00000001", Name: "Wallet"}
	repo.storeAccount(account)
	repo.transactions[2] = Transaction{ID: 2, OwnerID: 7, AccountID: 1, RefCode: "ACC-00000002"}
	repo.transactions[3] = Transaction{ID: 3, OwnerID: 7, AccountID: 1, RefCode: "ACC-00000003"}

	err := service.DeleteAccount(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, account.RefCode)
	if err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if _, exists := repo.accounts[account.ID]; exists || len(repo.transactions) != 0 {
		t.Fatalf("deleted ledger remained: accounts %#v transactions %#v", repo.accounts, repo.transactions)
	}
	if len(references.deletes) != 3 ||
		references.deletes[0].objectType != ref.ObjectTypeTransaction ||
		references.deletes[2].objectType != ref.ObjectTypeAccount {
		t.Fatalf("deleted references = %#v", references.deletes)
	}
	if len(audits.successes) != 3 ||
		audits.successes[0].TargetRefCode != "ACC-00000002" ||
		audits.successes[1].TargetRefCode != "ACC-00000003" ||
		audits.successes[2].TargetRefCode != account.RefCode {
		t.Fatalf("delete audit successes = %#v", audits.successes)
	}
	if audits.successes[0].Reason != deleteReasonCascadeAccountDelete ||
		audits.successes[1].Reason != deleteReasonCascadeAccountDelete ||
		audits.successes[2].Reason != "" {
		t.Fatalf("delete audit reasons = %#v", audits.successes)
	}
}

func TestServiceRejectsIncorrectSignedTransactionAmount(t *testing.T) {
	service, _, _, _ := newTestService()
	_, err := service.CreateTransaction(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, CreateTransactionInput{
		AccountRefCode: "ACC-00000001",
		OccurredOn:     time.Now(),
		Kind:           TransactionKindExpense,
		AmountCents:    50,
	})
	if !errors.Is(err, ErrInvalidTransaction) {
		t.Fatalf("invalid amount error = %v", err)
	}
}

func TestServiceRejectsInvalidCurrencyBeforeWrite(t *testing.T) {
	service, _, _, _ := newTestService()
	_, err := service.CreateAccount(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, CreateAccountInput{
		Name: "Wallet", Currency: "A$D",
	})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("invalid currency error = %v", err)
	}
}

func TestServiceListAndGetNormalizeQueriesAndRefCodes(t *testing.T) {
	service, repo, _, _ := newTestService()
	account := Account{ID: 1, OwnerID: 7, RefCode: "ACC-00000001", Name: "Wallet"}
	transaction := Transaction{
		ID: 2, OwnerID: 7, RefCode: "ACC-00000002", AccountID: account.ID, AccountRefCode: account.RefCode,
		OccurredOn: time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC), Kind: TransactionKindIncome,
		AmountCents: 1000, Status: TransactionStatusPosted,
	}
	repo.storeAccount(account)
	repo.transactions[transaction.ID] = transaction

	accounts, err := service.ListAccounts(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, AccountQuery{})
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if accounts.Limit != DefaultLimit || repo.lastAccountQuery.Limit != DefaultLimit {
		t.Fatalf("account query = page %#v repo %#v", accounts, repo.lastAccountQuery)
	}
	gotAccount, err := service.GetAccount(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, " acc-00000001 ")
	if err != nil || gotAccount.RefCode != account.RefCode {
		t.Fatalf("get account = %#v error = %v", gotAccount, err)
	}

	from := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	transactions, err := service.ListTransactions(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, TransactionQuery{
		AccountRefCode: " acc-00000001 ", Status: TransactionStatusPosted, Tag: " food ", From: &from, Limit: 4,
	})
	if err != nil {
		t.Fatalf("list transactions: %v", err)
	}
	if transactions.Limit != 4 ||
		repo.lastTransactionQuery.AccountRefCode != "ACC-00000001" ||
		repo.lastTransactionQuery.Tag != "food" {
		t.Fatalf("transaction query = page %#v repo %#v", transactions, repo.lastTransactionQuery)
	}
	gotTransaction, err := service.GetTransaction(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, " acc-00000002 ")
	if err != nil || gotTransaction.RefCode != transaction.RefCode {
		t.Fatalf("get transaction = %#v error = %v", gotTransaction, err)
	}
}

func TestServiceRejectsInvalidListQueries(t *testing.T) {
	service, _, _, _ := newTestService()
	if _, err := service.ListAccounts(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, AccountQuery{Limit: MaxLimit + 1}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("account query error = %v, want invalid query", err)
	}
	to := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, 1)
	if _, err := service.ListTransactions(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, TransactionQuery{
		Status: "archived", From: &from, To: &to,
	}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("transaction query error = %v, want invalid query", err)
	}
}

func TestTransactionProjectionTitleFallsBackToKindAndDate(t *testing.T) {
	occurredOn := time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		transaction Transaction
		want        string
	}{
		{name: "explicit title", transaction: Transaction{Title: "Lunch", OccurredOn: occurredOn}, want: "Lunch"},
		{name: "income", transaction: Transaction{Kind: TransactionKindIncome, OccurredOn: occurredOn}, want: "Income 2026-05-26"},
		{name: "expense", transaction: Transaction{Kind: TransactionKindExpense, OccurredOn: occurredOn}, want: "Expense 2026-05-26"},
		{name: "adjustment", transaction: Transaction{Kind: TransactionKindAdjustment, OccurredOn: occurredOn}, want: "Adjustment 2026-05-26"},
		{name: "unknown", transaction: Transaction{Kind: "transfer", OccurredOn: occurredOn}, want: "Transaction 2026-05-26"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transactionProjectionTitle(tt.transaction); got != tt.want {
				t.Fatalf("title = %q, want %q", got, tt.want)
			}
		})
	}
}

func newTestService() (*Service, *fakeRepository, *fakeReferences, *fakeAudits) {
	repo := &fakeRepository{accounts: make(map[int64]Account), transactions: make(map[int64]Transaction)}
	references := &fakeReferences{}
	audits := &fakeAudits{}
	return NewService(repo, nil, references, audits), repo, references, audits
}

type fakeRepository struct {
	nextID               int64
	accounts             map[int64]Account
	transactions         map[int64]Transaction
	lockedAccountRef     string
	lastAccountQuery     AccountQuery
	lastTransactionQuery TransactionQuery
}

func (r *fakeRepository) storeAccount(account Account) {
	r.accounts[account.ID] = account
}

func (r *fakeRepository) ListAccounts(_ context.Context, _ auth.Scope, query AccountQuery) (AccountPage, error) {
	r.lastAccountQuery = query
	return AccountPage{Limit: query.Limit, Offset: query.Offset}, nil
}

func (r *fakeRepository) CreateAccount(_ context.Context, ownerID int64, input CreateAccountInput) (Account, error) {
	r.nextID++
	account := Account{
		ID: r.nextID, OwnerID: ownerID, Name: input.Name, Type: input.Type, Currency: input.Currency,
		OpeningBalanceCents: input.OpeningBalanceCents, BalanceCents: input.OpeningBalanceCents,
	}
	r.accounts[account.ID] = account
	return account, nil
}

func (r *fakeRepository) FindAccountByRefCode(_ context.Context, _ auth.Scope, refCode string) (Account, error) {
	for _, account := range r.accounts {
		if account.RefCode == refCode {
			return account, nil
		}
	}
	return Account{}, ErrAccountNotFound
}

func (r *fakeRepository) LockAccountByRefCode(ctx context.Context, refCode string) (Account, error) {
	r.lockedAccountRef = refCode
	return r.FindAccountByRefCode(ctx, auth.Scope{All: true}, refCode)
}

func (r *fakeRepository) DeleteAccount(_ context.Context, ownerID int64, accountID int64) error {
	account, exists := r.accounts[accountID]
	if !exists || account.OwnerID != ownerID {
		return ErrAccountNotFound
	}
	delete(r.accounts, accountID)
	for id, transaction := range r.transactions {
		if transaction.AccountID == accountID {
			delete(r.transactions, id)
		}
	}
	return nil
}

func (r *fakeRepository) ListTransactions(_ context.Context, _ auth.Scope, query TransactionQuery) (TransactionPage, error) {
	r.lastTransactionQuery = query
	return TransactionPage{Limit: query.Limit, Offset: query.Offset}, nil
}

func (r *fakeRepository) CreateTransaction(_ context.Context, ownerID int64, accountID int64, input CreateTransactionInput) (Transaction, error) {
	r.nextID++
	transaction := Transaction{
		ID: r.nextID, OwnerID: ownerID, AccountID: accountID, AccountRefCode: input.AccountRefCode,
		OccurredOn: input.OccurredOn, Kind: input.Kind, AmountCents: input.AmountCents,
		Title: input.Title, Note: input.Note, Status: TransactionStatusPosted,
	}
	r.transactions[transaction.ID] = transaction
	return transaction, nil
}

func (r *fakeRepository) FindTransactionByRefCode(_ context.Context, _ auth.Scope, refCode string) (Transaction, error) {
	for _, transaction := range r.transactions {
		if transaction.RefCode == refCode {
			return transaction, nil
		}
	}
	return Transaction{}, ErrTransactionNotFound
}

func (r *fakeRepository) LockTransactionAccountByRefCode(ctx context.Context, refCode string) (Transaction, Account, error) {
	transaction, err := r.FindTransactionByRefCode(ctx, auth.Scope{All: true}, refCode)
	if err != nil {
		return Transaction{}, Account{}, err
	}
	account := r.accounts[transaction.AccountID]
	r.lockedAccountRef = account.RefCode
	return transaction, account, nil
}

func (r *fakeRepository) VoidTransaction(_ context.Context, transaction Transaction) (Transaction, error) {
	if transaction.Status != TransactionStatusPosted {
		return Transaction{}, ErrTransactionAlreadyVoided
	}
	transaction.Status = TransactionStatusVoided
	r.transactions[transaction.ID] = transaction
	return transaction, nil
}

func (r *fakeRepository) ListTransactionsForAccount(_ context.Context, ownerID int64, accountID int64) ([]Transaction, error) {
	transactions := make([]Transaction, 0)
	for _, transaction := range r.transactions {
		if transaction.OwnerID == ownerID && transaction.AccountID == accountID {
			transactions = append(transactions, transaction)
		}
	}
	sort.Slice(transactions, func(left int, right int) bool {
		return transactions[left].ID < transactions[right].ID
	})
	return transactions, nil
}

func (r *fakeRepository) RecalculateAccountBalance(_ context.Context, ownerID int64, accountID int64) (Account, error) {
	account, exists := r.accounts[accountID]
	if !exists || account.OwnerID != ownerID {
		return Account{}, ErrAccountNotFound
	}
	account.BalanceCents = account.OpeningBalanceCents
	for _, transaction := range r.transactions {
		if transaction.AccountID == accountID && transaction.Status == TransactionStatusPosted {
			account.BalanceCents += transaction.AmountCents
		}
	}
	r.accounts[accountID] = account
	return account, nil
}

type fakeReferences struct {
	sequence      int64
	registrations []ref.Registration
	updates       []ref.ProjectionUpdate
	deletes       []fakeReferenceDelete
}

type fakeReferenceDelete struct {
	objectType ref.ObjectType
	objectID   int64
}

func (r *fakeReferences) ClaimCode(_ context.Context, _ ref.ObjectType) (string, error) {
	r.sequence++
	return fmt.Sprintf("ACC-%08X", r.sequence), nil
}

func (r *fakeReferences) Register(_ context.Context, registration ref.Registration) (ref.ObjectRef, error) {
	r.registrations = append(r.registrations, registration)
	return ref.ObjectRef{ID: int64(len(r.registrations)), RefCode: registration.RefCode, Status: registration.Status}, nil
}

func (r *fakeReferences) UpdateProjection(_ context.Context, update ref.ProjectionUpdate) (ref.ObjectRef, error) {
	r.updates = append(r.updates, update)
	return ref.ObjectRef{Status: update.Status}, nil
}

func (r *fakeReferences) Delete(_ context.Context, _ int64, objectType ref.ObjectType, objectID int64) error {
	r.deletes = append(r.deletes, fakeReferenceDelete{objectType: objectType, objectID: objectID})
	return nil
}

type fakeAudits struct {
	successes []audit.Event
	failures  []audit.Event
}

func (a *fakeAudits) Record(_ context.Context, event audit.Event) (audit.Event, error) {
	a.successes = append(a.successes, event)
	return event, nil
}

func (a *fakeAudits) RecordStandalone(_ context.Context, event audit.Event) error {
	a.failures = append(a.failures, event)
	return nil
}
