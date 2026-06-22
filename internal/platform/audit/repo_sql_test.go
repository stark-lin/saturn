// This file tests audit SQL repository row mapping helpers.
package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	auditsqlc "github.com/stark-lin/go-proj/internal/platform/audit/sqlc"
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
		ID: 12, ActorType: string(ActorTypeUser), ActorUserID: sql.NullInt64{Int64: 7, Valid: true},
		Action: string(ActionUpdate), TargetRefCode: "NTE-00000001", Result: string(ResultFailed),
		Reason: sql.NullString{String: "validation_failed", Valid: true}, SourceIp: "203.0.113.10",
		UserAgent: sql.NullString{String: "saturn-test", Valid: true}, CreatedAt: createdAt,
	})
	if inserted.ID != 12 || inserted.ActorUserID != 7 || inserted.Reason != "validation_failed" || inserted.UserAgent != "saturn-test" {
		t.Fatalf("insert row event = %#v", inserted)
	}
	if !inserted.CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at = %v, want %v", inserted.CreatedAt, createdAt)
	}

	listed := eventFromListRow(auditsqlc.ListAuditLogsRow{
		ID: 13, ActorType: string(ActorTypeSystem), Action: string(ActionExport),
		TargetRefCode: SystemTargetRefCode, Result: string(ResultSuccess), SourceIp: "127.0.0.1", CreatedAt: createdAt,
	})
	if listed.ActorUserID != 0 || listed.Reason != "" || listed.UserAgent != "" {
		t.Fatalf("list row event = %#v", listed)
	}
}

func TestNullableAuditSQLArguments(t *testing.T) {
	if value := nullableID(0); value.Valid {
		t.Fatalf("zero id should be null: %#v", value)
	}
	if value := nullableID(7); !value.Valid || value.Int64 != 7 {
		t.Fatalf("id = %#v", value)
	}
	if value := nullableText(""); value.Valid {
		t.Fatalf("empty text should be null: %#v", value)
	}
	if value := nullableText("reason"); !value.Valid || value.String != "reason" {
		t.Fatalf("text = %#v", value)
	}
}
