// This file defines Accounting HTTP request and response payloads.
package accounting

import "time"

type CreateAccountRequest struct {
	Name                string      `json:"name"`
	Type                AccountType `json:"type"`
	Currency            string      `json:"currency"`
	OpeningBalanceCents int64       `json:"opening_balance_cents"`
	Tags                []string    `json:"tags"`
}

type CreateTransactionRequest struct {
	AccountRefCode string          `json:"account_ref_code"`
	OccurredOn     string          `json:"occurred_on"`
	Kind           TransactionKind `json:"kind"`
	AmountCents    int64           `json:"amount_cents"`
	Title          string          `json:"title"`
	Note           string          `json:"note"`
	Tags           []string        `json:"tags"`
}

type AccountDetail struct {
	RefCode             string      `json:"ref_code"`
	Name                string      `json:"name"`
	Type                AccountType `json:"type"`
	Currency            string      `json:"currency"`
	OpeningBalanceCents int64       `json:"opening_balance_cents"`
	BalanceCents        int64       `json:"balance_cents"`
	Tags                []string    `json:"tags"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

type TransactionDetail struct {
	RefCode        string            `json:"ref_code"`
	AccountRefCode string            `json:"account_ref_code"`
	OccurredOn     string            `json:"occurred_on"`
	Kind           TransactionKind   `json:"kind"`
	AmountCents    int64             `json:"amount_cents"`
	Title          string            `json:"title"`
	Note           string            `json:"note"`
	Status         TransactionStatus `json:"status"`
	Tags           []string          `json:"tags"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

type AccountResponse struct {
	Account AccountDetail `json:"account"`
}

type AccountsResponse struct {
	Accounts   []AccountDetail `json:"accounts"`
	Pagination Pagination      `json:"pagination"`
}

type TransactionResponse struct {
	Transaction TransactionDetail `json:"transaction"`
}

type TransactionsResponse struct {
	Transactions []TransactionDetail `json:"transactions"`
	Pagination   Pagination          `json:"pagination"`
}

func accountResponse(account Account) AccountResponse {
	return AccountResponse{Account: accountDetail(account)}
}

func accountsResponse(page AccountPage) AccountsResponse {
	accounts := make([]AccountDetail, 0, len(page.Accounts))
	for _, account := range page.Accounts {
		accounts = append(accounts, accountDetail(account))
	}
	return AccountsResponse{
		Accounts: accounts,
		Pagination: Pagination{
			Limit: page.Limit, Offset: page.Offset, HasMore: page.HasMore,
		},
	}
}

func accountDetail(account Account) AccountDetail {
	tags := account.Tags
	if tags == nil {
		tags = []string{}
	}
	return AccountDetail{
		RefCode: account.RefCode, Name: account.Name, Type: account.Type, Currency: account.Currency,
		OpeningBalanceCents: account.OpeningBalanceCents, BalanceCents: account.BalanceCents, Tags: tags,
		CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt,
	}
}

func transactionResponse(transaction Transaction) TransactionResponse {
	return TransactionResponse{Transaction: transactionDetail(transaction)}
}

func transactionsResponse(page TransactionPage) TransactionsResponse {
	transactions := make([]TransactionDetail, 0, len(page.Transactions))
	for _, transaction := range page.Transactions {
		transactions = append(transactions, transactionDetail(transaction))
	}
	return TransactionsResponse{
		Transactions: transactions,
		Pagination: Pagination{
			Limit: page.Limit, Offset: page.Offset, HasMore: page.HasMore,
		},
	}
}

func transactionDetail(transaction Transaction) TransactionDetail {
	tags := transaction.Tags
	if tags == nil {
		tags = []string{}
	}
	return TransactionDetail{
		RefCode: transaction.RefCode, AccountRefCode: transaction.AccountRefCode,
		OccurredOn: transaction.OccurredOn.Format(time.DateOnly), Kind: transaction.Kind,
		AmountCents: transaction.AmountCents, Title: transaction.Title, Note: transaction.Note,
		Status: transaction.Status, Tags: tags, CreatedAt: transaction.CreatedAt, UpdatedAt: transaction.UpdatedAt,
	}
}
