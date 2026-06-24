// This file enforces minimal immutable Accounting ledger rules and atomic writes.
package accounting

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/stark-lin/saturn/internal/platform/audit"
	"github.com/stark-lin/saturn/internal/platform/auth"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

var (
	ErrDependencyUnavailable = errors.New("accounting dependency is not wired")
	ErrInvalidAccount        = errors.New("invalid account")
	ErrInvalidTransaction    = errors.New("invalid transaction")
	ErrInvalidQuery          = errors.New("invalid accounting query")
)

const deleteReasonCascadeAccountDelete = "cascade_account_delete"

type ObjectReferenceService interface {
	ClaimCode(ctx context.Context, objectType ref.ObjectType) (string, error)
	Register(ctx context.Context, registration ref.Registration) (ref.ObjectRef, error)
	UpdateProjection(ctx context.Context, update ref.ProjectionUpdate) (ref.ObjectRef, error)
	Delete(ctx context.Context, ownerID int64, objectType ref.ObjectType, objectID int64) error
}

type AuditService interface {
	Record(ctx context.Context, event audit.Event) (audit.Event, error)
	RecordStandalone(ctx context.Context, event audit.Event) error
}

type Service struct {
	repo         Repository
	transactions platformdb.TransactionRunner
	references   ObjectReferenceService
	audit        AuditService
	authorizer   *auth.Authorizer
}

func NewService(
	repo Repository,
	transactions platformdb.TransactionRunner,
	references ObjectReferenceService,
	auditService AuditService,
) *Service {
	if transactions == nil {
		transactions = platformdb.NoopTransactionRunner{}
	}
	return &Service{
		repo: repo, transactions: transactions, references: references, audit: auditService,
		authorizer: auth.NewAuthorizer(),
	}
}

func (s *Service) ListAccounts(ctx context.Context, actor auth.Principal, query AccountQuery) (AccountPage, error) {
	if actor.IsZero() {
		return AccountPage{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return AccountPage{}, ErrRepositoryUnavailable
	}
	query, err := normalizeAccountQuery(query)
	if err != nil {
		return AccountPage{}, err
	}
	return s.repo.ListAccounts(ctx, auth.ScopeForPrincipal(actor), query)
}

func (s *Service) CreateAccount(ctx context.Context, actor auth.Principal, input CreateAccountInput) (Account, error) {
	if actor.IsZero() {
		return Account{}, auth.ErrUnauthenticated
	}
	input, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}
	if err := s.requireWriteDependencies(); err != nil {
		return Account{}, err
	}
	refCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeAccount)
	if err != nil {
		return Account{}, err
	}
	var created Account
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		account, err := s.repo.CreateAccount(txCtx, actor.ID, input)
		if err != nil {
			return err
		}
		object, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: actor.ID, RefCode: refCode, ObjectType: ref.ObjectTypeAccount,
			ObjectID: account.ID, Title: account.Name, Tags: input.Tags, Status: AccountStatusActive,
		})
		if err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
			TargetRefCode: object.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		account.ObjectRefID = object.ID
		account.RefCode = object.RefCode
		account.Status = object.Status
		account.Tags = input.Tags
		created = account
		return nil
	})
	if err != nil {
		return Account{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, refCode, err)
	}
	return created, nil
}

func (s *Service) GetAccount(ctx context.Context, actor auth.Principal, refCode string) (Account, error) {
	if actor.IsZero() {
		return Account{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return Account{}, ErrRepositoryUnavailable
	}
	return s.repo.FindAccountByRefCode(ctx, auth.ScopeForPrincipal(actor), ref.NormalizeCode(refCode))
}

func (s *Service) DeleteAccount(ctx context.Context, actor auth.Principal, refCode string) error {
	if actor.IsZero() {
		return auth.ErrUnauthenticated
	}
	if err := s.requireWriteDependencies(); err != nil {
		return err
	}
	refCode = ref.NormalizeCode(refCode)
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		account, err := s.repo.LockAccountByRefCode(txCtx, refCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionDelete, "account", account.ID, account.OwnerID); err != nil {
			return err
		}
		transactions, err := s.repo.ListTransactionsForAccount(txCtx, account.OwnerID, account.ID)
		if err != nil {
			return err
		}
		for _, transaction := range transactions {
			if _, err := s.audit.Record(txCtx, audit.Event{
				ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
				TargetRefCode: transaction.RefCode, Result: audit.ResultSuccess, Reason: deleteReasonCascadeAccountDelete,
			}); err != nil {
				return err
			}
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
			TargetRefCode: account.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		for _, transaction := range transactions {
			if err := s.references.Delete(txCtx, account.OwnerID, ref.ObjectTypeTransaction, transaction.ID); err != nil {
				return err
			}
		}
		if err := s.references.Delete(txCtx, account.OwnerID, ref.ObjectTypeAccount, account.ID); err != nil {
			return err
		}
		return s.repo.DeleteAccount(txCtx, account.OwnerID, account.ID)
	})
	if err != nil {
		return s.recordWriteFailure(ctx, actor, audit.ActionDelete, refCode, err)
	}
	return nil
}

func (s *Service) ListTransactions(ctx context.Context, actor auth.Principal, query TransactionQuery) (TransactionPage, error) {
	if actor.IsZero() {
		return TransactionPage{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return TransactionPage{}, ErrRepositoryUnavailable
	}
	query, err := normalizeTransactionQuery(query)
	if err != nil {
		return TransactionPage{}, err
	}
	return s.repo.ListTransactions(ctx, auth.ScopeForPrincipal(actor), query)
}

func (s *Service) CreateTransaction(ctx context.Context, actor auth.Principal, input CreateTransactionInput) (Transaction, error) {
	if actor.IsZero() {
		return Transaction{}, auth.ErrUnauthenticated
	}
	input, err := normalizeTransactionInput(input)
	if err != nil {
		return Transaction{}, err
	}
	if err := s.requireWriteDependencies(); err != nil {
		return Transaction{}, err
	}
	refCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeTransaction)
	if err != nil {
		return Transaction{}, err
	}
	var created Transaction
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		account, err := s.repo.LockAccountByRefCode(txCtx, input.AccountRefCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionUpdate, "account", account.ID, account.OwnerID); err != nil {
			return err
		}
		transaction, err := s.repo.CreateTransaction(txCtx, account.OwnerID, account.ID, input)
		if err != nil {
			return err
		}
		object, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: account.OwnerID, RefCode: refCode, ObjectType: ref.ObjectTypeTransaction,
			ObjectID: transaction.ID, Title: transactionProjectionTitle(transaction), Tags: input.Tags, Status: string(TransactionStatusPosted),
		})
		if err != nil {
			return err
		}
		if err := s.recalculateAndProjectAccount(txCtx, account); err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
			TargetRefCode: object.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		transaction.ObjectRefID = object.ID
		transaction.RefCode = object.RefCode
		transaction.AccountRefCode = account.RefCode
		transaction.Tags = input.Tags
		created = transaction
		return nil
	})
	if err != nil {
		return Transaction{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, refCode, err)
	}
	return created, nil
}

func (s *Service) GetTransaction(ctx context.Context, actor auth.Principal, refCode string) (Transaction, error) {
	if actor.IsZero() {
		return Transaction{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return Transaction{}, ErrRepositoryUnavailable
	}
	return s.repo.FindTransactionByRefCode(ctx, auth.ScopeForPrincipal(actor), ref.NormalizeCode(refCode))
}

func (s *Service) VoidTransaction(ctx context.Context, actor auth.Principal, refCode string) (Transaction, error) {
	if actor.IsZero() {
		return Transaction{}, auth.ErrUnauthenticated
	}
	if err := s.requireWriteDependencies(); err != nil {
		return Transaction{}, err
	}
	refCode = ref.NormalizeCode(refCode)
	var voided Transaction
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		transaction, account, err := s.repo.LockTransactionAccountByRefCode(txCtx, refCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionUpdate, "transaction", transaction.ID, transaction.OwnerID); err != nil {
			return err
		}
		if transaction.Status != TransactionStatusPosted {
			return ErrTransactionAlreadyVoided
		}
		transaction, err = s.repo.VoidTransaction(txCtx, transaction)
		if err != nil {
			return err
		}
		if _, err := s.references.UpdateProjection(txCtx, ref.ProjectionUpdate{
			OwnerID: transaction.OwnerID, ObjectType: ref.ObjectTypeTransaction, ObjectID: transaction.ID,
			Title: transactionProjectionTitle(transaction), Tags: transaction.Tags, Status: string(TransactionStatusVoided),
		}); err != nil {
			return err
		}
		if err := s.recalculateAndProjectAccount(txCtx, account); err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionUpdate,
			TargetRefCode: transaction.RefCode, Result: audit.ResultSuccess, Reason: "void",
		}); err != nil {
			return err
		}
		voided = transaction
		return nil
	})
	if err != nil {
		return Transaction{}, s.recordWriteFailure(ctx, actor, audit.ActionUpdate, refCode, err)
	}
	return voided, nil
}

func (s *Service) recalculateAndProjectAccount(ctx context.Context, locked Account) error {
	account, err := s.repo.RecalculateAccountBalance(ctx, locked.OwnerID, locked.ID)
	if err != nil {
		return err
	}
	_, err = s.references.UpdateProjection(ctx, ref.ProjectionUpdate{
		OwnerID: account.OwnerID, ObjectType: ref.ObjectTypeAccount, ObjectID: account.ID,
		Title: account.Name, Tags: locked.Tags, Status: AccountStatusActive,
	})
	return err
}

func (s *Service) recordWriteFailure(ctx context.Context, actor auth.Principal, action audit.Action, refCode string, operationErr error) error {
	result := audit.ResultFailed
	reason := "operation_failed"
	if errors.Is(operationErr, ErrAccountNotFound) || errors.Is(operationErr, ErrTransactionNotFound) ||
		errors.Is(operationErr, auth.ErrForbidden) || errors.Is(operationErr, ref.ErrNotFound) {
		result = audit.ResultDenied
		reason = "not_found"
	}
	auditErr := s.audit.RecordStandalone(ctx, audit.Event{
		ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: action,
		TargetRefCode: refCode, Result: result, Reason: reason,
	})
	if auditErr != nil {
		return errors.Join(operationErr, auditErr)
	}
	return operationErr
}

func (s *Service) requireWriteDependencies() error {
	if s.repo == nil {
		return ErrRepositoryUnavailable
	}
	if s.references == nil || s.audit == nil {
		return ErrDependencyUnavailable
	}
	return nil
}

func (s *Service) can(actor auth.Principal, action auth.Action, resourceType string, resourceID int64, ownerID int64) error {
	return s.authorizer.Can(actor, action, auth.Resource{Type: resourceType, ID: resourceID, OwnerID: ownerID})
}

func normalizeAccountInput(input CreateAccountInput) (CreateAccountInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.Tags = normalizedTags(input.Tags)
	if input.Type == "" {
		input.Type = AccountTypeDefault
	}
	if input.Currency == "" {
		input.Currency = "AUD"
	}
	if input.Name == "" || !validAccountType(input.Type) || !validCurrency(input.Currency) {
		return CreateAccountInput{}, ErrInvalidAccount
	}
	return input, nil
}

func normalizeTransactionInput(input CreateTransactionInput) (CreateTransactionInput, error) {
	input.AccountRefCode = ref.NormalizeCode(input.AccountRefCode)
	input.Title = strings.TrimSpace(input.Title)
	input.Tags = normalizedTags(input.Tags)
	if !ref.ValidCode(input.AccountRefCode) || !ref.CodeMatchesObjectType(input.AccountRefCode, ref.ObjectTypeAccount) ||
		input.OccurredOn.IsZero() || input.AmountCents == 0 || !validTransactionKind(input.Kind) {
		return CreateTransactionInput{}, ErrInvalidTransaction
	}
	if input.Kind == TransactionKindIncome && input.AmountCents < 0 ||
		input.Kind == TransactionKindExpense && input.AmountCents > 0 {
		return CreateTransactionInput{}, ErrInvalidTransaction
	}
	return input, nil
}

func normalizeAccountQuery(query AccountQuery) (AccountQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	if query.Limit < 1 || query.Limit > MaxLimit || query.Offset < 0 {
		return AccountQuery{}, ErrInvalidQuery
	}
	return query, nil
}

func normalizeTransactionQuery(query TransactionQuery) (TransactionQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	query.AccountRefCode = ref.NormalizeCode(query.AccountRefCode)
	query.Tag = strings.TrimSpace(query.Tag)
	if query.Limit < 1 || query.Limit > MaxLimit || query.Offset < 0 ||
		(query.AccountRefCode != "" && (!ref.ValidCode(query.AccountRefCode) || !ref.CodeMatchesObjectType(query.AccountRefCode, ref.ObjectTypeAccount))) ||
		(query.Status != "" && query.Status != TransactionStatusPosted && query.Status != TransactionStatusVoided) ||
		(query.From != nil && query.To != nil && query.From.After(*query.To)) {
		return TransactionQuery{}, ErrInvalidQuery
	}
	return query, nil
}

func validAccountType(accountType AccountType) bool {
	switch accountType {
	case AccountTypeDefault, AccountTypeCash, AccountTypeBank, AccountTypeCreditCard,
		AccountTypeDigitalWallet, AccountTypeStoredValue, AccountTypeOther:
		return true
	default:
		return false
	}
}

func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func validTransactionKind(kind TransactionKind) bool {
	switch kind {
	case TransactionKindIncome, TransactionKindExpense, TransactionKindAdjustment:
		return true
	default:
		return false
	}
}

func normalizedTags(names []string) []string {
	tags := make([]string, 0, len(names))
	seen := make(map[string]struct{})
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}
	return tags
}

func transactionProjectionTitle(transaction Transaction) string {
	if transaction.Title != "" {
		return transaction.Title
	}
	var kind string
	switch transaction.Kind {
	case TransactionKindIncome:
		kind = "Income"
	case TransactionKindExpense:
		kind = "Expense"
	case TransactionKindAdjustment:
		kind = "Adjustment"
	default:
		kind = "Transaction"
	}
	return fmt.Sprintf("%s %s", kind, transaction.OccurredOn.Format("2006-01-02"))
}
