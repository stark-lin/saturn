// This file applies SQL migration files for Saturn development schema setup.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type Migration struct {
	Name string
	Path string
}

type BootstrapOptions struct {
	DropTables bool
}

type migrationScript struct {
	Migration
	Statements []string
}

func ListMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		migrations = append(migrations, Migration{
			Name: entry.Name(),
			Path: filepath.Join(dir, entry.Name()),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name < migrations[j].Name
	})
	return migrations, nil
}

func BootstrapSchema(ctx context.Context, database *sql.DB, dir string, options BootstrapOptions) error {
	if database == nil {
		return fmt.Errorf("database is required")
	}

	scripts, err := loadMigrationScripts(dir)
	if err != nil {
		return err
	}
	if len(scripts) == 0 {
		return nil
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema bootstrap: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if options.DropTables {
		if err := dropCurrentSchemaTables(ctx, tx); err != nil {
			return err
		}
	} else {
		status, missingTables, err := inspectSchemaStatus(ctx, tx, requiredTableNames(scripts))
		if err != nil {
			return err
		}
		switch status {
		case schemaStatusEmpty:
		case schemaStatusComplete:
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit schema bootstrap: %w", err)
			}
			committed = true
			return nil
		case schemaStatusPartial:
			return fmt.Errorf("existing database schema is incomplete; missing tables: %s; set database.drop_tables=true to rebuild the development schema", strings.Join(missingTables, ", "))
		}
	}

	for _, script := range scripts {
		for _, statement := range script.Statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply migration %s: %w", script.Name, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema bootstrap: %w", err)
	}
	committed = true
	return nil
}

func loadMigrationScripts(dir string) ([]migrationScript, error) {
	migrations, err := ListMigrations(dir)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	scripts := make([]migrationScript, 0, len(migrations))
	for _, migration := range migrations {
		content, err := os.ReadFile(migration.Path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", migration.Name, err)
		}
		scripts = append(scripts, migrationScript{
			Migration:  migration,
			Statements: splitSQLStatements(string(content)),
		})
	}
	return scripts, nil
}

type schemaStatus int

const (
	schemaStatusEmpty schemaStatus = iota
	schemaStatusComplete
	schemaStatusPartial
)

func inspectSchemaStatus(ctx context.Context, tx *sql.Tx, requiredTables []string) (schemaStatus, []string, error) {
	if len(requiredTables) == 0 {
		return schemaStatusEmpty, nil, nil
	}

	existingTables, err := currentSchemaTables(ctx, tx)
	if err != nil {
		return schemaStatusEmpty, nil, err
	}

	existingSet := make(map[string]struct{}, len(existingTables))
	for _, tableName := range existingTables {
		existingSet[tableName] = struct{}{}
	}

	managedTableCount := 0
	missingTables := make([]string, 0)
	for _, tableName := range requiredTables {
		if _, ok := existingSet[tableName]; ok {
			managedTableCount++
			continue
		}
		missingTables = append(missingTables, tableName)
	}
	if managedTableCount == 0 {
		return schemaStatusEmpty, nil, nil
	}
	if len(missingTables) == 0 {
		return schemaStatusComplete, nil, nil
	}
	return schemaStatusPartial, missingTables, nil
}

func currentSchemaTables(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = current_schema()
  AND table_type = 'BASE TABLE'
ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("inspect database schema tables: %w", err)
	}
	defer rows.Close()

	tableNames := make([]string, 0)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("scan database schema table: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("inspect database schema tables: %w", err)
	}
	return tableNames, nil
}

func dropCurrentSchemaTables(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
SELECT quote_ident(table_schema) || '.' || quote_ident(table_name)
FROM information_schema.tables
WHERE table_schema = current_schema()
  AND table_type = 'BASE TABLE'
ORDER BY table_name`)
	if err != nil {
		return fmt.Errorf("list database schema tables to drop: %w", err)
	}
	defer rows.Close()

	tableRefs := make([]string, 0)
	for rows.Next() {
		var tableRef string
		if err := rows.Scan(&tableRef); err != nil {
			return fmt.Errorf("scan database schema table to drop: %w", err)
		}
		tableRefs = append(tableRefs, tableRef)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list database schema tables to drop: %w", err)
	}
	if len(tableRefs) == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS "+strings.Join(tableRefs, ", ")+" CASCADE"); err != nil {
		return fmt.Errorf("drop database schema tables: %w", err)
	}
	return nil
}

func requiredTableNames(scripts []migrationScript) []string {
	tableNames := make([]string, 0)
	seen := make(map[string]struct{})
	for _, script := range scripts {
		for _, statement := range script.Statements {
			tableName := createTableName(statement)
			if tableName == "" {
				continue
			}
			if _, ok := seen[tableName]; ok {
				continue
			}
			seen[tableName] = struct{}{}
			tableNames = append(tableNames, tableName)
		}
	}
	sort.Strings(tableNames)
	return tableNames
}

func createTableName(statement string) string {
	statement = trimLeadingSQLComments(statement)
	upperStatement := strings.ToUpper(statement)
	if !strings.HasPrefix(upperStatement, "CREATE TABLE") {
		return ""
	}
	if len(statement) > len("CREATE TABLE") {
		next := rune(statement[len("CREATE TABLE")])
		if !unicode.IsSpace(next) {
			return ""
		}
	}
	rest := strings.TrimSpace(statement[len("CREATE TABLE"):])
	upperRest := strings.ToUpper(rest)
	if strings.HasPrefix(upperRest, "IF NOT EXISTS") {
		rest = strings.TrimSpace(rest[len("IF NOT EXISTS"):])
	}
	rawName := readSQLName(rest)
	if rawName == "" {
		return ""
	}
	if strings.Contains(rawName, ".") {
		parts := strings.Split(rawName, ".")
		rawName = parts[len(parts)-1]
	}
	return strings.Trim(rawName, `"`)
}

func trimLeadingSQLComments(statement string) string {
	for {
		statement = strings.TrimSpace(statement)
		switch {
		case strings.HasPrefix(statement, "--"):
			newline := strings.IndexByte(statement, '\n')
			if newline < 0 {
				return ""
			}
			statement = statement[newline+1:]
		case strings.HasPrefix(statement, "/*"):
			end := strings.Index(statement, "*/")
			if end < 0 {
				return ""
			}
			statement = statement[end+2:]
		default:
			return statement
		}
	}
}

func readSQLName(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	for index, ch := range input {
		if unicode.IsSpace(ch) || ch == '(' {
			return input[:index]
		}
	}
	return input
}

func splitSQLStatements(sqlContent string) []string {
	statements := make([]string, 0)
	start := 0
	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false
	dollarQuoteTag := ""

	for index := 0; index < len(sqlContent); index++ {
		ch := sqlContent[index]
		next := byte(0)
		if index+1 < len(sqlContent) {
			next = sqlContent[index+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				index++
			}
			continue
		}
		if dollarQuoteTag != "" {
			if strings.HasPrefix(sqlContent[index:], dollarQuoteTag) {
				index += len(dollarQuoteTag) - 1
				dollarQuoteTag = ""
			}
			continue
		}
		if inSingleQuote {
			if ch == '\'' {
				if next == '\'' {
					index++
					continue
				}
				inSingleQuote = false
			}
			continue
		}
		if inDoubleQuote {
			if ch == '"' {
				inDoubleQuote = false
			}
			continue
		}

		switch {
		case ch == '-' && next == '-':
			inLineComment = true
			index++
		case ch == '/' && next == '*':
			inBlockComment = true
			index++
		case ch == '\'':
			inSingleQuote = true
		case ch == '"':
			inDoubleQuote = true
		case ch == '$':
			if tag := readDollarQuoteTag(sqlContent[index:]); tag != "" {
				dollarQuoteTag = tag
				index += len(tag) - 1
			}
		case ch == ';':
			statement := strings.TrimSpace(sqlContent[start:index])
			if statement != "" {
				statements = append(statements, statement)
			}
			start = index + 1
		}
	}

	tail := strings.TrimSpace(sqlContent[start:])
	if tail != "" {
		statements = append(statements, tail)
	}
	return statements
}

func readDollarQuoteTag(input string) string {
	if input == "" || input[0] != '$' {
		return ""
	}
	for index := 1; index < len(input); index++ {
		ch := rune(input[index])
		if ch == '$' {
			return input[:index+1]
		}
		if ch != '_' && !unicode.IsLetter(ch) && !unicode.IsDigit(ch) {
			return ""
		}
	}
	return ""
}
