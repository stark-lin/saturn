// This file registers the initial Saturn HTTP routes.
package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/httpx"
)

func newHTTPRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.RedirectTrailingSlash = false
	return router
}

func (a *App) registerRoutes() {
	registerGETAndHEAD(a.Router, "/healthz", http.HandlerFunc(a.health))

	authRoutes := a.Router.Group("/api/auth")
	authRoutes.POST("/login", adaptHTTPHandler(http.HandlerFunc(a.AuthHTTP.Login)))

	authenticatedAPI := a.Router.Group("/api", authenticateBearer(a.Auth))

	authenticatedAuthRoutes := authenticatedAPI.Group("/auth")
	registerGETAndHEAD(authenticatedAuthRoutes, "/me", http.HandlerFunc(a.AuthHTTP.Me))
	authenticatedAuthRoutes.PATCH("/me", adaptHTTPHandler(http.HandlerFunc(a.AuthHTTP.UpdateMe)))
	authenticatedAuthRoutes.PATCH("/me/password", adaptHTTPHandler(http.HandlerFunc(a.AuthHTTP.ChangeOwnPassword)))
	authenticatedAuthRoutes.POST("/users", adaptHTTPHandler(http.HandlerFunc(a.AuthHTTP.CreateUser)))
	authenticatedAuthRoutes.PATCH("/users/:id/password", adaptHTTPHandler(http.HandlerFunc(a.AuthHTTP.ResetUserPassword)))
	authenticatedAuthRoutes.POST("/logout", adaptHTTPHandler(http.HandlerFunc(a.AuthHTTP.Logout)))

	registerGETAndHEAD(authenticatedAPI, "/events", a.Events)

	platformRoutes := authenticatedAPI.Group("/platform")
	registerGETAndHEAD(platformRoutes, "/search", http.HandlerFunc(a.SearchHTTP.Metadata))
	platformRoutes.POST("/object-refs/search", adaptHTTPHandler(http.HandlerFunc(a.SearchHTTP.SearchObjectRefs)))
	registerGETAndHEAD(platformRoutes, "/object-refs/:ref_code", http.HandlerFunc(a.SearchHTTP.ObjectRefMetadata))
	registerGETAndHEAD(platformRoutes, "/recent-objects", http.HandlerFunc(a.SearchHTTP.RecentObjects))
	registerGETAndHEAD(platformRoutes, "/audit-logs", http.HandlerFunc(a.AuditHTTP.List))

	accountingRoutes := authenticatedAPI.Group("/accounting")
	registerGETAndHEAD(accountingRoutes, "/accounts", http.HandlerFunc(a.AccountingHTTP.ListAccounts))
	accountingRoutes.POST("/accounts", adaptHTTPHandler(http.HandlerFunc(a.AccountingHTTP.CreateAccount)))
	registerGETAndHEAD(accountingRoutes, "/accounts/:ref_code", http.HandlerFunc(a.AccountingHTTP.GetAccount))
	accountingRoutes.DELETE("/accounts/:ref_code", adaptHTTPHandler(http.HandlerFunc(a.AccountingHTTP.DeleteAccount)))
	registerGETAndHEAD(accountingRoutes, "/transactions", http.HandlerFunc(a.AccountingHTTP.ListTransactions))
	accountingRoutes.POST("/transactions", adaptHTTPHandler(http.HandlerFunc(a.AccountingHTTP.CreateTransaction)))
	registerGETAndHEAD(accountingRoutes, "/transactions/:ref_code", http.HandlerFunc(a.AccountingHTTP.GetTransaction))
	accountingRoutes.POST("/transactions/:ref_code/void", adaptHTTPHandler(http.HandlerFunc(a.AccountingHTTP.VoidTransaction)))

	calendarRoutes := authenticatedAPI.Group("/calendar")
	registerGETAndHEAD(calendarRoutes, "/view", http.HandlerFunc(a.CalendarHTTP.CalendarView))
	registerGETAndHEAD(calendarRoutes, "/aggregates", http.HandlerFunc(a.CalendarHTTP.ListEventAggregates))
	calendarRoutes.POST("/aggregates", adaptHTTPHandler(http.HandlerFunc(a.CalendarHTTP.CreateEventAggregate)))
	calendarRoutes.POST("/aggregates/import-ics", adaptHTTPHandler(http.HandlerFunc(a.CalendarHTTP.ImportEventAggregate)))
	registerGETAndHEAD(calendarRoutes, "/aggregates/:ref_code", http.HandlerFunc(a.CalendarHTTP.GetEventAggregate))
	calendarRoutes.DELETE("/aggregates/:ref_code", adaptHTTPHandler(http.HandlerFunc(a.CalendarHTTP.DeleteEventAggregate)))
	calendarRoutes.POST("/aggregates/:ref_code/events", adaptHTTPHandler(http.HandlerFunc(a.CalendarHTTP.CreateEvent)))
	registerGETAndHEAD(calendarRoutes, "/events/:ref_code", http.HandlerFunc(a.CalendarHTTP.GetEvent))
	calendarRoutes.POST("/events/:ref_code/finish", adaptHTTPHandler(http.HandlerFunc(a.CalendarHTTP.FinishEvent)))
	calendarRoutes.POST("/events/:ref_code/void", adaptHTTPHandler(http.HandlerFunc(a.CalendarHTTP.VoidEvent)))

	filesRoutes := authenticatedAPI.Group("/files")
	registerGETAndHEAD(filesRoutes, "/collections", http.HandlerFunc(a.FilesHTTP.ListCollections))
	filesRoutes.POST("/collections", adaptHTTPHandler(http.HandlerFunc(a.FilesHTTP.CreateCollection)))
	registerGETAndHEAD(filesRoutes, "/collections/:ref_code", http.HandlerFunc(a.FilesHTTP.GetCollection))
	filesRoutes.DELETE("/collections/:ref_code", adaptHTTPHandler(http.HandlerFunc(a.FilesHTTP.DeleteCollection)))
	filesRoutes.POST("/collections/:ref_code/files", adaptHTTPHandler(http.HandlerFunc(a.FilesHTTP.CreateFile)))
	registerGETAndHEAD(filesRoutes, "", http.HandlerFunc(a.FilesHTTP.ListFiles))
	registerGETAndHEAD(filesRoutes, "/:ref_code", http.HandlerFunc(a.FilesHTTP.GetFile))
	registerGETAndHEAD(filesRoutes, "/objects/:ref_code/download", http.HandlerFunc(a.FilesHTTP.DownloadFile))
	filesRoutes.DELETE("/:ref_code", adaptHTTPHandler(http.HandlerFunc(a.FilesHTTP.DeleteFile)))

	llmRoutes := authenticatedAPI.Group("/llm")
	registerGETAndHEAD(llmRoutes, "/sessions", http.HandlerFunc(a.LLMHTTP.ListSessions))
	llmRoutes.POST("/sessions", adaptHTTPHandler(http.HandlerFunc(a.LLMHTTP.CreateSession)))
	registerGETAndHEAD(llmRoutes, "/sessions/:ref_code", http.HandlerFunc(a.LLMHTTP.GetSession))
	llmRoutes.DELETE("/sessions/:ref_code", adaptHTTPHandler(http.HandlerFunc(a.LLMHTTP.DeleteSession)))
	llmRoutes.POST("/sessions/:ref_code/requests", adaptHTTPHandler(http.HandlerFunc(a.LLMHTTP.CreateRequest)))
	registerGETAndHEAD(llmRoutes, "/requests/:ref_code", http.HandlerFunc(a.LLMHTTP.GetRequest))

	notesRoutes := authenticatedAPI.Group("/notes")
	registerGETAndHEAD(notesRoutes, "", http.HandlerFunc(a.NotesHTTP.List))
	notesRoutes.POST("", adaptHTTPHandler(http.HandlerFunc(a.NotesHTTP.Create)))
	registerGETAndHEAD(notesRoutes, "/versions/by-ref/:ref_code", http.HandlerFunc(a.NotesHTTP.GetVersion))
	registerGETAndHEAD(notesRoutes, "/:ref_code", http.HandlerFunc(a.NotesHTTP.Get))
	registerGETAndHEAD(notesRoutes, "/:ref_code/versions", http.HandlerFunc(a.NotesHTTP.ListVersions))
	notesRoutes.PATCH("/:ref_code", adaptHTTPHandler(http.HandlerFunc(a.NotesHTTP.Update)))
	notesRoutes.DELETE("/:ref_code", adaptHTTPHandler(http.HandlerFunc(a.NotesHTTP.Delete)))

	a.Router.NoRoute(adaptHTTPHandler(http.HandlerFunc(a.web)))
}

func registerGETAndHEAD(routes gin.IRoutes, path string, handler http.Handler) {
	wrapped := adaptHTTPHandler(handler)
	routes.GET(path, wrapped)
	routes.HEAD(path, wrapped)
}

func adaptHTTPHandler(handler http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, parameter := range c.Params {
			c.Request.SetPathValue(parameter.Key, parameter.Value)
		}
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

func authenticateBearer(service *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var authenticatedRequest *http.Request
		auth.AuthenticateBearer(service, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			authenticatedRequest = request
		})).ServeHTTP(c.Writer, c.Request)
		if authenticatedRequest == nil {
			c.Abort()
			return
		}

		c.Request = authenticatedRequest
		c.Next()
	}
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
