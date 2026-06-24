// This file loads application dependencies before the HTTP server starts.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/stark-lin/saturn/internal/platform/audit"
	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/config"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
	"github.com/stark-lin/saturn/internal/platform/httpx"
	"github.com/stark-lin/saturn/internal/platform/logger"
	platformredis "github.com/stark-lin/saturn/internal/platform/redis"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

const migrationsDir = "migrations"

type Dependencies struct {
	Config     config.Config
	Database   *platformdb.Handle
	Events     *httpx.Broker
	Logger     *slog.Logger
	Redis      *platformredis.Client
	Auth       *auth.Service
	Audits     *audit.Service
	References *ref.Service
	StartedAt  time.Time
}

func LoadDependencies(ctx context.Context, configPath string) (Dependencies, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return Dependencies{}, err
	}

	log := logger.New(os.Stdout, cfg.Logging.Level)
	database, err := platformdb.Open(ctx, cfg.Database)
	if err != nil {
		return Dependencies{}, err
	}
	if err := platformdb.BootstrapSchema(ctx, database.DB, migrationsDir, platformdb.BootstrapOptions{DropTables: cfg.Database.DropTables}); err != nil {
		_ = database.Close()
		return Dependencies{}, fmt.Errorf("bootstrap database schema: %w", err)
	}
	authRepo := auth.NewSQLRepository(database.DB)
	if err := auth.EnsureDevelopmentAdmin(ctx, authRepo); err != nil {
		_ = database.Close()
		return Dependencies{}, err
	}

	platformredis.ConfigureLogger(log)
	redisClient, err := platformredis.Open(ctx, cfg.Redis)
	if err != nil {
		_ = database.Close()
		log.Error("redis dependency unavailable", "addr", cfg.Redis.Addr, "error", err)
		return Dependencies{}, fmt.Errorf("redis dependency unavailable at %s; start Redis or update redis.addr in config.json: %w", cfg.Redis.Addr, err)
	}
	tokenManager, err := auth.NewTokenManager(cfg.Auth.JWTSecret, time.Duration(cfg.Auth.TokenTTLMinutes)*time.Minute)
	if err != nil {
		_ = redisClient.Close()
		_ = database.Close()
		return Dependencies{}, fmt.Errorf("configure authentication tokens: %w", err)
	}
	log.Info("redis dependency ready", "addr", redisClient.Addr)
	transactionRunner := platformdb.SQLTransactionRunner{DB: database.DB}
	auditService := audit.NewService(audit.NewSQLRepository(database.DB), transactionRunner)

	return Dependencies{
		Config:     cfg,
		Database:   database,
		Events:     httpx.NewBroker(),
		Logger:     log,
		Redis:      redisClient,
		Auth:       auth.NewService(authRepo, auth.NewRedisSessionStore(redisClient), tokenManager, auditService),
		Audits:     auditService,
		References: ref.NewService(ref.NewSQLRepository(database.DB)),
		StartedAt:  time.Now().UTC(),
	}, nil
}
