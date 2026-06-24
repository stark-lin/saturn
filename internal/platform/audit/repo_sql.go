// This file persists append-only platform audit logs through generated PostgreSQL queries.
package audit

import (
	"context"
	"database/sql"
	"fmt"

	auditsqlc "github.com/stark-lin/saturn/internal/platform/audit/sqlc"
	platformdb "github.com/stark-lin/saturn/internal/platform/db"
)

type SQLRepository struct {
	database *sql.DB
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{database: database}
}

func (r *SQLRepository) Insert(ctx context.Context, event Event) (Event, error) {
	if r == nil || r.database == nil {
		return Event{}, fmt.Errorf("audit database is required")
	}
	executor, ok := platformdb.TransactionExecutorFromContext(ctx)
	if !ok {
		return Event{}, fmt.Errorf("audit inserts require a SQL transaction")
	}
	row, err := auditsqlc.New(executor).InsertAuditLog(ctx, auditsqlc.InsertAuditLogParams{
		ActorType:     string(event.ActorType),
		ActorUserID:   nullableID(event.ActorUserID),
		Action:        string(event.Action),
		TargetRefCode: event.TargetRefCode,
		Result:        string(event.Result),
		Reason:        nullableText(event.Reason),
		SourceIp:      event.SourceIP,
		UserAgent:     nullableText(event.UserAgent),
	})
	if err != nil {
		return Event{}, err
	}
	return eventFromInsertRow(row), nil
}

func (r *SQLRepository) List(ctx context.Context, query Query) ([]Event, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("audit database is required")
	}
	rows, err := auditsqlc.New(platformdb.ExecutorFromContext(ctx, r.database)).ListAuditLogs(ctx, auditsqlc.ListAuditLogsParams{
		TargetRefCode: query.TargetRefCode,
		ActorUserID:   query.ActorUserID,
		Action:        string(query.Action),
		Result:        string(query.Result),
		PageLimit:     int32(query.Limit),
		PageOffset:    int32(query.Offset),
	})
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, eventFromListRow(row))
	}
	return events, nil
}

func eventFromInsertRow(row auditsqlc.InsertAuditLogRow) Event {
	return Event{
		ID:            row.ID,
		ActorType:     ActorType(row.ActorType),
		ActorUserID:   row.ActorUserID.Int64,
		Action:        Action(row.Action),
		TargetRefCode: row.TargetRefCode,
		Result:        Result(row.Result),
		Reason:        row.Reason.String,
		SourceIP:      row.SourceIp,
		UserAgent:     row.UserAgent.String,
		CreatedAt:     row.CreatedAt,
	}
}

func eventFromListRow(row auditsqlc.ListAuditLogsRow) Event {
	return Event{
		ID:            row.ID,
		ActorType:     ActorType(row.ActorType),
		ActorUserID:   row.ActorUserID.Int64,
		Action:        Action(row.Action),
		TargetRefCode: row.TargetRefCode,
		Result:        Result(row.Result),
		Reason:        row.Reason.String,
		SourceIP:      row.SourceIp,
		UserAgent:     row.UserAgent.String,
		CreatedAt:     row.CreatedAt,
	}
}

func nullableID(id int64) sql.NullInt64 {
	return sql.NullInt64{Int64: id, Valid: id > 0}
}

func nullableText(text string) sql.NullString {
	return sql.NullString{String: text, Valid: text != ""}
}
