// This file registers the initial Saturn HTTP routes.
package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/httpx"
)

func (a *App) registerRoutes() {
	a.Router.HandleFunc("GET /healthz", a.health)
	a.Router.HandleFunc("POST /api/auth/login", a.AuthHTTP.Login)
	a.Router.Handle("GET /api/auth/me", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AuthHTTP.Me)))
	a.Router.Handle("PATCH /api/auth/me", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AuthHTTP.UpdateMe)))
	a.Router.Handle("PATCH /api/auth/me/password", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AuthHTTP.ChangeOwnPassword)))
	a.Router.Handle("POST /api/auth/users", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AuthHTTP.CreateUser)))
	a.Router.Handle("PATCH /api/auth/users/{id}/password", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AuthHTTP.ResetUserPassword)))
	a.Router.Handle("POST /api/auth/logout", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AuthHTTP.Logout)))
	a.Router.Handle("GET /api/events", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.Events.ServeHTTP)))
	a.Router.Handle("GET /api/platform/search", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.SearchHTTP.Metadata)))
	a.Router.Handle("POST /api/platform/object-refs/search", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.SearchHTTP.SearchObjectRefs)))
	a.Router.Handle("GET /api/platform/object-refs/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.SearchHTTP.ObjectRefMetadata)))
	a.Router.Handle("GET /api/platform/recent-objects", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.SearchHTTP.RecentObjects)))
	a.Router.Handle("GET /api/platform/audit-logs", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AuditHTTP.List)))
	a.Router.Handle("GET /api/accounting/accounts", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AccountingHTTP.ListAccounts)))
	a.Router.Handle("POST /api/accounting/accounts", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AccountingHTTP.CreateAccount)))
	a.Router.Handle("GET /api/accounting/accounts/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AccountingHTTP.GetAccount)))
	a.Router.Handle("DELETE /api/accounting/accounts/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AccountingHTTP.DeleteAccount)))
	a.Router.Handle("GET /api/accounting/transactions", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AccountingHTTP.ListTransactions)))
	a.Router.Handle("POST /api/accounting/transactions", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AccountingHTTP.CreateTransaction)))
	a.Router.Handle("GET /api/accounting/transactions/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AccountingHTTP.GetTransaction)))
	a.Router.Handle("POST /api/accounting/transactions/{ref_code}/void", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.AccountingHTTP.VoidTransaction)))
	a.Router.Handle("GET /api/calendar/view", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.CalendarView)))
	a.Router.Handle("GET /api/calendar/aggregates", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.ListEventAggregates)))
	a.Router.Handle("POST /api/calendar/aggregates", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.CreateEventAggregate)))
	a.Router.Handle("GET /api/calendar/aggregates/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.GetEventAggregate)))
	a.Router.Handle("DELETE /api/calendar/aggregates/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.DeleteEventAggregate)))
	a.Router.Handle("POST /api/calendar/aggregates/{ref_code}/events", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.CreateEvent)))
	a.Router.Handle("GET /api/calendar/events/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.GetEvent)))
	a.Router.Handle("POST /api/calendar/events/{ref_code}/finish", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.FinishEvent)))
	a.Router.Handle("POST /api/calendar/events/{ref_code}/void", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.CalendarHTTP.VoidEvent)))
	a.Router.Handle("GET /api/files/collections", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.ListCollections)))
	a.Router.Handle("POST /api/files/collections", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.CreateCollection)))
	a.Router.Handle("GET /api/files/collections/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.GetCollection)))
	a.Router.Handle("DELETE /api/files/collections/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.DeleteCollection)))
	a.Router.Handle("POST /api/files/collections/{ref_code}/files", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.CreateFile)))
	a.Router.Handle("GET /api/files", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.ListFiles)))
	a.Router.Handle("GET /api/files/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.GetFile)))
	a.Router.Handle("GET /api/files/objects/{ref_code}/download", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.DownloadFile)))
	a.Router.Handle("DELETE /api/files/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.FilesHTTP.DeleteFile)))
	a.Router.Handle("GET /api/llm/sessions", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.LLMHTTP.ListSessions)))
	a.Router.Handle("POST /api/llm/sessions", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.LLMHTTP.CreateSession)))
	a.Router.Handle("GET /api/llm/sessions/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.LLMHTTP.GetSession)))
	a.Router.Handle("DELETE /api/llm/sessions/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.LLMHTTP.DeleteSession)))
	a.Router.Handle("POST /api/llm/sessions/{ref_code}/requests", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.LLMHTTP.CreateRequest)))
	a.Router.Handle("GET /api/llm/requests/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.LLMHTTP.GetRequest)))
	a.Router.Handle("GET /api/notes", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.NotesHTTP.List)))
	a.Router.Handle("POST /api/notes", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.NotesHTTP.Create)))
	a.Router.Handle("GET /api/notes/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.NotesHTTP.Get)))
	a.Router.Handle("PATCH /api/notes/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.NotesHTTP.Update)))
	a.Router.Handle("DELETE /api/notes/{ref_code}", auth.AuthenticateBearer(a.Auth, http.HandlerFunc(a.NotesHTTP.Delete)))
	a.Router.HandleFunc("/", a.web)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"service":    "saturn",
		"started_at": a.StartedAt.Format(time.RFC3339),
	})
}

func (a *App) web(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	if r.URL.Path == "/" {
		http.ServeFile(w, r, filepath.Join(a.Config.Web.Root, "index.html"))
		return
	}

	cleanPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
	target := filepath.Join(a.Config.Web.Root, cleanPath)
	if _, err := os.Stat(target); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, target)
}
