// This file tests Saturn Redis client boundaries.
package redis

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stark-lin/go-proj/internal/platform/config"
)

func TestMain(m *testing.M) {
	ConfigureLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func TestOpenRequiresAddr(t *testing.T) {
	_, err := Open(context.Background(), config.RedisConfig{})
	if err == nil {
		t.Fatal("expected redis addr error")
	}
	if !strings.Contains(err.Error(), "redis addr") {
		t.Fatalf("expected redis addr error, got %v", err)
	}
}

func TestOpenWithRedis(t *testing.T) {
	addr := os.Getenv("SATURN_TEST_REDIS_ADDR")
	if strings.TrimSpace(addr) == "" {
		t.Skip("set SATURN_TEST_REDIS_ADDR to run Redis connection test")
	}

	client, err := Open(context.Background(), config.RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close redis: %v", err)
		}
	})
}
