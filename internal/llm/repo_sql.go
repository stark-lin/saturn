// This file persists LLM sessions, requests, results, and references through PostgreSQL.
package llm

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

func (r *SQLRepository) ListSessions(ctx context.Context, scope auth.Scope, limit int, offset int) (SessionPage, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return SessionPage{}, err
	}
	statement := sessionBaseSQL + `
ORDER BY s.created_at DESC, s.id DESC
LIMIT $1 OFFSET $2`
	arguments := []any{limit + 1, offset}
	if !scope.All {
		statement = sessionBaseSQL + `
WHERE s.owner_id = $1
ORDER BY s.created_at DESC, s.id DESC
LIMIT $2 OFFSET $3`
		arguments = []any{scope.OwnerID, limit + 1, offset}
	}
	rows, err := executor.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return SessionPage{}, err
	}
	defer rows.Close()

	sessions := make([]Session, 0, limit+1)
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return SessionPage{}, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return SessionPage{}, err
	}
	hasMore := len(sessions) > limit
	if hasMore {
		sessions = sessions[:limit]
	}
	return SessionPage{Sessions: sessions, Limit: limit, Offset: offset, HasMore: hasMore}, nil
}

func (r *SQLRepository) CreateSession(ctx context.Context, ownerID int64, input CreateSessionInput) (Session, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Session{}, err
	}
	var session Session
	err = executor.QueryRowContext(ctx, `
INSERT INTO llm_sessions (owner_id, title)
VALUES ($1, $2)
RETURNING id, owner_id, title, status, created_at, updated_at`,
		ownerID, input.Title).Scan(
		&session.ID, &session.OwnerID, &session.Title, &session.Status, &session.CreatedAt, &session.UpdatedAt,
	)
	return session, err
}

func (r *SQLRepository) FindSessionByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Session, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Session{}, err
	}
	statement := sessionBaseSQL + `WHERE session_ref.ref_code = $1`
	arguments := []any{refCode}
	if !scope.All {
		statement += ` AND s.owner_id = $2`
		arguments = append(arguments, scope.OwnerID)
	}
	session, err := scanSession(executor.QueryRowContext(ctx, statement, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (r *SQLRepository) LockSessionByRefCode(ctx context.Context, refCode string) (Session, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Session{}, err
	}
	session, err := scanSession(executor.QueryRowContext(ctx, sessionBaseSQL+`WHERE session_ref.ref_code = $1 FOR UPDATE OF s`, refCode))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	return session, err
}

func (r *SQLRepository) DeleteSession(ctx context.Context, ownerID int64, sessionID int64) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	if _, err := executor.ExecContext(ctx, `SELECT set_config('saturn.deleting_llm_session', 'on', true)`); err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `DELETE FROM llm_sessions WHERE owner_id = $1 AND id = $2`, ownerID, sessionID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *SQLRepository) CreateRequest(ctx context.Context, ownerID int64, sessionID int64, input PersistedRequestInput) (Request, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Request{}, err
	}
	var request Request
	var completedAt sql.NullTime
	err = executor.QueryRowContext(ctx, `
INSERT INTO llm_requests (owner_id, session_id, actor_user_id, prompt, model, max_tokens, context_json, request_json)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, owner_id, session_id, actor_user_id, prompt, model, max_tokens, context_json, request_json,
          response_status, content, error_code, error_message, response_json,
          created_at, updated_at, completed_at`,
		ownerID, sessionID, input.ActorUserID, input.Prompt, input.Model, input.MaxTokens, jsonArgument(input.ContextJSON), jsonArgument(input.RequestJSON)).Scan(
		&request.ID, &request.OwnerID, &request.SessionID, &request.ActorUserID, &request.Prompt, &request.Model,
		&request.MaxTokens, &request.ContextJSON, &request.RequestJSON, &request.ResponseStatus,
		&request.ResponseContent, &request.ResponseErrorCode, &request.ResponseErrorMessage,
		&request.ResponseJSON, &request.CreatedAt, &request.UpdatedAt, &completedAt,
	)
	if completedAt.Valid {
		request.CompletedAt = &completedAt.Time
	}
	return request, err
}

func (r *SQLRepository) FindRequestByRefCode(ctx context.Context, scope auth.Scope, refCode string) (Request, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Request{}, err
	}
	statement := requestBaseSQL + `WHERE request_ref.ref_code = $1`
	arguments := []any{refCode}
	if !scope.All {
		statement += ` AND request.owner_id = $2`
		arguments = append(arguments, scope.OwnerID)
	}
	request, err := scanRequestWithRef(executor.QueryRowContext(ctx, statement, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return Request{}, ErrRequestNotFound
	}
	if err != nil {
		return Request{}, err
	}
	request.References, err = r.listRequestReferences(ctx, request.ID)
	return request, err
}

func (r *SQLRepository) InsertRequestReference(ctx context.Context, requestID int64, reference ResolvedReference) (RequestReference, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return RequestReference{}, err
	}
	return scanRequestReference(executor.QueryRowContext(ctx, `
INSERT INTO llm_request_references (
    request_id, object_ref_id, ref_code, module, object_type, title, status, payload_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, request_id, object_ref_id, ref_code, module, object_type, title, status, payload_json, created_at`,
		requestID, reference.ObjectRefID, reference.RefCode, reference.Module, reference.ObjectType,
		reference.Title, reference.Status, jsonArgument(reference.PayloadJSON)))
}

func (r *SQLRepository) ListRequests(ctx context.Context, ownerID int64, sessionID int64, limit int, offset int) ([]Request, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := executor.QueryContext(ctx, requestBaseSQL+`
WHERE request.owner_id = $1
  AND request.session_id = $2
ORDER BY request.created_at ASC, request.id ASC
LIMIT $3 OFFSET $4`, ownerID, sessionID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]Request, 0)
	for rows.Next() {
		request, err := scanRequestWithRef(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range requests {
		requests[index].References, err = r.listRequestReferences(ctx, requests[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return requests, nil
}

func (r *SQLRepository) ListRequestDeletionTargets(ctx context.Context, ownerID int64, sessionID int64) ([]RequestDeletionTarget, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := executor.QueryContext(ctx, `
SELECT request.id, request_ref.ref_code
FROM llm_requests AS request
JOIN object_refs AS request_ref
  ON request_ref.owner_id = request.owner_id
 AND request_ref.object_type = 'llm_request'
 AND request_ref.object_id = request.id
WHERE request.owner_id = $1
  AND request.session_id = $2
ORDER BY request.created_at ASC, request.id ASC`, ownerID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := make([]RequestDeletionTarget, 0)
	for rows.Next() {
		var target RequestDeletionTarget
		if err := rows.Scan(&target.ID, &target.RefCode); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (r *SQLRepository) ClaimNextQueuedRequest(ctx context.Context) (Request, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Request{}, err
	}
	request, err := scanRequestWithRef(executor.QueryRowContext(ctx, `
WITH candidate AS (
    SELECT id
    FROM llm_requests
    WHERE response_status = 'queued'
    ORDER BY created_at ASC, id ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
),
updated AS (
    UPDATE llm_requests AS request
    SET response_status = 'running',
        updated_at = NOW()
    FROM candidate
    WHERE request.id = candidate.id
    RETURNING request.id, request.owner_id, request.session_id, request.actor_user_id,
              request.prompt, request.model, request.max_tokens, request.context_json, request.request_json,
              request.response_status, request.content, request.error_code, request.error_message, request.response_json,
              request.created_at, request.updated_at, request.completed_at
)
SELECT updated.id, updated.owner_id, updated.session_id, updated.actor_user_id, request_ref.id, request_ref.ref_code, request_ref.tags,
       updated.prompt, updated.model, updated.max_tokens, updated.context_json, updated.request_json,
       updated.response_status, updated.content, updated.error_code, updated.error_message, updated.response_json,
       updated.created_at, updated.updated_at, updated.completed_at
FROM updated
JOIN object_refs AS request_ref
  ON request_ref.owner_id = updated.owner_id
 AND request_ref.object_type = 'llm_request'
 AND request_ref.object_id = updated.id`))
	if errors.Is(err, sql.ErrNoRows) {
		return Request{}, ErrNoQueuedRequest
	}
	return request, err
}

func (r *SQLRepository) CompleteRequestResponse(ctx context.Context, ownerID int64, requestID int64, input CompleteResponseInput) (Response, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Response{}, err
	}
	response, err := scanResponseFromRequest(executor.QueryRowContext(ctx, `
WITH updated AS (
    UPDATE llm_requests
    SET response_status = $3,
        content = $4,
        error_code = $5,
        error_message = $6,
        response_json = $7,
        updated_at = NOW(),
        completed_at = NOW()
    WHERE owner_id = $1
      AND id = $2
      AND response_status = 'running'
    RETURNING id, owner_id, session_id, response_status, content, error_code, error_message,
              response_json, created_at, updated_at, completed_at
)
SELECT updated.owner_id, updated.session_id, updated.id,
       updated.response_status, updated.content, updated.error_code, updated.error_message, updated.response_json,
       updated.created_at, updated.updated_at, updated.completed_at
FROM updated`,
		ownerID, requestID, input.Status, input.Content, input.ErrorCode, input.ErrorMessage, jsonArgument(input.ResponseJSON)))
	if errors.Is(err, sql.ErrNoRows) {
		return Response{}, ErrRequestAlreadyFinal
	}
	return response, err
}

func (r *SQLRepository) listRequestReferences(ctx context.Context, requestID int64) ([]RequestReference, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := executor.QueryContext(ctx, `
SELECT id, request_id, object_ref_id, ref_code, module, object_type, title, status, payload_json, created_at
FROM llm_request_references
WHERE request_id = $1
ORDER BY id ASC`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	references := make([]RequestReference, 0)
	for rows.Next() {
		reference, err := scanRequestReference(rows)
		if err != nil {
			return nil, err
		}
		references = append(references, reference)
	}
	return references, rows.Err()
}

func (r *SQLRepository) executor(ctx context.Context) (platformdb.Executor, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("llm database is required")
	}
	return platformdb.ExecutorFromContext(ctx, r.database), nil
}

func jsonArgument(value json.RawMessage) string {
	if len(value) == 0 {
		return "{}"
	}
	return string(value)
}

func tagsFromPayloadJSON(value json.RawMessage) []string {
	if len(value) == 0 {
		return []string{}
	}
	payload := struct {
		Tags []string `json:"tags"`
	}{}
	if err := json.Unmarshal(value, &payload); err != nil || payload.Tags == nil {
		return []string{}
	}
	return payload.Tags
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(row rowScanner) (Session, error) {
	var session Session
	err := row.Scan(
		&session.ID, &session.OwnerID, &session.ObjectRefID, &session.RefCode,
		pq.Array(&session.Tags), &session.Title, &session.Status, &session.CreatedAt, &session.UpdatedAt,
	)
	session.Tags = nonNilTags(session.Tags)
	return session, err
}

func scanResponseFromRequest(row rowScanner) (Response, error) {
	var response Response
	var completedAt sql.NullTime
	err := row.Scan(
		&response.OwnerID, &response.SessionID, &response.RequestID, &response.Status,
		&response.Content, &response.ErrorCode, &response.ErrorMessage, &response.ResponseJSON,
		&response.CreatedAt, &response.UpdatedAt, &completedAt,
	)
	if completedAt.Valid {
		response.CompletedAt = &completedAt.Time
	}
	return response, err
}

func scanRequestReference(row rowScanner) (RequestReference, error) {
	var reference RequestReference
	var objectRefID sql.NullInt64
	err := row.Scan(
		&reference.ID, &reference.RequestID, &objectRefID, &reference.RefCode, &reference.Module,
		&reference.ObjectType, &reference.Title, &reference.Status, &reference.PayloadJSON, &reference.CreatedAt,
	)
	if objectRefID.Valid {
		reference.ObjectRefID = objectRefID.Int64
	}
	reference.Tags = tagsFromPayloadJSON(reference.PayloadJSON)
	return reference, err
}

func scanRequestWithRef(row rowScanner) (Request, error) {
	var request Request
	var completedAt sql.NullTime
	err := row.Scan(
		&request.ID, &request.OwnerID, &request.SessionID, &request.ActorUserID, &request.ObjectRefID,
		&request.RefCode, pq.Array(&request.Tags), &request.Prompt, &request.Model, &request.MaxTokens,
		&request.ContextJSON, &request.RequestJSON, &request.ResponseStatus,
		&request.ResponseContent, &request.ResponseErrorCode, &request.ResponseErrorMessage,
		&request.ResponseJSON, &request.CreatedAt, &request.UpdatedAt, &completedAt,
	)
	if completedAt.Valid {
		request.CompletedAt = &completedAt.Time
	}
	request.Tags = nonNilTags(request.Tags)
	return request, err
}

const sessionBaseSQL = `
SELECT s.id, s.owner_id, session_ref.id, session_ref.ref_code, session_ref.tags, s.title, s.status, s.created_at, s.updated_at
FROM llm_sessions AS s
JOIN object_refs AS session_ref
  ON session_ref.owner_id = s.owner_id
 AND session_ref.object_type = 'llm_session'
 AND session_ref.object_id = s.id
`

const requestBaseSQL = `
SELECT request.id, request.owner_id, request.session_id, request.actor_user_id, request_ref.id, request_ref.ref_code, request_ref.tags,
       request.prompt, request.model, request.max_tokens, request.context_json, request.request_json,
       request.response_status, request.content, request.error_code, request.error_message, request.response_json,
       request.created_at, request.updated_at, request.completed_at
FROM llm_requests AS request
JOIN object_refs AS request_ref
  ON request_ref.owner_id = request.owner_id
 AND request_ref.object_type = 'llm_request'
 AND request_ref.object_id = request.id
`

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
