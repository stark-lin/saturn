// This file tests OpenAI-style provider client boundaries without external network calls.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stark-lin/go-proj/internal/platform/config"
)

func TestNewOpenAIStyleClientAppliesDefaultsAndTrimsConfig(t *testing.T) {
	client := NewOpenAIStyleClient(config.LLMConfig{
		Endpoint:           " https://example.invalid/chat ",
		APIKey:             " test-key ",
		TimeoutSeconds:     0,
		RateLimitPerMinute: 60,
	})
	if client.endpoint != "https://example.invalid/chat" || client.apiKey != "test-key" {
		t.Fatalf("client config = %#v", client)
	}
	if client.httpClient.Timeout != time.Minute {
		t.Fatalf("timeout = %v, want 1m", client.httpClient.Timeout)
	}
	if client.limiter == nil {
		t.Fatal("expected rate limiter")
	}
}

func TestOpenAIStyleClientCompleteSendsAuthorizedJSONAndParsesMessageContent(t *testing.T) {
	var gotAuth string
	var gotContentType string
	var gotBody json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"answer"}}]}`))
	}))
	defer server.Close()

	client := &OpenAIStyleClient{endpoint: server.URL, apiKey: "secret", httpClient: server.Client()}
	result, err := client.Complete(context.Background(), json.RawMessage(`{"model":"test-model"}`))
	if err != nil {
		t.Fatalf("Complete error = %v", err)
	}
	if result.Content != "answer" || string(result.RawJSON) == "" {
		t.Fatalf("result = %#v", result)
	}
	if gotAuth != "Bearer secret" || gotContentType != "application/json" || string(gotBody) != `{"model":"test-model"}` {
		t.Fatalf("provider request auth=%q content-type=%q body=%s", gotAuth, gotContentType, gotBody)
	}
}

func TestOpenAIStyleClientCompleteMapsProviderFailures(t *testing.T) {
	if _, err := (*OpenAIStyleClient)(nil).Complete(context.Background(), json.RawMessage(`{}`)); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("nil client error = %v, want unavailable", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()
	client := &OpenAIStyleClient{endpoint: server.URL, apiKey: "secret", httpClient: server.Client()}
	if _, err := client.Complete(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected provider status error")
	}
}

func TestOpenAIMessageContentSupportsChatAndLegacyText(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "chat message content", body: `{"choices":[{"message":{"content":"chat answer"}}]}`, want: "chat answer"},
		{name: "legacy text content", body: `{"choices":[{"text":"legacy answer"}]}`, want: "legacy answer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := openAIMessageContent([]byte(tt.body))
			if err != nil {
				t.Fatalf("openAIMessageContent error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("content = %q, want %q", got, tt.want)
			}
		})
	}

	for _, body := range []string{`{`, `{"choices":[]}`, `{"choices":[{"message":{"content":""}}]}`} {
		if _, err := openAIMessageContent([]byte(body)); err == nil {
			t.Fatalf("expected error for body %s", body)
		}
	}
}

func TestMinuteRateLimiterHonorsContextCancellation(t *testing.T) {
	limiter := newMinuteRateLimiter(1)
	if limiter == nil {
		t.Fatal("expected limiter")
	}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("first wait error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wait error = %v, want context canceled", err)
	}
	if newMinuteRateLimiter(0) != nil {
		t.Fatal("zero rate limit should disable limiter")
	}
}
