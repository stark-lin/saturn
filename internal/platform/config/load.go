// This file loads Saturn runtime configuration from JSON files.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const DefaultPath = "config.json"
const defaultReadinessTimeoutSeconds = 30

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath
	}

	content, err := os.ReadFile(path)
	if err == nil {
		return parseConfig(path, content)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := Default()
	if err := applyEnvironment(&cfg); err != nil {
		return Config{}, err
	}
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	if err := writeGenerated(path, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Default() Config {
	return Config{
		HTTP: HTTPConfig{
			Addr:              ":8080",
			TrustedProxyCIDRs: []string{},
		},
		Web: WebConfig{
			Root: "web/src",
		},
		Startup: StartupConfig{
			ReadinessTimeoutSeconds: defaultReadinessTimeoutSeconds,
		},
		Database: DatabaseConfig{
			URL:        "postgres://saturn:saturn@localhost:5432/saturn?sslmode=disable",
			DropTables: false,
		},
		Redis: RedisConfig{
			Addr: "127.0.0.1:6379",
		},
		Auth: defaultAuth(),
		Storage: StorageConfig{
			Root: "./objects",
		},
		LLM: LLMConfig{
			Enabled:            false,
			Endpoint:           "https://api.openai.com/v1/chat/completions",
			Model:              "gpt-4o-mini",
			RateLimitPerMinute: 60,
			MaxTokens:          1024,
			WorkerCount:        1,
			TimeoutSeconds:     60,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

func parseConfig(path string, content []byte) (Config, error) {
	var cfg Config
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("parse config %q: multiple JSON values are not allowed", path)
	}
	applyDefaults(&cfg)
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Startup.ReadinessTimeoutSeconds == 0 {
		cfg.Startup.ReadinessTimeoutSeconds = defaultReadinessTimeoutSeconds
	}
}

func writeGenerated(path string, cfg Config) error {
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode generated config %q: %w", path, err)
	}
	content = append(content, '\n')

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create config directory %q: %w", dir, err)
		}
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create config %q: %w", path, err)
	}

	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write config %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config %q: %w", path, err)
	}
	return nil
}

func applyEnvironment(cfg *Config) error {
	setString("SATURN_HTTP_ADDR", &cfg.HTTP.Addr)
	setString("SATURN_WEB_ROOT", &cfg.Web.Root)
	setString("SATURN_DATABASE_URL", &cfg.Database.URL)
	setString("SATURN_REDIS_ADDR", &cfg.Redis.Addr)
	setString("SATURN_AUTH_JWT_SECRET", &cfg.Auth.JWTSecret)
	setString("SATURN_STORAGE_ROOT", &cfg.Storage.Root)
	setString("SATURN_LLM_API_KEY", &cfg.LLM.APIKey)
	setString("SATURN_LLM_ENDPOINT", &cfg.LLM.Endpoint)
	setString("SATURN_LLM_MODEL", &cfg.LLM.Model)
	setString("SATURN_LOG_LEVEL", &cfg.Logging.Level)

	if value := os.Getenv("SATURN_DATABASE_DROP_TABLES"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse SATURN_DATABASE_DROP_TABLES: %w", err)
		}
		cfg.Database.DropTables = parsed
	}
	if value := os.Getenv("SATURN_STARTUP_READINESS_TIMEOUT_SECONDS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse SATURN_STARTUP_READINESS_TIMEOUT_SECONDS: %w", err)
		}
		cfg.Startup.ReadinessTimeoutSeconds = parsed
	}
	if value := os.Getenv("SATURN_LLM_ENABLED"); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse SATURN_LLM_ENABLED: %w", err)
		}
		cfg.LLM.Enabled = parsed
	}
	if value := os.Getenv("SATURN_AUTH_TOKEN_TTL_MINUTES"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse SATURN_AUTH_TOKEN_TTL_MINUTES: %w", err)
		}
		cfg.Auth.TokenTTLMinutes = parsed
	}
	if value := os.Getenv("SATURN_LLM_RATE_LIMIT_PER_MINUTE"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse SATURN_LLM_RATE_LIMIT_PER_MINUTE: %w", err)
		}
		cfg.LLM.RateLimitPerMinute = parsed
	}
	if value := os.Getenv("SATURN_LLM_MAX_TOKENS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse SATURN_LLM_MAX_TOKENS: %w", err)
		}
		cfg.LLM.MaxTokens = parsed
	}
	if value := os.Getenv("SATURN_LLM_WORKER_COUNT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse SATURN_LLM_WORKER_COUNT: %w", err)
		}
		cfg.LLM.WorkerCount = parsed
	}
	if value := os.Getenv("SATURN_LLM_TIMEOUT_SECONDS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse SATURN_LLM_TIMEOUT_SECONDS: %w", err)
		}
		cfg.LLM.TimeoutSeconds = parsed
	}
	return nil
}

func setString(key string, target *string) {
	if value := os.Getenv(key); value != "" {
		*target = value
	}
}

func validate(cfg Config) error {
	missingFields := make([]string, 0)
	if strings.TrimSpace(cfg.HTTP.Addr) == "" {
		missingFields = append(missingFields, "http.addr")
	}
	for _, cidr := range cfg.HTTP.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(cidr)); err != nil {
			return fmt.Errorf("invalid http.trusted_proxy_cidrs value %q: %w", cidr, err)
		}
	}
	if strings.TrimSpace(cfg.Database.URL) == "" {
		missingFields = append(missingFields, "database.url")
	}
	if strings.TrimSpace(cfg.Redis.Addr) == "" {
		missingFields = append(missingFields, "redis.addr")
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		missingFields = append(missingFields, "auth.jwt_secret")
	}
	if cfg.Auth.TokenTTLMinutes <= 0 {
		missingFields = append(missingFields, "auth.token_ttl_minutes")
	}
	if strings.TrimSpace(cfg.Storage.Root) == "" {
		missingFields = append(missingFields, "storage.root")
	}
	if cfg.Startup.ReadinessTimeoutSeconds < 1 {
		missingFields = append(missingFields, "startup.readiness_timeout_seconds")
	}
	if cfg.LLM.Enabled {
		if strings.TrimSpace(cfg.LLM.APIKey) == "" {
			missingFields = append(missingFields, "llm.api_key")
		}
		if strings.TrimSpace(cfg.LLM.Endpoint) == "" {
			missingFields = append(missingFields, "llm.endpoint")
		}
		if strings.TrimSpace(cfg.LLM.Model) == "" {
			missingFields = append(missingFields, "llm.model")
		}
		if cfg.LLM.RateLimitPerMinute < 1 {
			missingFields = append(missingFields, "llm.rate_limit_per_minute")
		}
		if cfg.LLM.MaxTokens < 1 {
			missingFields = append(missingFields, "llm.max_tokens")
		}
		if cfg.LLM.WorkerCount < 1 {
			missingFields = append(missingFields, "llm.worker_count")
		}
		if cfg.LLM.TimeoutSeconds < 1 {
			missingFields = append(missingFields, "llm.timeout_seconds")
		}
	}
	if len(missingFields) > 0 {
		return fmt.Errorf("missing required config fields: %s", strings.Join(missingFields, ", "))
	}
	return nil
}

func defaultAuth() AuthConfig {
	return AuthConfig{
		JWTSecret:       "development-only-change-this-jwt-secret",
		TokenTTLMinutes: 480,
	}
}
