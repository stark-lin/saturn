// This file tests audit SQL repository row mapping helpers.
package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	auditsqlc "github.com/stark-lin/saturn/internal/platform/audit/sqlc"
)

func TestNewSQLRepositoryStoresDatabaseDependency(t *testing.T) {
	repo := NewSQLRepository(nil)
	if repo == nil {
		t.Fatal("expected repository")
	}
}

func TestSQLRepositoryRejectsMissingDatabase(t *testing.T) {
	repo := NewSQLRepository(nil)
	if _, err := repo.Insert(context.Background(), Event{}); err == nil {
		t.Fatal("expected insert database error")
	}
	if _, err := repo.List(context.Background(), Query{}); err == nil {
		t.Fatal("expected list database error")
	}
}

func TestAuditSQLRowMappingPreservesNullableFields(t *testing.T) {
	createdAt := time.Unix(100, 0).UTC()
	inserted := eventFromInsertRow(auditsqlc.InsertAuditLogRow{
		ID: 12, ActorRefCode: "KEY-4F8A2C10",
		Action: string(ActionUpdate), TargetRefCode: "NTE-00000001", Result: string(ResultFailed),
		Reason: sql.NullString{String: "validation_failed", Valid: true}, SourceIp: "203.0.113.10",
		UserAgent: sql.NullString{String: "saturn-test", Valid: true}, CreatedAt: createdAt,
	})
	if inserted.ID != 12 || inserted.ActorRefCode != "KEY-4F8A2C10" || inserted.Reason != "validation_failed" || inserted.UserAgent != "saturn-test" {
		t.Fatalf("insert row event = %#v", inserted)
	}
	if !inserted.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at = %v, want %v", inserted.CreatedAt, createdAt)
	}

	listed := eventFromListRow(auditsqlc.ListAuditLogsRow{
		ID: 13, ActorRefCode: SystemTargetRefCode, Action: string(ActionExport),
		TargetRefCode: SystemTargetRefCode, Result: string(ResultSuccess), SourceIp: "127.0.0.1", CreatedAt: createdAt,
	})
	if listed.ActorRefCode != SystemTargetRefCode || listed.Reason != "" || listed.UserAgent != "" {
		t.Fatalf("list row event = %#v", listed)
	}
}

func TestNullableAuditSQLArguments(t *testing.T) {
	if value := nullableText(""); value.Valid {
		t.Fatalf("empty text should be null: %#v", value)
	}
	if value := nullableText("reason"); !value.Valid || value.String != "reason" {
		t.Fatalf("text = %#v", value)
	}
}
