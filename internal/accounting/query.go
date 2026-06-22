// This file defines Accounting pagination and immutable ledger filters.
package accounting

import "time"

const (
	DefaultLimit = 25
	MaxLimit     = 100
)

type AccountQuery struct {
	Limit  int
	Offset int
}

type TransactionQuery struct {
	AccountRefCode string
	Status         TransactionStatus
	Tag            string
	From           *time.Time
	To             *time.Time
	Limit          int
	Offset         int
}

type AccountPage struct {
	Accounts []Account
	Limit    int
	Offset   int
	HasMore  bool
}

type TransactionPage struct {
	Transactions []Transaction
	Limit        int
	Offset       int
	HasMore      bool
}
