// This file tests the superuser-only audit log HTTP contract.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

func TestHandlerListRejectsOrdinaryUser(t *testing.T) {
	handler := NewHandler(&fakeLister{err: auth.ErrForbidden})
	request := httptest.NewRequest(http.MethodGet, "/api/platform/audit-logs", nil)
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
	response := httptest.NewRecorder()

	handler.List(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("list status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestHandlerListReturnsSuperuserAuditEnvelope(t *testing.T) {
	lister := &fakeLister{logs: []Event{{ID: 1, ActorType: ActorTypeUser, Action: ActionLogin, TargetRefCode: SystemTargetRefCode, Result: ResultSuccess}}}
	handler := NewHandler(lister)
	request := httptest.NewRequest(http.MethodGet, "/api/platform/audit-logs?limit=10&offset=2&action=login&result=success&target_ref_code=nte-00000001&actor_user_id=7", nil)
	request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 1, Role: auth.RoleSuperuser}))
	response := httptest.NewRecorder()

	handler.List(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if lister.query.Limit != 10 || lister.query.Offset != 2 ||
		lister.query.Action != ActionLogin || lister.query.Result != ResultSuccess ||
		lister.query.TargetRefCode != "NTE-00000001" || lister.query.ActorUserID != 7 {
		t.Fatalf("query = %#v", lister.query)
	}
	var body struct {
		AuditLogs []Event `json:"audit_logs"`
		Limit     int     `json:"limit"`
		Offset    int     `json:"offset"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.AuditLogs) != 1 || body.Limit != 10 || body.Offset != 2 {
		t.Fatalf("body = %#v", body)
	}
}

func TestHandlerListRejectsMissingPrincipal(t *testing.T) {
	handler := NewHandler(&fakeLister{})
	request := httptest.NewRequest(http.MethodGet, "/api/platform/audit-logs", nil)
	response := httptest.NewRecorder()

	handler.List(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestHandlerListRejectsInvalidQuery(t *testing.T) {
	tests := []string{
		"/api/platform/audit-logs?unknown=1",
		"/api/platform/audit-logs?target_ref_code=bad",
		"/api/platform/audit-logs?actor_user_id=0",
		"/api/platform/audit-logs?action=bad",
		"/api/platform/audit-logs?result=bad",
		"/api/platform/audit-logs?limit=0",
		"/api/platform/audit-logs?offset=-1",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			handler := NewHandler(&fakeLister{})
			request := httptest.NewRequest(http.MethodGet, target, nil)
			request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 1, Role: auth.RoleSuperuser}))
			response := httptest.NewRecorder()

			handler.List(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
}

func TestHandlerListMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "unauthenticated", err: auth.ErrUnauthenticated, wantStatus: http.StatusUnauthorized},
		{name: "forbidden", err: auth.ErrForbidden, wantStatus: http.StatusForbidden},
		{name: "unexpected", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(&fakeLister{err: tt.err})
			request := httptest.NewRequest(http.MethodGet, "/api/platform/audit-logs", nil)
			request = request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 1, Role: auth.RoleSuperuser}))
			response := httptest.NewRecorder()

			handler.List(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
		})
	}
}

type fakeLister struct {
	logs  []Event
	err   error
	query Query
}

func (l *fakeLister) List(_ context.Context, _ auth.Principal, query Query) ([]Event, error) {
	l.query = query
	return l.logs, l.err
}
