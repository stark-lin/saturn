// This file defines Saturn runtime configuration structures.
package config

type Config struct {
	HTTP     HTTPConfig     `json:"http"`
	Web      WebConfig      `json:"web"`
	Database DatabaseConfig `json:"database"`
	Redis    RedisConfig    `json:"redis"`
	Auth     AuthConfig     `json:"auth"`
	Storage  StorageConfig  `json:"storage"`
	LLM      LLMConfig      `json:"llm"`
	Logging  LoggingConfig  `json:"logging"`
}

type HTTPConfig struct {
	Addr              string   `json:"addr"`
	TrustedProxyCIDRs []string `json:"trusted_proxy_cidrs"`
}

type WebConfig struct {
	Root string `json:"root"`
}

type DatabaseConfig struct {
	URL        string `json:"url"`
	DropTables bool   `json:"drop_tables"`
}

type RedisConfig struct {
	Addr string `json:"addr"`
}

type AuthConfig struct {
	JWTSecret       string `json:"jwt_secret"`
	TokenTTLMinutes int    `json:"token_ttl_minutes"`
}

type StorageConfig struct {
	Root string `json:"root"`
}

type LLMConfig struct {
	Enabled            bool   `json:"enabled"`
	APIKey             string `json:"api_key"`
	Endpoint           string `json:"endpoint"`
	Model              string `json:"model"`
	RateLimitPerMinute int    `json:"rate_limit_per_minute"`
	MaxTokens          int    `json:"max_tokens"`
	WorkerCount        int    `json:"worker_count"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
}

type LoggingConfig struct {
	Level string `json:"level"`
}
