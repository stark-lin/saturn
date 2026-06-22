// This file tests Saturn development schema bootstrap behavior.
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stark-lin/go-proj/internal/platform/config"
)

func TestListMigrationsReturnsSortedSQLFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "000002_second.sql"), "SELECT 2;")
	writeTestFile(t, filepath.Join(dir, "notes.txt"), "ignored")
	writeTestFile(t, filepath.Join(dir, "000001_first.sql"), "SELECT 1;")
	if err := os.Mkdir(filepath.Join(dir, "000000_dir.sql"), 0o700); err != nil {
		t.Fatalf("make migration-like directory: %v", err)
	}

	migrations, err := ListMigrations(dir)
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}

	got := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		got = append(got, migration.Name)
	}
	want := []string{"000001_first.sql", "000002_second.sql"}
	if !slices.Equal(got, want) {
		t.Fatalf("migration names = %v, want %v", got, want)
	}
}

func TestSplitSQLStatementsIgnoresSemicolonsInsideQuotedSections(t *testing.T) {
	input := `
CREATE TABLE first_table (value TEXT DEFAULT 'a;b');
CREATE TABLE "second;table" (id BIGINT);
/* comment ; */
DO $$ BEGIN RAISE NOTICE 'x;y'; END $$;
`

	got := splitSQLStatements(input)
	if len(got) != 3 {
		t.Fatalf("statement count = %d, want 3: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "first_table") {
		t.Fatalf("first statement = %q, want first_table", got[0])
	}
	if !strings.Contains(got[1], `"second;table"`) {
		t.Fatalf("second statement = %q, want quoted table name", got[1])
	}
	if !strings.Contains(got[2], "RAISE NOTICE 'x;y'") {
		t.Fatalf("third statement = %q, want dollar quoted block", got[2])
	}
}

func TestBootstrapSchemaAppliesMigrationsInOrder(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "000002_second.sql"), "SELECT 'second';")
	writeTestFile(t, filepath.Join(dir, "000001_first.sql"), "SELECT 'first';\nSELECT 'first again';")
	state := &recordingSQLState{}
	database := openRecordingDB(t, state)

	err := BootstrapSchema(context.Background(), database, dir, BootstrapOptions{})
	if err != nil {
		t.Fatalf("bootstrap schema: %v", err)
	}

	want := []string{"SELECT 'first'", "SELECT 'first again'", "SELECT 'second'"}
	if !slices.Equal(state.executedStatements(), want) {
		t.Fatalf("executed statements = %v, want %v", state.executedStatements(), want)
	}
	if !state.wasCommitted() {
		t.Fatal("expected transaction commit")
	}
	if state.wasRolledBack() {
		t.Fatal("did not expect transaction rollback")
	}
}

func TestBootstrapSchemaRollsBackOnSQLError(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "000001_first.sql"), "SELECT 'first';")
	writeTestFile(t, filepath.Join(dir, "000002_bad.sql"), "SELECT 'bad';")
	writeTestFile(t, filepath.Join(dir, "000003_after_bad.sql"), "SELECT 'after bad';")
	state := &recordingSQLState{failOn: "bad"}
	database := openRecordingDB(t, state)

	err := BootstrapSchema(context.Background(), database, dir, BootstrapOptions{})
	if err == nil {
		t.Fatal("expected schema bootstrap error")
	}
	if !strings.Contains(err.Error(), "000002_bad.sql") {
		t.Fatalf("expected migration name in error, got %v", err)
	}

	want := []string{"SELECT 'first'", "SELECT 'bad'"}
	if !slices.Equal(state.executedStatements(), want) {
		t.Fatalf("executed statements = %v, want %v", state.executedStatements(), want)
	}
	if state.wasCommitted() {
		t.Fatal("did not expect transaction commit")
	}
	if !state.wasRolledBack() {
		t.Fatal("expected transaction rollback")
	}
}

func TestBootstrapSchemaDropsCurrentSchemaTablesWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "000001_first.sql"), "CREATE TABLE first_table (id BIGINT);")
	state := &recordingSQLState{existingTables: []string{"first_table", "legacy_table"}}
	database := openRecordingDB(t, state)

	err := BootstrapSchema(context.Background(), database, dir, BootstrapOptions{DropTables: true})
	if err != nil {
		t.Fatalf("bootstrap schema: %v", err)
	}

	want := []string{
		"DROP TABLE IF EXISTS public.first_table, public.legacy_table CASCADE",
		"CREATE TABLE first_table (id BIGINT)",
	}
	if !slices.Equal(state.executedStatements(), want) {
		t.Fatalf("executed statements = %v, want %v", state.executedStatements(), want)
	}
	if !state.wasCommitted() {
		t.Fatal("expected transaction commit")
	}
}

func TestBootstrapSchemaSkipsMigrationsWhenRequiredTablesExist(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "000001_first.sql"), `
-- create existing tables
CREATE TABLE first_table (id BIGINT);
CREATE TABLE second_table (id BIGINT);
`)
	state := &recordingSQLState{existingTables: []string{"first_table", "second_table"}}
	database := openRecordingDB(t, state)

	err := BootstrapSchema(context.Background(), database, dir, BootstrapOptions{})
	if err != nil {
		t.Fatalf("bootstrap schema: %v", err)
	}

	if len(state.executedStatements()) != 0 {
		t.Fatalf("expected no migration statements, got %v", state.executedStatements())
	}
	if !state.wasCommitted() {
		t.Fatal("expected transaction commit")
	}
	if state.wasRolledBack() {
		t.Fatal("did not expect transaction rollback")
	}
}

func TestBootstrapSchemaFailsOnPartialExistingSchema(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "000001_first.sql"), `
CREATE TABLE first_table (id BIGINT);
CREATE TABLE second_table (id BIGINT);
`)
	state := &recordingSQLState{existingTables: []string{"first_table"}}
	database := openRecordingDB(t, state)

	err := BootstrapSchema(context.Background(), database, dir, BootstrapOptions{})
	if err == nil {
		t.Fatal("expected schema bootstrap error")
	}
	if !strings.Contains(err.Error(), "database.drop_tables=true") {
		t.Fatalf("expected drop_tables guidance, got %v", err)
	}
	if len(state.executedStatements()) != 0 {
		t.Fatalf("expected no migration statements, got %v", state.executedStatements())
	}
	if state.wasCommitted() {
		t.Fatal("did not expect transaction commit")
	}
	if !state.wasRolledBack() {
		t.Fatal("expected transaction rollback")
	}
}

func TestBootstrapSchemaWithPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("SATURN_TEST_DATABASE_URL")
	if strings.TrimSpace(databaseURL) == "" {
		t.Skip("set SATURN_TEST_DATABASE_URL to run destructive PostgreSQL schema bootstrap test")
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
	if err := BootstrapSchema(ctx, handle.DB, migrationsDir, BootstrapOptions{DropTables: true}); err != nil {
		t.Fatalf("bootstrap schema: %v", err)
	}

	for _, tableName := range []string{"users", "audit_logs", "files", "notes", "accounts", "events", "llm_requests", "system_settings", "user_preferences"} {
		t.Run(tableName, func(t *testing.T) {
			var exists bool
			err := handle.DB.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM information_schema.tables
    WHERE table_schema = 'public'
      AND table_name = $1
)`, tableName).Scan(&exists)
			if err != nil {
				t.Fatalf("query table existence: %v", err)
			}
			if !exists {
				t.Fatalf("expected table %q to exist", tableName)
			}
		})
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file %q: %v", path, err)
	}
}

const recordingSQLDriverName = "saturn_recording_sql"

var (
	registerRecordingSQLDriver sync.Once
	recordingDriver            = &recordingSQLDriver{states: make(map[string]*recordingSQLState)}
)

type recordingSQLDriver struct {
	mu     sync.Mutex
	states map[string]*recordingSQLState
}

func (d *recordingSQLDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	state := d.states[name]
	d.mu.Unlock()
	if state == nil {
		return nil, fmt.Errorf("recording sql state %q not found", name)
	}
	return &recordingSQLConn{state: state}, nil
}

func (d *recordingSQLDriver) setState(name string, state *recordingSQLState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.states[name] = state
}

func (d *recordingSQLDriver) deleteState(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.states, name)
}

type recordingSQLState struct {
	mu             sync.Mutex
	statements     []string
	existingTables []string
	failOn         string
	committed      bool
	rolledBack     bool
}

func (s *recordingSQLState) recordStatement(query string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statements = append(s.statements, strings.TrimSpace(query))
	if s.failOn != "" && strings.Contains(query, s.failOn) {
		return errors.New("forced execution failure")
	}
	return nil
}

func (s *recordingSQLState) executedStatements() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.statements)
}

func (s *recordingSQLState) commit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.committed = true
}

func (s *recordingSQLState) rollback() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rolledBack = true
}

func (s *recordingSQLState) queryRows(query string) driver.Rows {
	s.mu.Lock()
	defer s.mu.Unlock()

	values := make([][]driver.Value, 0, len(s.existingTables))
	for _, tableName := range s.existingTables {
		value := driver.Value(tableName)
		if strings.Contains(query, "quote_ident") {
			value = driver.Value("public." + tableName)
		}
		values = append(values, []driver.Value{value})
	}
	return &recordingSQLRows{
		columns: []string{"table_name"},
		values:  values,
	}
}

func (s *recordingSQLState) wasCommitted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.committed
}

func (s *recordingSQLState) wasRolledBack() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rolledBack
}

type recordingSQLConn struct {
	state *recordingSQLState
}

func (c *recordingSQLConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported by recording driver")
}

func (c *recordingSQLConn) Close() error {
	return nil
}

func (c *recordingSQLConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *recordingSQLConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	return &recordingSQLTx{state: c.state}, nil
}

func (c *recordingSQLConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if err := c.state.recordStatement(query); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (c *recordingSQLConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	return c.state.queryRows(query), nil
}

type recordingSQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *recordingSQLRows) Columns() []string {
	return r.columns
}

func (r *recordingSQLRows) Close() error {
	return nil
}

func (r *recordingSQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

type recordingSQLTx struct {
	state *recordingSQLState
}

func (tx *recordingSQLTx) Commit() error {
	tx.state.commit()
	return nil
}

func (tx *recordingSQLTx) Rollback() error {
	tx.state.rollback()
	return nil
}

func openRecordingDB(t *testing.T, state *recordingSQLState) *sql.DB {
	t.Helper()
	registerRecordingSQLDriver.Do(func() {
		sql.Register(recordingSQLDriverName, recordingDriver)
	})

	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	recordingDriver.setState(name, state)

	database, err := sql.Open(recordingSQLDriverName, name)
	if err != nil {
		t.Fatalf("open recording database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close recording database: %v", err)
		}
		recordingDriver.deleteState(name)
	})
	return database
}
