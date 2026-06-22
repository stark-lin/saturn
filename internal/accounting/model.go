// This file defines minimal immutable Accounting ledger models.
package accounting

import "time"

type AccountType string

const (
	AccountTypeDefault       AccountType = "default"
	AccountTypeCash          AccountType = "cash"
	AccountTypeBank          AccountType = "bank"
	AccountTypeCreditCard    AccountType = "credit_card"
	AccountTypeDigitalWallet AccountType = "digital_wallet"
	AccountTypeStoredValue   AccountType = "stored_value"
	AccountTypeOther         AccountType = "other"
	AccountStatusActive                  = "active"
)

type TransactionKind string

const (
	TransactionKindIncome     TransactionKind = "income"
	TransactionKindExpense    TransactionKind = "expense"
	TransactionKindAdjustment TransactionKind = "adjustment"
)

type TransactionStatus string

const (
	TransactionStatusPosted TransactionStatus = "posted"
	TransactionStatusVoided TransactionStatus = "voided"
)

type Account struct {
	ID                  int64
	OwnerID             int64
	ObjectRefID         int64
	RefCode             string
	Name                string
	Type                AccountType
	Currency            string
	OpeningBalanceCents int64
	BalanceCents        int64
	Tags                []string
	Status              string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Transaction struct {
	ID             int64
	OwnerID        int64
	ObjectRefID    int64
	RefCode        string
	AccountID      int64
	AccountRefCode string
	OccurredOn     time.Time
	Kind           TransactionKind
	AmountCents    int64
	Title          string
	Note           string
	Status         TransactionStatus
	Tags           []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateAccountInput struct {
	Name                string
	Type                AccountType
	Currency            string
	OpeningBalanceCents int64
	Tags                []string
}

type CreateTransactionInput struct {
	AccountRefCode string
	OccurredOn     time.Time
	Kind           TransactionKind
	AmountCents    int64
	Title          string
	Note           string
	Tags           []string
}
