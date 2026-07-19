// This file tests Calendar timestamp migration behavior with PostgreSQL.
package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stark-lin/saturn/internal/platform/config"
)

func TestCalendarEndsAtMigrationWithPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("SATURN_TEST_DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		t.Skip("set SATURN_TEST_DATABASE_URL to run destructive Calendar migration test")
	}

	ctx := context.Background()
	handle, err := Open(ctx, config.DatabaseConfig{URL: databaseURL})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	preEndsAtMigrationsDir := t.TempDir()
	migrations, err := ListMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	for _, migration := range migrations {
		if migration.Name >= "000016_calendar_ends_at.sql" {
			continue
		}
		content, err := os.ReadFile(migration.Path)
		if err != nil {
			t.Fatalf("read migration %s: %v", migration.Name, err)
		}
		writeTestFile(t, filepath.Join(preEndsAtMigrationsDir, migration.Name), string(content))
	}
	if err := BootstrapSchema(ctx, handle.DB, preEndsAtMigrationsDir, BootstrapOptions{DropTables: true}); err != nil {
		t.Fatalf("bootstrap schema before Calendar ends_at: %v", err)
	}

	var ownerID int64
	if err := handle.DB.QueryRowContext(ctx, `
INSERT INTO users (ref_code, password_hash)
VALUES ('USR-00000001', 'hash')
RETURNING id`).Scan(&ownerID); err != nil {
		t.Fatalf("insert migration user: %v", err)
	}
	var aggregateID int64
	if err := handle.DB.QueryRowContext(ctx, `
INSERT INTO event_aggregates (owner_id, metadata)
VALUES ($1, '{"title":"Migration"}'::jsonb)
RETURNING id`, ownerID).Scan(&aggregateID); err != nil {
		t.Fatalf("insert migration aggregate: %v", err)
	}
	startsAt := time.Date(2026, time.June, 1, 9, 30, 0, 0, time.UTC)
	var eventID int64
	if err := handle.DB.QueryRowContext(ctx, `
INSERT INTO events (owner_id, aggregate_id, starts_at, duration_minutes, metadata)
VALUES ($1, $2, $3, 45, '{"title":"Migration event"}'::jsonb)
RETURNING id`, ownerID, aggregateID, startsAt).Scan(&eventID); err != nil {
		t.Fatalf("insert duration event: %v", err)
	}

	scripts, err := loadMigrationScripts(migrationsDir)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	var endsAtScript migrationScript
	for _, script := range scripts {
		if script.Name == "000016_calendar_ends_at.sql" {
			endsAtScript = script
			break
		}
	}
	if endsAtScript.Name == "" {
		t.Fatal("Calendar ends_at migration not found")
	}
	tx, err := handle.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin Calendar migration: %v", err)
	}
	for _, statement := range endsAtScript.Statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			_ = tx.Rollback()
			t.Fatalf("apply Calendar ends_at migration: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit Calendar ends_at migration: %v", err)
	}

	var endsAt time.Time
	if err := handle.DB.QueryRowContext(ctx, `SELECT ends_at FROM events WHERE id = $1`, eventID).Scan(&endsAt); err != nil {
		t.Fatalf("read migrated ends_at: %v", err)
	}
	if want := startsAt.Add(45 * time.Minute); !endsAt.Equal(want) {
		t.Fatalf("migrated ends_at = %s, want %s", endsAt, want)
	}
	var durationColumnExists bool
	if err := handle.DB.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'events'
      AND column_name = 'duration_minutes'
)`).Scan(&durationColumnExists); err != nil {
		t.Fatalf("inspect duration column: %v", err)
	}
	if durationColumnExists {
		t.Fatal("duration_minutes column still exists after migration")
	}
	if _, err := handle.DB.ExecContext(ctx, `UPDATE events SET ends_at = ends_at + INTERVAL '1 minute' WHERE id = $1`, eventID); err == nil {
		t.Fatal("expected immutable event trigger to reject ends_at update")
	}
}
