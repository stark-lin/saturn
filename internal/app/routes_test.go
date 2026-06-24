// This file tests the static web route contract.
package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stark-lin/saturn/internal/accounting"
	"github.com/stark-lin/saturn/internal/calendar"
	"github.com/stark-lin/saturn/internal/files"
	"github.com/stark-lin/saturn/internal/llm"
	"github.com/stark-lin/saturn/internal/notes"
	"github.com/stark-lin/saturn/internal/platform/audit"
	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/config"
	"github.com/stark-lin/saturn/internal/platform/httpx"
	"github.com/stark-lin/saturn/internal/platform/search"
)

func TestWebResponsesDisableBrowserCache(t *testing.T) {
	root := t.TempDir()
	webFiles := map[string]string{
		"index.html":  "<h1>Current UI</h1>",
		"app/main.js": "export const current = true;",
	}

	for path, content := range webFiles {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("create web directory: %v", err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatalf("write web file: %v", err)
		}
	}

	saturn := &App{
		Config: config.Config{
			Web: config.WebConfig{
				Root: root,
			},
		},
	}

	for _, test := range []struct {
		name string
		path string
	}{
		{name: "document", path: "/"},
		{name: "javascript module", path: "/app/main.js"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			saturn.web(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("web status = %d, want %d", response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
			}
		})
	}
}

func TestRoutesRegisterWithoutPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("registerRoutes panicked: %v", r)
		}
	}()

	_ = newTestApp(t)
}

func TestHealthRoute(t *testing.T) {
	a := newTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	a.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health route status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	a := newTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	rec := httptest.NewRecorder()

	a.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown route status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}

func TestInvalidMethodReturns404Or405(t *testing.T) {
	a := newTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	a.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
		t.Errorf("invalid method status = %d; want %d or %d",
			rec.Code,
			http.StatusMethodNotAllowed,
			http.StatusNotFound,
		)
	}
}

func TestMiddlewareAddsRequestContextAndLogsCompletedRequest(t *testing.T) {
	log := &fakeLogger{}
	a := &App{
		Config: config.Config{
			HTTP: config.HTTPConfig{
				TrustedProxyCIDRs: []string{"127.0.0.0/8"},
			},
		},
		Logger: log,
	}
	handler := a.withMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		source := httpx.RequestSourceFromContext(r.Context())
		_, _ = w.Write([]byte(source.IP + "|" + source.UserAgent))
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Request-ID", "request-123")
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	request.Header.Set("User-Agent", "SaturnTest/1.0")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("middleware status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("X-Request-ID") != "request-123" {
		t.Fatalf("request id header = %q", response.Header().Get("X-Request-ID"))
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if got := string(body); got != "203.0.113.9|SaturnTest/1.0" {
		t.Fatalf("middleware context = %q", got)
	}
	if len(log.infos) != 1 || !strings.Contains(log.infos[0], "request completed") {
		t.Fatalf("info logs = %#v", log.infos)
	}
}

func TestCalendarRoutesAreRegistered(t *testing.T) {
	a := newTestApp(t)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/calendar/view?from=2026-06-01&to=2026-06-02"},
		{method: http.MethodGet, path: "/api/calendar/aggregates"},
		{method: http.MethodPost, path: "/api/calendar/aggregates"},
		{method: http.MethodGet, path: "/api/calendar/aggregates/CAL-00000001"},
		{method: http.MethodDelete, path: "/api/calendar/aggregates/CAL-00000001"},
		{method: http.MethodPost, path: "/api/calendar/aggregates/CAL-00000001/events"},
		{method: http.MethodGet, path: "/api/calendar/events/CAL-00000002"},
		{method: http.MethodPost, path: "/api/calendar/events/CAL-00000002/finish"},
		{method: http.MethodPost, path: "/api/calendar/events/CAL-00000002/void"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			rec := httptest.NewRecorder()

			a.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("calendar route status = %d; want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestLLMRoutesAreRegistered(t *testing.T) {
	a := newTestApp(t)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/llm/sessions"},
		{method: http.MethodPost, path: "/api/llm/sessions"},
		{method: http.MethodGet, path: "/api/llm/sessions/LLM-00000001"},
		{method: http.MethodDelete, path: "/api/llm/sessions/LLM-00000001"},
		{method: http.MethodPost, path: "/api/llm/sessions/LLM-00000001/requests"},
		{method: http.MethodGet, path: "/api/llm/requests/LLM-00000002"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			rec := httptest.NewRecorder()

			a.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("llm route status = %d; want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestPlatformObjectRefRoutesAreRegistered(t *testing.T) {
	a := newTestApp(t)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/platform/object-refs/NTE-00000001"},
		{method: http.MethodPost, path: "/api/platform/object-refs/search"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			rec := httptest.NewRecorder()

			a.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("platform object ref route status = %d; want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAuthAccountRoutesAreRegistered(t *testing.T) {
	a := newTestApp(t)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPatch, path: "/api/auth/me"},
		{method: http.MethodPatch, path: "/api/auth/me/password"},
		{method: http.MethodPost, path: "/api/auth/users"},
		{method: http.MethodPatch, path: "/api/auth/users/1/password"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, nil)
			rec := httptest.NewRecorder()

			a.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("auth account route status = %d; want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func newTestApp(t *testing.T) *App {
	t.Helper()

	a := &App{
		Router:         http.NewServeMux(),
		StartedAt:      time.Unix(0, 0),
		AuthHTTP:       &auth.Handler{},
		Events:         &httpx.Broker{},
		SearchHTTP:     &search.Handler{},
		AuditHTTP:      &audit.Handler{},
		AccountingHTTP: &accounting.Handler{},
		CalendarHTTP:   &calendar.Handler{},
		FilesHTTP:      &files.Handler{},
		LLMHTTP:        &llm.Handler{},
		NotesHTTP:      &notes.Handler{},
	}

	a.registerRoutes()

	return a
}

type fakeLogger struct {
	infos  []string
	errors []string
}

func (l *fakeLogger) Info(msg string, _ ...any) {
	l.infos = append(l.infos, msg)
}

func (l *fakeLogger) Error(msg string, _ ...any) {
	l.errors = append(l.errors, msg)
}
