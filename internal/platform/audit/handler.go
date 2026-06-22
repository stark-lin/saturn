// This file exposes the superuser-only read endpoint for audit logs.
package audit

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/httpx"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

type EventLister interface {
	List(ctx context.Context, actor auth.Principal, query Query) ([]Event, error)
}

type Handler struct {
	service EventLister
}

func NewHandler(service EventLister) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	query, err := bindQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid audit log query")
		return
	}
	logs, err := h.service.List(r.Context(), actor, query)
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
	case errors.Is(err, auth.ErrForbidden):
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "Superuser access is required")
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "audit_logs_unavailable", "Audit logs are unavailable")
	default:
		httpx.WriteJSON(w, http.StatusOK, struct {
			AuditLogs []Event `json:"audit_logs"`
			Limit     int     `json:"limit"`
			Offset    int     `json:"offset"`
		}{AuditLogs: logs, Limit: query.Limit, Offset: query.Offset})
	}
}

func bindQuery(values url.Values) (Query, error) {
	for key := range values {
		switch key {
		case "target_ref_code", "actor_user_id", "action", "result", "limit", "offset":
		default:
			return Query{}, errors.New("unsupported query parameter")
		}
	}
	query := Query{Limit: DefaultLimit}
	if value := strings.TrimSpace(values.Get("target_ref_code")); value != "" {
		query.TargetRefCode = ref.NormalizeCode(value)
		if !ref.ValidCode(query.TargetRefCode) {
			return Query{}, ErrInvalidEvent
		}
	}
	if value := strings.TrimSpace(values.Get("actor_user_id")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			return Query{}, ErrInvalidEvent
		}
		query.ActorUserID = parsed
	}
	if value := strings.TrimSpace(values.Get("action")); value != "" {
		query.Action = Action(strings.ToUpper(value))
		if !validQueryAction(query.Action) {
			return Query{}, ErrInvalidEvent
		}
	}
	if value := strings.TrimSpace(values.Get("result")); value != "" {
		query.Result = Result(strings.ToUpper(value))
		if !validQueryResult(query.Result) {
			return Query{}, ErrInvalidEvent
		}
	}
	var err error
	if value := strings.TrimSpace(values.Get("limit")); value != "" {
		query.Limit, err = strconv.Atoi(value)
		if err != nil || query.Limit < 1 || query.Limit > MaxLimit {
			return Query{}, ErrInvalidEvent
		}
	}
	if value := strings.TrimSpace(values.Get("offset")); value != "" {
		query.Offset, err = strconv.Atoi(value)
		if err != nil || query.Offset < 0 {
			return Query{}, ErrInvalidEvent
		}
	}
	return query, nil
}

func validQueryAction(action Action) bool {
	switch action {
	case ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionExport, ActionLogin, ActionLogout:
		return true
	default:
		return false
	}
}

func validQueryResult(result Result) bool {
	switch result {
	case ResultSuccess, ResultFailed, ResultDenied:
		return true
	default:
		return false
	}
}
