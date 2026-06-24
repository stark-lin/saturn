// This file tests Saturn JSON runtime configuration loading.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGeneratesConfigFromEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("SATURN_HTTP_ADDR", ":9090")
	t.Setenv("SATURN_DATABASE_DROP_TABLES", "true")
	t.Setenv("SATURN_STARTUP_READINESS_TIMEOUT_SECONDS", "12")
	t.Setenv("SATURN_STORAGE_ROOT", filepath.Join(t.TempDir(), "objects"))
	t.Setenv("SATURN_LLM_WORKER_COUNT", "3")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTP.Addr != ":9090" {
		t.Fatalf("expected env http addr, got %q", cfg.HTTP.Addr)
	}
	if !strings.HasSuffix(cfg.Storage.Root, "objects") {
		t.Fatalf("expected env storage root, got %q", cfg.Storage.Root)
	}
	if cfg.Database.URL != "postgres://saturn:saturn@localhost:5432/saturn?sslmode=disable" {
		t.Fatalf("expected default database url, got %q", cfg.Database.URL)
	}
	if !cfg.Database.DropTables {
		t.Fatal("expected env database drop tables to be true")
	}
	if cfg.Startup.ReadinessTimeoutSeconds != 12 {
		t.Fatalf("expected env startup readiness timeout, got %d", cfg.Startup.ReadinessTimeoutSeconds)
	}
	if cfg.Redis.Addr != "127.0.0.1:6379" {
		t.Fatalf("expected default redis addr, got %q", cfg.Redis.Addr)
	}
	if cfg.Auth.JWTSecret == "" || cfg.Auth.TokenTTLMinutes != 480 {
		t.Fatalf("expected default auth config, got %#v", cfg.Auth)
	}
	if cfg.LLM.WorkerCount != 3 {
		t.Fatalf("expected env llm worker count, got %d", cfg.LLM.WorkerCount)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat generated config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected generated config mode 0600, got %v", info.Mode().Perm())
	}
}

func TestLoadExistingConfigIgnoresEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
  "http": { "addr": ":7070" },
  "web": { "root": "web/custom" },
  "startup": { "readiness_timeout_seconds": 17 },
  "database": { "url": "postgres://file", "drop_tables": true },
  "redis": { "addr": "file-redis:6379" },
  "auth": {
    "jwt_secret": "file-secret-for-auth",
    "token_ttl_minutes": 120
  },
  "storage": { "root": "file-objects" },
  "logging": { "level": "debug" }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("SATURN_HTTP_ADDR", ":9090")
	t.Setenv("SATURN_STORAGE_ROOT", "env-objects")
	t.Setenv("SATURN_STARTUP_READINESS_TIMEOUT_SECONDS", "12")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTP.Addr != ":7070" {
		t.Fatalf("expected file http addr, got %q", cfg.HTTP.Addr)
	}
	if cfg.Storage.Root != "file-objects" {
		t.Fatalf("expected file storage root, got %q", cfg.Storage.Root)
	}
	if !cfg.Database.DropTables {
		t.Fatal("expected file database drop tables to be true")
	}
	if cfg.Startup.ReadinessTimeoutSeconds != 17 {
		t.Fatalf("expected file startup readiness timeout, got %d", cfg.Startup.ReadinessTimeoutSeconds)
	}
	if cfg.Auth.JWTSecret != "file-secret-for-auth" || cfg.Auth.TokenTTLMinutes != 120 {
		t.Fatalf("expected file auth config, got %#v", cfg.Auth)
	}
}

func TestLoadExistingConfigDefaultsStartupReadinessTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
  "http": { "addr": ":7070" },
  "web": { "root": "web/custom" },
  "database": { "url": "postgres://file", "drop_tables": true },
  "redis": { "addr": "file-redis:6379" },
  "auth": {
    "jwt_secret": "file-secret-for-auth",
    "token_ttl_minutes": 120
  },
  "storage": { "root": "file-objects" },
  "logging": { "level": "debug" }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Startup.ReadinessTimeoutSeconds != defaultReadinessTimeoutSeconds {
		t.Fatalf("startup readiness timeout = %d, want %d", cfg.Startup.ReadinessTimeoutSeconds, defaultReadinessTimeoutSeconds)
	}
}

func TestLoadFailsFastOnInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"http":`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid json error")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestLoadFailsFastOnUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"unexpected": true}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestLoadFailsOnMissingRequiredField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected missing required fields error")
	}
	for _, field := range []string{"http.addr", "database.url", "redis.addr", "auth.jwt_secret", "auth.token_ttl_minutes", "storage.root"} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("expected missing field %s in error %q", field, err.Error())
		}
	}
}

func TestLoadFailsOnInvalidTrustedProxyCIDR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
  "http": { "addr": ":7070", "trusted_proxy_cidrs": ["not-a-network"] },
  "database": { "url": "postgres://file" },
  "redis": { "addr": "file-redis:6379" },
  "auth": { "jwt_secret": "file-secret-for-auth", "token_ttl_minutes": 120 },
  "storage": { "root": "objects" }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "trusted_proxy_cidrs") {
		t.Fatalf("trusted proxy CIDR error = %v, want configuration failure", err)
	}
}

func TestLoadFailsOnInvalidLLMWorkerCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
  "http": { "addr": ":7070", "trusted_proxy_cidrs": [] },
  "web": { "root": "web/src" },
  "database": { "url": "postgres://file" },
  "redis": { "addr": "file-redis:6379" },
  "auth": { "jwt_secret": "file-secret-for-auth", "token_ttl_minutes": 120 },
  "storage": { "root": "objects" },
  "llm": {
    "enabled": true,
    "api_key": "test-key",
    "endpoint": "https://example.invalid/v1/chat/completions",
    "model": "test-model",
    "rate_limit_per_minute": 60,
    "max_tokens": 1024,
    "worker_count": 0,
    "timeout_seconds": 60
  },
  "logging": { "level": "info" }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "llm.worker_count") {
		t.Fatalf("llm worker count error = %v, want configuration failure", err)
	}
}

func TestLoadFailsOnInvalidStartupReadinessTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := `{
  "http": { "addr": ":7070", "trusted_proxy_cidrs": [] },
  "web": { "root": "web/src" },
  "startup": { "readiness_timeout_seconds": -1 },
  "database": { "url": "postgres://file" },
  "redis": { "addr": "file-redis:6379" },
  "auth": { "jwt_secret": "file-secret-for-auth", "token_ttl_minutes": 120 },
  "storage": { "root": "objects" },
  "logging": { "level": "info" }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "startup.readiness_timeout_seconds") {
		t.Fatalf("startup readiness timeout error = %v, want configuration failure", err)
	}
}
