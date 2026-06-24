// This file tests the minimal Accounting HTTP contract.
package accounting

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stark-lin/saturn/internal/platform/auth"
)

func TestHandlerCreatesTransactionUsingReadableAccountReference(t *testing.T) {
	handler := NewHandler(&fakeLedgerService{transaction: Transaction{
		RefCode: "ACC-00000002", AccountRefCode: "ACC-00000001", Status: TransactionStatusPosted,
	}})
	request := authenticatedAccountingRequest(http.MethodPost, "/api/accounting/transactions",
		`{"account_ref_code":"ACC-00000001","occurred_on":"2026-05-26","kind":"expense","amount_cents":-1250,"tags":["food"]}`)
	response := httptest.NewRecorder()

	handler.CreateTransaction(response, request)

	if response.Code != http.StatusCreated || response.Header().Get("Location") != "/api/accounting/transactions/ACC-00000002" {
		t.Fatalf("create response = %d location %q", response.Code, response.Header().Get("Location"))
	}
	var body TransactionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Transaction.AccountRefCode != "ACC-00000001" {
		t.Fatalf("transaction response = %#v", body.Transaction)
	}
}

func TestHandlerCreatesAccountWithTags(t *testing.T) {
	service := &fakeLedgerService{account: Account{
		RefCode: "ACC-00000001", Name: "Wallet", Type: AccountTypeDefault, Currency: "AUD",
		Tags: []string{"cash", "daily"},
	}}
	handler := NewHandler(service)
	request := authenticatedAccountingRequest(http.MethodPost, "/api/accounting/accounts",
		`{"name":"Wallet","currency":"AUD","opening_balance_cents":5000,"tags":["cash","daily"]}`)
	response := httptest.NewRecorder()

	handler.CreateAccount(response, request)

	if response.Code != http.StatusCreated || response.Header().Get("Location") != "/api/accounting/accounts/ACC-00000001" {
		t.Fatalf("create response = %d location %q", response.Code, response.Header().Get("Location"))
	}
	var body AccountResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(service.createAccountInput.Tags) != 2 || body.Account.Tags[0] != "cash" || body.Account.Tags[1] != "daily" {
		t.Fatalf("account tags input = %#v response = %#v", service.createAccountInput.Tags, body.Account.Tags)
	}
}

func TestHandlerRejectsTransactionMutationFieldsAndMapsRepeatVoidToConflict(t *testing.T) {
	handler := NewHandler(&fakeLedgerService{err: ErrTransactionAlreadyVoided})
	request := authenticatedAccountingRequest(http.MethodPost, "/api/accounting/transactions/ACC-00000002/void", "")
	request.SetPathValue("ref_code", "ACC-00000002")
	response := httptest.NewRecorder()

	handler.VoidTransaction(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("void response status = %d, want %d", response.Code, http.StatusConflict)
	}

	create := authenticatedAccountingRequest(http.MethodPost, "/api/accounting/transactions",
		`{"account_ref_code":"ACC-00000001","occurred_on":"2026-05-26","kind":"expense","amount_cents":-1,"status":"voided"}`)
	rejected := httptest.NewRecorder()
	handler.CreateTransaction(rejected, create)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("server-owned status response = %d, want %d", rejected.Code, http.StatusBadRequest)
	}
}

func TestHandlerListsGetsAndDeletesAccount(t *testing.T) {
	service := &fakeLedgerService{account: Account{
		RefCode: "ACC-00000001", Name: "Wallet", Type: AccountTypeCash, Currency: "AUD", Tags: []string{"cash"},
	}}
	handler := NewHandler(service)

	listRequest := authenticatedAccountingRequest(http.MethodGet, "/api/accounting/accounts?limit=5&offset=2", "")
	listResponse := httptest.NewRecorder()
	handler.ListAccounts(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list accounts status = %d", listResponse.Code)
	}
	if service.listAccountQuery.Limit != 5 || service.listAccountQuery.Offset != 2 {
		t.Fatalf("list account query = %#v", service.listAccountQuery)
	}
	var listBody AccountsResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode accounts response: %v", err)
	}
	if len(listBody.Accounts) != 1 || listBody.Accounts[0].RefCode != "ACC-00000001" || listBody.Accounts[0].Tags[0] != "cash" {
		t.Fatalf("accounts response = %#v", listBody.Accounts)
	}

	getRequest := authenticatedAccountingRequest(http.MethodGet, "/api/accounting/accounts/ACC-00000001", "")
	getRequest.SetPathValue("ref_code", "ACC-00000001")
	getResponse := httptest.NewRecorder()
	handler.GetAccount(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get account status = %d", getResponse.Code)
	}

	deleteRequest := authenticatedAccountingRequest(http.MethodDelete, "/api/accounting/accounts/ACC-00000001", "")
	deleteRequest.SetPathValue("ref_code", "ACC-00000001")
	deleteResponse := httptest.NewRecorder()
	handler.DeleteAccount(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || service.deleteAccountRefCode != "ACC-00000001" {
		t.Fatalf("delete account response = %d ref = %q", deleteResponse.Code, service.deleteAccountRefCode)
	}
}

func TestHandlerListsAndGetsTransactionsWithQueryFilters(t *testing.T) {
	occurredOn := time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC)
	service := &fakeLedgerService{transaction: Transaction{
		RefCode: "ACC-00000002", AccountRefCode: "ACC-00000001", OccurredOn: occurredOn,
		Kind: TransactionKindExpense, AmountCents: -1250, Status: TransactionStatusPosted, Tags: []string{"food"},
	}}
	handler := NewHandler(service)

	listRequest := authenticatedAccountingRequest(http.MethodGet,
		"/api/accounting/transactions?account_ref_code=acc-00000001&status=posted&tag=food&from=2026-05-01&to=2026-05-31&limit=7&offset=3", "")
	listResponse := httptest.NewRecorder()
	handler.ListTransactions(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list transactions status = %d", listResponse.Code)
	}
	if service.listTransactionQuery.AccountRefCode != "ACC-00000001" ||
		service.listTransactionQuery.Status != TransactionStatusPosted ||
		service.listTransactionQuery.Tag != "food" ||
		service.listTransactionQuery.Limit != 7 ||
		service.listTransactionQuery.Offset != 3 ||
		service.listTransactionQuery.From == nil ||
		service.listTransactionQuery.To == nil {
		t.Fatalf("list transaction query = %#v", service.listTransactionQuery)
	}
	var listBody TransactionsResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode transactions response: %v", err)
	}
	if len(listBody.Transactions) != 1 ||
		listBody.Transactions[0].OccurredOn != "2026-05-26" ||
		listBody.Transactions[0].Tags[0] != "food" {
		t.Fatalf("transactions response = %#v", listBody.Transactions)
	}

	getRequest := authenticatedAccountingRequest(http.MethodGet, "/api/accounting/transactions/ACC-00000002", "")
	getRequest.SetPathValue("ref_code", "ACC-00000002")
	getResponse := httptest.NewRecorder()
	handler.GetTransaction(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get transaction status = %d", getResponse.Code)
	}
}

func TestHandlerRejectsInvalidAccountingQueriesAndUnauthenticatedList(t *testing.T) {
	handler := NewHandler(&fakeLedgerService{})
	invalidRequest := authenticatedAccountingRequest(http.MethodGet, "/api/accounting/transactions?from=bad", "")
	invalidResponse := httptest.NewRecorder()
	handler.ListTransactions(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid query status = %d, want %d", invalidResponse.Code, http.StatusBadRequest)
	}

	unauthenticatedRequest := httptest.NewRequest(http.MethodGet, "/api/accounting/accounts", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ListAccounts(unauthenticatedResponse, unauthenticatedRequest)
	if unauthenticatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", unauthenticatedResponse.Code, http.StatusUnauthorized)
	}
}

type fakeLedgerService struct {
	account              Account
	transaction          Transaction
	createAccountInput   CreateAccountInput
	listAccountQuery     AccountQuery
	listTransactionQuery TransactionQuery
	deleteAccountRefCode string
	err                  error
}

func (s *fakeLedgerService) ListAccounts(_ context.Context, _ auth.Principal, query AccountQuery) (AccountPage, error) {
	s.listAccountQuery = query
	return AccountPage{Accounts: []Account{s.account}, Limit: query.Limit, Offset: query.Offset}, s.err
}

func (s *fakeLedgerService) CreateAccount(_ context.Context, _ auth.Principal, input CreateAccountInput) (Account, error) {
	s.createAccountInput = input
	return s.account, s.err
}

func (s *fakeLedgerService) GetAccount(_ context.Context, _ auth.Principal, _ string) (Account, error) {
	return s.account, s.err
}

func (s *fakeLedgerService) DeleteAccount(_ context.Context, _ auth.Principal, refCode string) error {
	s.deleteAccountRefCode = refCode
	return s.err
}

func (s *fakeLedgerService) ListTransactions(_ context.Context, _ auth.Principal, query TransactionQuery) (TransactionPage, error) {
	s.listTransactionQuery = query
	return TransactionPage{Transactions: []Transaction{s.transaction}, Limit: query.Limit, Offset: query.Offset}, s.err
}

func (s *fakeLedgerService) CreateTransaction(_ context.Context, _ auth.Principal, _ CreateTransactionInput) (Transaction, error) {
	return s.transaction, s.err
}

func (s *fakeLedgerService) GetTransaction(_ context.Context, _ auth.Principal, _ string) (Transaction, error) {
	return s.transaction, s.err
}

func (s *fakeLedgerService) VoidTransaction(_ context.Context, _ auth.Principal, _ string) (Transaction, error) {
	return s.transaction, s.err
}

func authenticatedAccountingRequest(method string, target string, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	return request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
}
