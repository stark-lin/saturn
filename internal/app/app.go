// This file defines the Saturn application container and HTTP router holder.
package app

import (
	"context"
	"net/http"
	"time"

	"github.com/stark-lin/go-proj/internal/accounting"
	"github.com/stark-lin/go-proj/internal/calendar"
	"github.com/stark-lin/go-proj/internal/files"
	"github.com/stark-lin/go-proj/internal/llm"
	"github.com/stark-lin/go-proj/internal/notes"
	"github.com/stark-lin/go-proj/internal/platform/audit"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/config"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
	"github.com/stark-lin/go-proj/internal/platform/httpx"
	platformredis "github.com/stark-lin/go-proj/internal/platform/redis"
	"github.com/stark-lin/go-proj/internal/platform/ref"
	"github.com/stark-lin/go-proj/internal/platform/search"
	"github.com/stark-lin/go-proj/internal/platform/storage"
)

type App struct {
	Config         config.Config
	Database       *platformdb.Handle
	Events         *httpx.Broker
	Logger         Logger
	Redis          *platformredis.Client
	Auth           *auth.Service
	AuthHTTP       *auth.Handler
	Audits         *audit.Service
	AuditHTTP      *audit.Handler
	References     *ref.Service
	AccountingHTTP *accounting.Handler
	CalendarHTTP   *calendar.Handler
	FilesHTTP      *files.Handler
	LLMHTTP        *llm.Handler
	NotesHTTP      *notes.Handler
	LLMWorker      *llm.Worker
	SearchHTTP     *search.Handler
	Router         *http.ServeMux
	Server         *http.Server
	StartedAt      time.Time
}

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

func New(_ context.Context, deps Dependencies) (*App, error) {
	router := http.NewServeMux()
	transactionRunner := platformdb.SQLTransactionRunner{DB: deps.Database.DB}
	notesModule := notes.NewModule(
		notes.NewSQLRepository(deps.Database.DB),
		transactionRunner,
		deps.References,
		deps.Audits,
	)
	accountingModule := accounting.NewModule(
		accounting.NewSQLRepository(deps.Database.DB),
		transactionRunner,
		deps.References,
		deps.Audits,
	)
	calendarModule := calendar.NewModule(
		calendar.NewSQLRepository(deps.Database.DB),
		transactionRunner,
		deps.References,
		deps.Audits,
	)
	storageService := storage.NewService(
		storage.NewLocalFSClient(deps.Config.Storage.Root),
		storage.NewSQLRepository(deps.Database.DB),
	)
	filesModule := files.NewModule(
		files.NewSQLRepository(deps.Database.DB),
		transactionRunner,
		deps.References,
		deps.Audits,
		storageService,
	)
	llmClient := llm.NewOpenAIStyleClient(deps.Config.LLM)
	llmResolver := llm.NewBusinessReferenceResolver(
		deps.References,
		accountingModule.Service,
		notesModule.Service,
		filesModule.Service,
		calendarModule.Service,
	)
	llmModule := llm.NewModule(llm.ServiceDependencies{
		Repository:   llm.NewSQLRepository(deps.Database.DB),
		Transactions: transactionRunner,
		References:   deps.References,
		Audit:        deps.Audits,
		Client:       llmClient,
		Resolver:     llmResolver,
		Config: llm.RuntimeConfig{
			Enabled:   deps.Config.LLM.Enabled,
			Model:     deps.Config.LLM.Model,
			MaxTokens: deps.Config.LLM.MaxTokens,
		},
	})
	llmWorker := llm.NewWorker(llmModule.Service, llm.WorkerConfig{
		WorkerCount:    deps.Config.LLM.WorkerCount,
		RequestTimeout: time.Duration(deps.Config.LLM.TimeoutSeconds) * time.Second,
	}, deps.Logger)
	saturn := &App{
		Config:         deps.Config,
		Database:       deps.Database,
		Events:         deps.Events,
		Logger:         deps.Logger,
		Redis:          deps.Redis,
		Auth:           deps.Auth,
		AuthHTTP:       auth.NewHandler(deps.Auth),
		Audits:         deps.Audits,
		AuditHTTP:      audit.NewHandler(deps.Audits),
		References:     deps.References,
		AccountingHTTP: accountingModule.Handler,
		CalendarHTTP:   calendarModule.Handler,
		FilesHTTP:      filesModule.Handler,
		LLMHTTP:        llmModule.Handler,
		NotesHTTP:      notesModule.Handler,
		LLMWorker:      llmWorker,
		SearchHTTP:     search.NewHandler(deps.References),
		Router:         router,
		StartedAt:      deps.StartedAt,
	}
	saturn.registerRoutes()
	saturn.Server = &http.Server{
		Addr:              deps.Config.HTTP.Addr,
		Handler:           saturn.withMiddleware(router),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return saturn, nil
}
