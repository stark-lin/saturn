// This file opens and owns Saturn PostgreSQL connections.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/stark-lin/saturn/internal/platform/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const postgresDriverName = "pgx"

type Handle struct {
	URL string
	DB  *sql.DB
}

func Open(ctx context.Context, cfg config.DatabaseConfig) (*Handle, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("database url is required")
	}

	database, err := sql.Open(postgresDriverName, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	database.SetMaxOpenConns(10)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(30 * time.Minute)

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Handle{
		URL: cfg.URL,
		DB:  database,
	}, nil
}

func (h *Handle) Close() error {
	if h == nil || h.DB == nil {
		return nil
	}
	return h.DB.Close()
}
