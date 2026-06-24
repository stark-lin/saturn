// This file exposes authenticated minimal Accounting ledger API handlers.
package accounting

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/httpx"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	query, err := bindAccountQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid account query")
		return
	}
	page, err := h.service.ListAccounts(r.Context(), principal, query)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, accountsResponse(page))
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request CreateAccountRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid account request")
		return
	}
	account, err := h.service.CreateAccount(r.Context(), principal, CreateAccountInput{
		Name: request.Name, Type: request.Type, Currency: request.Currency,
		OpeningBalanceCents: request.OpeningBalanceCents, Tags: request.Tags,
	})
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/accounting/accounts/"+account.RefCode)
	httpx.WriteJSON(w, http.StatusCreated, accountResponse(account))
}

func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := accountingResourceRequest(w, r)
	if !ok {
		return
	}
	account, err := h.service.GetAccount(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, accountResponse(account))
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := accountingResourceRequest(w, r)
	if !ok {
		return
	}
	if h.writeServiceError(w, h.service.DeleteAccount(r.Context(), principal, refCode)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	query, err := bindTransactionQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid transaction query")
		return
	}
	page, err := h.service.ListTransactions(r.Context(), principal, query)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, transactionsResponse(page))
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request CreateTransactionRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid transaction request")
		return
	}
	occurredOn, err := time.Parse(time.DateOnly, strings.TrimSpace(request.OccurredOn))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid transaction request")
		return
	}
	transaction, err := h.service.CreateTransaction(r.Context(), principal, CreateTransactionInput{
		AccountRefCode: request.AccountRefCode, OccurredOn: occurredOn, Kind: request.Kind,
		AmountCents: request.AmountCents, Title: request.Title, Note: request.Note, Tags: request.Tags,
	})
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/accounting/transactions/"+transaction.RefCode)
	httpx.WriteJSON(w, http.StatusCreated, transactionResponse(transaction))
}

func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := accountingResourceRequest(w, r)
	if !ok {
		return
	}
	transaction, err := h.service.GetTransaction(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, transactionResponse(transaction))
}

func (h *Handler) VoidTransaction(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := accountingResourceRequest(w, r)
	if !ok {
		return
	}
	transaction, err := h.service.VoidTransaction(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, transactionResponse(transaction))
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
	case errors.Is(err, ErrInvalidAccount), errors.Is(err, ErrInvalidTransaction), errors.Is(err, ErrInvalidQuery):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid accounting request")
	case errors.Is(err, ErrTransactionAlreadyVoided):
		httpx.WriteError(w, http.StatusConflict, "conflict", "Transaction is already voided")
	case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrTransactionNotFound),
		errors.Is(err, auth.ErrForbidden), errors.Is(err, ref.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Accounting resource not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "accounting_unavailable", "Accounting service is unavailable")
	}
	return true
}

func authenticatedPrincipal(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return auth.Principal{}, false
	}
	return principal, true
}

func accountingResourceRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, string, bool) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return auth.Principal{}, "", false
	}
	refCode := ref.NormalizeCode(r.PathValue("ref_code"))
	if !ref.ValidCode(refCode) || !ref.CodeMatchesObjectType(refCode, ref.ObjectTypeAccount) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return auth.Principal{}, "", false
	}
	return principal, refCode, true
}

func bindAccountQuery(values url.Values) (AccountQuery, error) {
	for key := range values {
		if key != "limit" && key != "offset" {
			return AccountQuery{}, ErrInvalidQuery
		}
	}
	query := AccountQuery{Limit: DefaultLimit}
	if err := bindPagination(values, &query.Limit, &query.Offset); err != nil {
		return AccountQuery{}, err
	}
	return query, nil
}

func bindTransactionQuery(values url.Values) (TransactionQuery, error) {
	for key := range values {
		switch key {
		case "account_ref_code", "status", "tag", "from", "to", "limit", "offset":
		default:
			return TransactionQuery{}, ErrInvalidQuery
		}
	}
	query := TransactionQuery{
		AccountRefCode: strings.TrimSpace(values.Get("account_ref_code")),
		Status:         TransactionStatus(strings.TrimSpace(values.Get("status"))),
		Tag:            strings.TrimSpace(values.Get("tag")),
		Limit:          DefaultLimit,
	}
	var err error
	if value := strings.TrimSpace(values.Get("from")); value != "" {
		date, parseErr := time.Parse(time.DateOnly, value)
		if parseErr != nil {
			return TransactionQuery{}, ErrInvalidQuery
		}
		query.From = &date
	}
	if value := strings.TrimSpace(values.Get("to")); value != "" {
		date, parseErr := time.Parse(time.DateOnly, value)
		if parseErr != nil {
			return TransactionQuery{}, ErrInvalidQuery
		}
		query.To = &date
	}
	err = bindPagination(values, &query.Limit, &query.Offset)
	if err != nil {
		return TransactionQuery{}, err
	}
	return normalizeTransactionQuery(query)
}

func bindPagination(values url.Values, limit *int, offset *int) error {
	if value := values.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > MaxLimit {
			return ErrInvalidQuery
		}
		*limit = parsed
	}
	if value := values.Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return ErrInvalidQuery
		}
		*offset = parsed
	}
	return nil
}
