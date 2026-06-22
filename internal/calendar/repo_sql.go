// This file persists Calendar event aggregates through PostgreSQL.
package calendar

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
)

type SQLRepository struct {
	database *sql.DB
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{database: database}
}

func (r *SQLRepository) ListEventAggregates(ctx context.Context, scope auth.Scope, query EventAggregateQuery) (EventAggregatePage, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return EventAggregatePage{}, err
	}
	statement := eventAggregateBaseSQL + `
ORDER BY a.created_at DESC, aggregate_ref.ref_code DESC
LIMIT $1 OFFSET $2`
	arguments := []any{query.Limit + 1, query.Offset}
	if !scope.All {
		statement = eventAggregateBaseSQL + `
WHERE a.owner_id = $1
ORDER BY a.created_at DESC, aggregate_ref.ref_code DESC
LIMIT $2 OFFSET $3`
		arguments = []any{scope.OwnerID, query.Limit + 1, query.Offset}
	}
	rows, err := executor.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return EventAggregatePage{}, err
	}
	defer rows.Close()

	aggregates := make([]EventAggregate, 0, query.Limit+1)
	for rows.Next() {
		aggregate, err := scanEventAggregate(rows)
		if err != nil {
			return EventAggregatePage{}, err
		}
		aggregates = append(aggregates, aggregate)
	}
	if err := rows.Err(); err != nil {
		return EventAggregatePage{}, err
	}
	hasMore := len(aggregates) > query.Limit
	if hasMore {
		aggregates = aggregates[:query.Limit]
	}
	return EventAggregatePage{Aggregates: aggregates, Limit: query.Limit, Offset: query.Offset, HasMore: hasMore}, nil
}

func (r *SQLRepository) CreateEventAggregate(ctx context.Context, ownerID int64, input CreateEventAggregateInput) (EventAggregate, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return EventAggregate{}, err
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return EventAggregate{}, err
	}
	var aggregate EventAggregate
	var metadataJSON []byte
	err = executor.QueryRowContext(ctx, `
INSERT INTO event_aggregates (owner_id, metadata)
VALUES ($1, $2)
RETURNING id, owner_id, metadata, created_at`, ownerID, metadata).Scan(
		&aggregate.ID, &aggregate.OwnerID, &metadataJSON, &aggregate.CreatedAt,
	)
	if err != nil {
		return EventAggregate{}, err
	}
	if err := json.Unmarshal(metadataJSON, &aggregate.Metadata); err != nil {
		return EventAggregate{}, err
	}
	return aggregate, nil
}

func (r *SQLRepository) FindEventAggregateByRefCode(ctx context.Context, scope auth.Scope, refCode string) (EventAggregate, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return EventAggregate{}, err
	}
	statement := eventAggregateBaseSQL + `WHERE aggregate_ref.ref_code = $1`
	arguments := []any{refCode}
	if !scope.All {
		statement += ` AND a.owner_id = $2`
		arguments = append(arguments, scope.OwnerID)
	}
	aggregate, err := scanEventAggregate(executor.QueryRowContext(ctx, statement, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return EventAggregate{}, ErrEventAggregateNotFound
	}
	if err != nil {
		return EventAggregate{}, err
	}
	return aggregate, nil
}

func (r *SQLRepository) LockEventAggregateByRefCode(ctx context.Context, refCode string) (EventAggregate, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return EventAggregate{}, err
	}
	aggregate, err := scanEventAggregate(executor.QueryRowContext(ctx, eventAggregateBaseSQL+`
WHERE aggregate_ref.ref_code = $1
FOR UPDATE OF a`, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return EventAggregate{}, ErrEventAggregateNotFound
	}
	return aggregate, err
}

func (r *SQLRepository) DeleteEventAggregate(ctx context.Context, ownerID int64, aggregateID int64) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `DELETE FROM event_aggregates WHERE owner_id = $1 AND id = $2`, ownerID, aggregateID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrEventAggregateNotFound
	}
	return nil
}

func (r *SQLRepository) ListViewEvents(ctx context.Context, scope auth.Scope, query CalendarViewQuery) (EventPage, error) {
	statement := eventBaseSQL + `
WHERE e.status = 'scheduled'
  AND e.starts_at >= $1
  AND e.starts_at <= $2
ORDER BY e.starts_at ASC, e.id ASC
LIMIT $3 OFFSET $4`
	arguments := []any{query.From, query.To, query.Limit + 1, query.Offset}
	if !scope.All {
		statement = eventBaseSQL + `
WHERE e.owner_id = $1
  AND e.status = 'scheduled'
  AND e.starts_at >= $2
  AND e.starts_at <= $3
ORDER BY e.starts_at ASC, e.id ASC
LIMIT $4 OFFSET $5`
		arguments = []any{scope.OwnerID, query.From, query.To, query.Limit + 1, query.Offset}
	}
	page, err := r.listEvents(ctx, statement, arguments, query.Limit)
	page.Offset = query.Offset
	return page, err
}

func (r *SQLRepository) CreateEvent(ctx context.Context, ownerID int64, aggregateID int64, input CreateEventInput) (Event, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Event{}, err
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return Event{}, err
	}
	var event Event
	var metadataJSON []byte
	err = executor.QueryRowContext(ctx, `
INSERT INTO events (owner_id, aggregate_id, starts_at, duration_minutes, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, owner_id, aggregate_id, starts_at, duration_minutes, metadata, status, created_at, updated_at`,
		ownerID, aggregateID, input.StartsAt, input.DurationMinutes, metadata).Scan(
		&event.ID, &event.OwnerID, &event.AggregateID, &event.StartsAt, &event.DurationMinutes,
		&metadataJSON, &event.Status, &event.CreatedAt, &event.UpdatedAt,
	)
	if err != nil {
		return Event{}, err
	}
	if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
		return Event{}, err
	}
	return event, nil
}

func (r *SQLRepository) ListEventsForAggregate(ctx context.Context, ownerID int64, aggregateID int64) ([]Event, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := executor.QueryContext(ctx, eventBaseSQL+`
WHERE e.owner_id = $1 AND e.aggregate_id = $2
ORDER BY e.starts_at ASC, e.id ASC`, ownerID, aggregateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return eventsFromRows(rows)
}

func (r *SQLRepository) ListEventIDsForAggregate(ctx context.Context, ownerID int64, aggregateID int64) ([]int64, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := executor.QueryContext(ctx, `
SELECT id
FROM events
WHERE owner_id = $1 AND aggregate_id = $2
ORDER BY id ASC`, ownerID, aggregateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *SQLRepository) FindEventByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Event, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Event{}, err
	}
	statement := eventBaseSQL + `WHERE event_ref.ref_code = $1`
	arguments := []any{refCode}
	if !scope.All {
		statement += ` AND e.owner_id = $2`
		arguments = append(arguments, scope.OwnerID)
	}
	event, err := scanEvent(executor.QueryRowContext(ctx, statement, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

func (r *SQLRepository) LockEventByRefCode(ctx context.Context, refCode string) (Event, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Event{}, err
	}
	event, err := scanEvent(executor.QueryRowContext(ctx, eventBaseSQL+`
WHERE event_ref.ref_code = $1
FOR UPDATE OF e`, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

func (r *SQLRepository) FinishEvent(ctx context.Context, event Event) (Event, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Event{}, err
	}
	err = executor.QueryRowContext(ctx, `
UPDATE events
SET status = 'finished', updated_at = NOW()
WHERE owner_id = $1 AND id = $2 AND status = 'scheduled'
RETURNING status, updated_at`, event.OwnerID, event.ID).Scan(&event.Status, &event.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrEventAlreadyFinished
	}
	return event, err
}

func (r *SQLRepository) VoidEvent(ctx context.Context, event Event) (Event, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Event{}, err
	}
	err = executor.QueryRowContext(ctx, `
UPDATE events
SET status = 'voided', updated_at = NOW()
WHERE owner_id = $1 AND id = $2 AND status IN ('scheduled', 'finished')
RETURNING status, updated_at`, event.OwnerID, event.ID).Scan(&event.Status, &event.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrEventAlreadyVoided
	}
	return event, err
}

func (r *SQLRepository) listEvents(ctx context.Context, statement string, arguments []any, limit int) (EventPage, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return EventPage{}, err
	}
	rows, err := executor.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return EventPage{}, err
	}
	defer rows.Close()
	events, err := eventsFromRows(rows)
	if err != nil {
		return EventPage{}, err
	}
	hasMore := limit > 0 && len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	return EventPage{Events: events, Limit: limit, HasMore: hasMore}, nil
}

func eventsFromRows(rows *sql.Rows) ([]Event, error) {
	events := make([]Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *SQLRepository) executor(ctx context.Context) (platformdb.Executor, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("calendar database is required")
	}
	return platformdb.ExecutorFromContext(ctx, r.database), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEventAggregate(row rowScanner) (EventAggregate, error) {
	var aggregate EventAggregate
	var metadataJSON []byte
	err := row.Scan(&aggregate.ID, &aggregate.OwnerID, &aggregate.ObjectRefID, &aggregate.RefCode, pq.Array(&aggregate.Tags), &metadataJSON, &aggregate.CreatedAt)
	if err != nil {
		return EventAggregate{}, err
	}
	aggregate.Tags = nonNilTags(aggregate.Tags)
	if err := json.Unmarshal(metadataJSON, &aggregate.Metadata); err != nil {
		return EventAggregate{}, err
	}
	return aggregate, nil
}

func scanEvent(row rowScanner) (Event, error) {
	var event Event
	var metadataJSON []byte
	err := row.Scan(
		&event.ID, &event.OwnerID, &event.ObjectRefID, &event.RefCode, pq.Array(&event.Tags), &event.AggregateID, &event.AggregateRefCode,
		&event.StartsAt, &event.DurationMinutes, &metadataJSON, &event.Status, &event.CreatedAt, &event.UpdatedAt,
	)
	if err != nil {
		return Event{}, err
	}
	event.Tags = nonNilTags(event.Tags)
	if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
		return Event{}, err
	}
	return event, nil
}

const eventAggregateBaseSQL = `
SELECT a.id, a.owner_id, aggregate_ref.id, aggregate_ref.ref_code, aggregate_ref.tags, a.metadata, a.created_at
FROM event_aggregates AS a
JOIN object_refs AS aggregate_ref
  ON aggregate_ref.owner_id = a.owner_id
 AND aggregate_ref.object_type = 'event_aggregate'
 AND aggregate_ref.object_id = a.id
`

const eventBaseSQL = `
SELECT e.id, e.owner_id, event_ref.id, event_ref.ref_code, event_ref.tags, e.aggregate_id, aggregate_ref.ref_code,
       e.starts_at, e.duration_minutes, e.metadata, e.status, e.created_at, e.updated_at
FROM events AS e
JOIN object_refs AS event_ref
  ON event_ref.owner_id = e.owner_id
 AND event_ref.object_type = 'event'
 AND event_ref.object_id = e.id
JOIN object_refs AS aggregate_ref
  ON aggregate_ref.owner_id = e.owner_id
 AND aggregate_ref.object_type = 'event_aggregate'
 AND aggregate_ref.object_id = e.aggregate_id
`

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
