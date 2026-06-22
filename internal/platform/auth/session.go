// This file defines session data owned by the authentication platform package.
package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	platformredis "github.com/stark-lin/go-proj/internal/platform/redis"
)

type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (s Session) Expired(now time.Time) bool {
	return !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt)
}

type SessionStore interface {
	Save(ctx context.Context, session Session) error
	Active(ctx context.Context, sessionID string) (bool, error)
	Delete(ctx context.Context, sessionID string) error
}

type RedisSessionStore struct {
	client *platformredis.Client
}

func NewRedisSessionStore(client *platformredis.Client) *RedisSessionStore {
	return &RedisSessionStore{client: client}
}

func (s *RedisSessionStore) Save(ctx context.Context, session Session) error {
	if s == nil || s.client == nil || session.ID == "" || session.UserID <= 0 {
		return fmt.Errorf("valid session and redis client are required")
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session expiration must be in the future")
	}
	return s.client.Set(ctx, sessionKey(session.ID), strconv.FormatInt(session.UserID, 10), ttl)
}

func (s *RedisSessionStore) Active(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.client == nil || sessionID == "" {
		return false, nil
	}
	return s.client.Exists(ctx, sessionKey(sessionID))
}

func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	if s == nil || s.client == nil || sessionID == "" {
		return nil
	}
	return s.client.Delete(ctx, sessionKey(sessionID))
}

func sessionKey(sessionID string) string {
	return "auth:session:" + sessionID
}
