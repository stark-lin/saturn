// This file opens and owns Saturn Redis connections.
package redis

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/stark-lin/saturn/internal/platform/config"

	goredis "github.com/redis/go-redis/v9"
)

const pingTimeout = 3 * time.Second

type slogLogger struct {
	logger *slog.Logger
}

type Client struct {
	Addr   string
	client *goredis.Client
}

func init() {
	ConfigureLogger(slog.Default())
}

func ConfigureLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}
	goredis.SetLogger(slogLogger{logger: logger})
}

func (l slogLogger) Printf(ctx context.Context, format string, values ...interface{}) {
	if l.logger == nil {
		return
	}
	l.logger.WarnContext(ctx, "redis client log", "message", fmt.Sprintf(format, values...))
}

func Open(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}

	client := goredis.NewClient(&goredis.Options{Addr: addr})
	handle := &Client{
		Addr:   addr,
		client: client,
	}
	if err := handle.Ping(ctx); err != nil {
		_ = handle.Close()
		return nil, err
	}
	return handle, nil
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("redis is not open")
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := c.client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("ping redis at %s: %w", c.Addr, err)
	}
	return nil
}

func (c *Client) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("redis is not open")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("redis key is required")
	}
	if err := c.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if c == nil || c.client == nil {
		return false, fmt.Errorf("redis is not open")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return false, fmt.Errorf("redis key is required")
	}
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists %q: %w", key, err)
	}
	return count > 0, nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("redis is not open")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("redis key is required")
	}
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete %q: %w", key, err)
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
