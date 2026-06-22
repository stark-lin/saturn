// This file persists object references through generated PostgreSQL queries.
package ref

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
	refsqlc "github.com/stark-lin/go-proj/internal/platform/ref/sqlc"
)

type SQLRepository struct {
	database *sql.DB
}

func NewSQLRepository(database *sql.DB) *SQLRepository {
	return &SQLRepository{database: database}
}

func (r *SQLRepository) NextSequence(ctx context.Context) (int64, error) {
	queries, err := r.queries(ctx)
	if err != nil {
		return 0, err
	}
	return queries.NextObjectRefSequence(ctx)
}

func (r *SQLRepository) Register(ctx context.Context, object ObjectRef) (ObjectRef, error) {
	queries, err := r.queries(ctx)
	if err != nil {
		return ObjectRef{}, err
	}
	row, err := queries.RegisterObjectRef(ctx, refsqlc.RegisterObjectRefParams{
		OwnerID:    object.OwnerID,
		RefCode:    object.RefCode,
		ObjectType: string(object.ObjectType),
		ObjectID:   object.ObjectID,
		Title:      object.Title,
		Tags:       object.Tags,
		Status:     object.Status,
	})
	if err != nil {
		return ObjectRef{}, err
	}
	return objectRefFromDatabaseRow(row), nil
}

func (r *SQLRepository) FindByCode(ctx context.Context, code string) (ObjectRef, error) {
	queries, err := r.queries(ctx)
	if err != nil {
		return ObjectRef{}, err
	}
	row, err := queries.FindObjectRefByCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return ObjectRef{}, ErrNotFound
	}
	if err != nil {
		return ObjectRef{}, err
	}
	return objectRefFromDatabaseRow(row), nil
}

func (r *SQLRepository) ListRecentByOwner(ctx context.Context, ownerID int64, limit int) ([]ObjectRef, error) {
	queries, err := r.queries(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queries.ListRecentObjectRefsByOwner(ctx, refsqlc.ListRecentObjectRefsByOwnerParams{
		OwnerID: ownerID,
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, err
	}
	objects := make([]ObjectRef, 0, len(rows))
	for _, row := range rows {
		objects = append(objects, objectRefFromDatabaseRow(row))
	}
	return objects, nil
}

func (r *SQLRepository) SearchByOwner(ctx context.Context, ownerID int64, query MetadataSearchQuery) ([]ObjectRef, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("object ref database is required")
	}
	statement, arguments := metadataSearchStatement(ownerID, query)
	rows, err := platformdb.ExecutorFromContext(ctx, r.database).QueryContext(ctx, statement, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	objects := make([]ObjectRef, 0, query.Limit)
	for rows.Next() {
		var object ObjectRef
		if err := rows.Scan(
			&object.ID,
			&object.OwnerID,
			&object.RefCode,
			&object.ObjectType,
			&object.ObjectID,
			&object.Title,
			pq.Array(&object.Tags),
			&object.Status,
			&object.CreatedAt,
			&object.UpdatedAt,
		); err != nil {
			return nil, err
		}
		object.Tags = nonNilTags(object.Tags)
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return objects, nil
}

func (r *SQLRepository) UpdateProjection(ctx context.Context, update ProjectionUpdate) (ObjectRef, error) {
	queries, err := r.queries(ctx)
	if err != nil {
		return ObjectRef{}, err
	}
	row, err := queries.UpdateObjectRefProjection(ctx, refsqlc.UpdateObjectRefProjectionParams{
		OwnerID:    update.OwnerID,
		ObjectType: string(update.ObjectType),
		ObjectID:   update.ObjectID,
		Title:      update.Title,
		Tags:       update.Tags,
		Status:     update.Status,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ObjectRef{}, ErrNotFound
	}
	if err != nil {
		return ObjectRef{}, err
	}
	return objectRefFromDatabaseRow(row), nil
}

func (r *SQLRepository) Delete(ctx context.Context, ownerID int64, objectType ObjectType, objectID int64) error {
	queries, err := r.queries(ctx)
	if err != nil {
		return err
	}
	return queries.DeleteObjectRef(ctx, refsqlc.DeleteObjectRefParams{
		OwnerID:    ownerID,
		ObjectType: string(objectType),
		ObjectID:   objectID,
	})
}

func (r *SQLRepository) queries(ctx context.Context) (*refsqlc.Queries, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("object ref database is required")
	}
	return refsqlc.New(platformdb.ExecutorFromContext(ctx, r.database)), nil
}

func metadataSearchStatement(ownerID int64, query MetadataSearchQuery) (string, []any) {
	var statement strings.Builder
	statement.WriteString(`SELECT id, owner_id, ref_code, object_type, object_id, title, tags, status, created_at, updated_at
FROM object_refs
WHERE owner_id = $1`)
	arguments := []any{ownerID}
	addArgument := func(value any) string {
		arguments = append(arguments, value)
		return fmt.Sprintf("$%d", len(arguments))
	}
	if len(query.ObjectTypes) > 0 {
		statement.WriteString("\n  AND object_type = ANY(")
		statement.WriteString(addArgument(pq.Array(objectTypeStrings(query.ObjectTypes))))
		statement.WriteString("::text[])")
	}
	if len(query.Statuses) > 0 {
		statement.WriteString("\n  AND status = ANY(")
		statement.WriteString(addArgument(pq.Array(query.Statuses)))
		statement.WriteString("::text[])")
	}
	if len(query.Tags) > 0 {
		statement.WriteString("\n  AND tags @> ")
		statement.WriteString(addArgument(pq.Array(query.Tags)))
		statement.WriteString("::text[]")
	}
	if query.CreatedFrom != nil {
		statement.WriteString("\n  AND created_at >= ")
		statement.WriteString(addArgument(*query.CreatedFrom))
	}
	if query.CreatedTo != nil {
		statement.WriteString("\n  AND created_at < ")
		statement.WriteString(addArgument(*query.CreatedTo))
	}
	if query.UpdatedFrom != nil {
		statement.WriteString("\n  AND updated_at >= ")
		statement.WriteString(addArgument(*query.UpdatedFrom))
	}
	if query.UpdatedTo != nil {
		statement.WriteString("\n  AND updated_at < ")
		statement.WriteString(addArgument(*query.UpdatedTo))
	}
	statement.WriteString("\nORDER BY ")
	statement.WriteString(metadataSearchOrderBy(query.Sort))
	statement.WriteString("\nLIMIT ")
	statement.WriteString(addArgument(query.Limit))
	return statement.String(), arguments
}

func metadataSearchOrderBy(sort MetadataSearchSort) string {
	field := sort.Field
	if field == "" {
		field = MetadataSearchSortUpdatedAt
	}
	direction := "DESC"
	if sort.Direction == MetadataSearchSortAscending {
		direction = "ASC"
	}
	switch field {
	case MetadataSearchSortCreatedAt:
		return "created_at " + direction + ", ref_code " + direction
	case MetadataSearchSortRefCode:
		return "ref_code " + direction
	default:
		return "updated_at " + direction + ", ref_code " + direction
	}
}

func objectTypeStrings(objectTypes []ObjectType) []string {
	values := make([]string, 0, len(objectTypes))
	for _, objectType := range objectTypes {
		values = append(values, string(objectType))
	}
	return values
}

func objectRefFromDatabaseRow(row refsqlc.ObjectRef) ObjectRef {
	return ObjectRef{
		ID:         row.ID,
		OwnerID:    row.OwnerID,
		RefCode:    row.RefCode,
		ObjectType: ObjectType(row.ObjectType),
		ObjectID:   row.ObjectID,
		Title:      row.Title,
		Tags:       nonNilTags(row.Tags),
		Status:     row.Status,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
