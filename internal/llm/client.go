// This file calls OpenAI-style chat completion APIs without exposing secrets.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stark-lin/saturn/internal/platform/config"
)

var ErrInvalidClientResponse = errors.New("invalid llm provider response")

type OpenAIStyleClient struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
	limiter    *minuteRateLimiter
}

func NewOpenAIStyleClient(cfg config.LLMConfig) *OpenAIStyleClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &OpenAIStyleClient{
		endpoint: strings.TrimSpace(cfg.Endpoint),
		apiKey:   strings.TrimSpace(cfg.APIKey),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		limiter: newMinuteRateLimiter(cfg.RateLimitPerMinute),
	}
}

func (c *OpenAIStyleClient) Complete(ctx context.Context, requestJSON json.RawMessage) (ClientResult, error) {
	if c == nil || c.endpoint == "" || c.apiKey == "" {
		return ClientResult{}, ErrClientUnavailable
	}
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return ClientResult{}, err
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(requestJSON))
	if err != nil {
		return ClientResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return ClientResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return ClientResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ClientResult{}, fmt.Errorf("llm provider returned HTTP %d", response.StatusCode)
	}
	content, err := openAIMessageContent(body)
	if err != nil {
		return ClientResult{}, err
	}
	return ClientResult{Content: content, RawJSON: json.RawMessage(body)}, nil
}

func openAIMessageContent(body []byte) (string, error) {
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if len(payload.Choices) == 0 {
		return "", ErrInvalidClientResponse
	}
	content := payload.Choices[0].Message.Content
	if content == "" {
		content = payload.Choices[0].Text
	}
	if content == "" {
		return "", ErrInvalidClientResponse
	}
	return content, nil
}

type minuteRateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newMinuteRateLimiter(limit int) *minuteRateLimiter {
	if limit <= 0 {
		return nil
	}
	return &minuteRateLimiter{interval: time.Minute / time.Duration(limit)}
}

func (l *minuteRateLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	waitUntil := l.next
	if waitUntil.Before(now) {
		waitUntil = now
	}
	l.next = waitUntil.Add(l.interval)
	l.mu.Unlock()

	delay := time.Until(waitUntil)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
