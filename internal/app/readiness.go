// This file waits for required startup dependencies before the HTTP server starts.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stark-lin/saturn/internal/platform/config"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
	platformredis "github.com/stark-lin/saturn/internal/platform/redis"
)

const startupDependencyRetryInterval = 500 * time.Millisecond

type startupDependencies struct {
	Database *platformdb.Handle
	Redis    *platformredis.Client
}

type startupDependencyOpeners struct {
	openDatabase  func(context.Context) (*platformdb.Handle, error)
	openRedis     func(context.Context) (*platformredis.Client, error)
	closeDatabase func(*platformdb.Handle) error
	closeRedis    func(*platformredis.Client) error
	retryInterval time.Duration
}

type startupDependencyResult[T any] struct {
	name     string
	handle   T
	attempts int
	err      error
}

func waitForStartupDependencies(ctx context.Context, cfg config.Config, log *slog.Logger) (startupDependencies, error) {
	readinessTimeout := time.Duration(cfg.Startup.ReadinessTimeoutSeconds) * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()

	if log == nil {
		log = slog.Default()
	}
	log.Info("waiting for startup dependencies", "timeout", readinessTimeout.String())

	dependencies, err := waitForStartupDependenciesWithOpeners(waitCtx, startupDependencyOpeners{
		openDatabase: func(ctx context.Context) (*platformdb.Handle, error) {
			return platformdb.Open(ctx, cfg.Database)
		},
		openRedis: func(ctx context.Context) (*platformredis.Client, error) {
			return platformredis.Open(ctx, cfg.Redis)
		},
		closeDatabase: func(database *platformdb.Handle) error {
			return database.Close()
		},
		closeRedis: func(redisClient *platformredis.Client) error {
			return redisClient.Close()
		},
		retryInterval: startupDependencyRetryInterval,
	}, log)
	if err != nil {
		return startupDependencies{}, fmt.Errorf("startup dependencies not ready within %s: %w", readinessTimeout, err)
	}

	log.Info("startup dependencies ready")
	return dependencies, nil
}

func waitForStartupDependenciesWithOpeners(ctx context.Context, openers startupDependencyOpeners, log *slog.Logger) (startupDependencies, error) {
	if openers.retryInterval <= 0 {
		openers.retryInterval = startupDependencyRetryInterval
	}
	if openers.closeDatabase == nil {
		openers.closeDatabase = func(database *platformdb.Handle) error { return database.Close() }
	}
	if openers.closeRedis == nil {
		openers.closeRedis = func(redisClient *platformredis.Client) error { return redisClient.Close() }
	}
	if log == nil {
		log = slog.Default()
	}

	databaseResults := make(chan startupDependencyResult[*platformdb.Handle], 1)
	redisResults := make(chan startupDependencyResult[*platformredis.Client], 1)
	go func() {
		database, attempts, err := waitForDependency(ctx, openers.openDatabase, openers.retryInterval)
		databaseResults <- startupDependencyResult[*platformdb.Handle]{
			name:     "database",
			handle:   database,
			attempts: attempts,
			err:      err,
		}
	}()
	go func() {
		redisClient, attempts, err := waitForDependency(ctx, openers.openRedis, openers.retryInterval)
		redisResults <- startupDependencyResult[*platformredis.Client]{
			name:     "redis",
			handle:   redisClient,
			attempts: attempts,
			err:      err,
		}
	}()

	var dependencies startupDependencies
	var readinessErrors []error
	for databaseResults != nil || redisResults != nil {
		select {
		case result := <-databaseResults:
			databaseResults = nil
			if result.err != nil {
				readinessErrors = append(readinessErrors, fmt.Errorf("database readiness: %w", result.err))
				continue
			}
			dependencies.Database = result.handle
			log.Info("database dependency ready", "attempts", result.attempts)
		case result := <-redisResults:
			redisResults = nil
			if result.err != nil {
				readinessErrors = append(readinessErrors, fmt.Errorf("redis readiness: %w", result.err))
				continue
			}
			dependencies.Redis = result.handle
			log.Info("redis dependency ready", "addr", result.handle.Addr, "attempts", result.attempts)
		}
	}

	if len(readinessErrors) == 0 {
		return dependencies, nil
	}

	cleanupError := closeStartupDependencies(dependencies, openers)
	return startupDependencies{}, errors.Join(append(readinessErrors, cleanupError)...)
}

func waitForDependency[T any](ctx context.Context, open func(context.Context) (T, error), retryInterval time.Duration) (T, int, error) {
	var zero T
	var lastErr error
	attempts := 0
	for {
		if err := ctx.Err(); err != nil {
			return zero, attempts, dependencyWaitError(err, lastErr)
		}

		attempts++
		handle, err := open(ctx)
		if err == nil {
			return handle, attempts, nil
		}
		lastErr = err

		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return zero, attempts, dependencyWaitError(ctx.Err(), lastErr)
		case <-timer.C:
		}
	}
}

func dependencyWaitError(ctxErr error, lastErr error) error {
	if lastErr == nil {
		return ctxErr
	}
	return fmt.Errorf("%w; last error: %v", ctxErr, lastErr)
}

func closeStartupDependencies(dependencies startupDependencies, openers startupDependencyOpeners) error {
	var cleanupErrors []error
	if dependencies.Database != nil {
		if err := openers.closeDatabase(dependencies.Database); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("close database after readiness failure: %w", err))
		}
	}
	if dependencies.Redis != nil {
		if err := openers.closeRedis(dependencies.Redis); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("close redis after readiness failure: %w", err))
		}
	}
	return errors.Join(cleanupErrors...)
}
