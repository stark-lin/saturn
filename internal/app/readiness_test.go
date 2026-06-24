// This file tests bounded startup dependency readiness waits.
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	platformdb "github.com/stark-lin/saturn/internal/platform/db"
	platformredis "github.com/stark-lin/saturn/internal/platform/redis"
)

func TestWaitForStartupDependenciesRunsChecksConcurrently(t *testing.T) {
	redisStarted := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	dependencies, err := waitForStartupDependenciesWithOpeners(ctx, startupDependencyOpeners{
		openDatabase: func(ctx context.Context) (*platformdb.Handle, error) {
			select {
			case <-redisStarted:
				return &platformdb.Handle{URL: "postgres://ready"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		openRedis: func(context.Context) (*platformredis.Client, error) {
			close(redisStarted)
			return &platformredis.Client{Addr: "redis:6379"}, nil
		},
		retryInterval: time.Millisecond,
	}, discardLogger())
	if err != nil {
		t.Fatalf("wait for dependencies: %v", err)
	}
	if dependencies.Database == nil || dependencies.Redis == nil {
		t.Fatalf("expected both dependencies, got %#v", dependencies)
	}
}

func TestWaitForStartupDependenciesRetriesUntilReady(t *testing.T) {
	var databaseAttempts atomic.Int32
	var redisAttempts atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	dependencies, err := waitForStartupDependenciesWithOpeners(ctx, startupDependencyOpeners{
		openDatabase: func(context.Context) (*platformdb.Handle, error) {
			if databaseAttempts.Add(1) < 3 {
				return nil, errors.New("database is booting")
			}
			return &platformdb.Handle{URL: "postgres://ready"}, nil
		},
		openRedis: func(context.Context) (*platformredis.Client, error) {
			if redisAttempts.Add(1) < 2 {
				return nil, errors.New("redis is booting")
			}
			return &platformredis.Client{Addr: "redis:6379"}, nil
		},
		retryInterval: time.Millisecond,
	}, discardLogger())
	if err != nil {
		t.Fatalf("wait for dependencies: %v", err)
	}
	if dependencies.Database == nil || dependencies.Redis == nil {
		t.Fatalf("expected both dependencies, got %#v", dependencies)
	}
	if databaseAttempts.Load() != 3 {
		t.Fatalf("database attempts = %d, want 3", databaseAttempts.Load())
	}
	if redisAttempts.Load() != 2 {
		t.Fatalf("redis attempts = %d, want 2", redisAttempts.Load())
	}
}

func TestWaitForStartupDependenciesCleansPartialSuccessOnFailure(t *testing.T) {
	var redisClosed atomic.Int32
	redisOpened := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := waitForStartupDependenciesWithOpeners(ctx, startupDependencyOpeners{
		openDatabase: func(context.Context) (*platformdb.Handle, error) {
			<-redisOpened
			cancel()
			return nil, errors.New("database is unavailable")
		},
		openRedis: func(context.Context) (*platformredis.Client, error) {
			close(redisOpened)
			return &platformredis.Client{Addr: "redis:6379"}, nil
		},
		closeRedis: func(*platformredis.Client) error {
			redisClosed.Add(1)
			return nil
		},
		retryInterval: time.Millisecond,
	}, discardLogger())
	if err == nil {
		t.Fatal("expected readiness error")
	}
	if !strings.Contains(err.Error(), "database is unavailable") {
		t.Fatalf("expected last database error, got %v", err)
	}
	if redisClosed.Load() != 1 {
		t.Fatalf("redis close count = %d, want 1", redisClosed.Load())
	}
}

func TestWaitForStartupDependenciesReportsLastDependencyError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err := waitForStartupDependenciesWithOpeners(ctx, startupDependencyOpeners{
		openDatabase: func(context.Context) (*platformdb.Handle, error) {
			return nil, errors.New("database is unavailable")
		},
		openRedis: func(context.Context) (*platformredis.Client, error) {
			return nil, errors.New("redis is unavailable")
		},
		retryInterval: time.Millisecond,
	}, discardLogger())
	if err == nil {
		t.Fatal("expected readiness error")
	}
	for _, want := range []string{"database readiness", "database is unavailable", "redis readiness", "redis is unavailable"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
