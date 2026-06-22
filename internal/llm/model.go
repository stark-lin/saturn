// This file defines LLM domain models.
package llm

import (
	"encoding/json"
	"time"
)

type SessionStatus string

const (
	SessionStatusActive SessionStatus = "active"
)

type ResponseStatus string

const (
	ResponseStatusQueued  ResponseStatus = "queued"
	ResponseStatusRunning ResponseStatus = "running"
	ResponseStatusSuccess ResponseStatus = "success"
	ResponseStatusError   ResponseStatus = "error"
)

type Session struct {
	ID          int64
	OwnerID     int64
	ObjectRefID int64
	RefCode     string
	Title       string
	Status      SessionStatus
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Request struct {
	ID                   int64
	OwnerID              int64
	SessionID            int64
	ActorUserID          int64
	ObjectRefID          int64
	RefCode              string
	Prompt               string
	Model                string
	MaxTokens            int
	ContextJSON          json.RawMessage
	RequestJSON          json.RawMessage
	ResponseStatus       ResponseStatus
	ResponseContent      string
	ResponseErrorCode    string
	ResponseErrorMessage string
	ResponseJSON         json.RawMessage
	References           []RequestReference
	Tags                 []string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CompletedAt          *time.Time
}

type Response struct {
	OwnerID      int64
	SessionID    int64
	RequestID    int64
	Status       ResponseStatus
	Content      string
	ErrorCode    string
	ErrorMessage string
	ResponseJSON json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}

type RequestReference struct {
	ID          int64           `json:"-"`
	RequestID   int64           `json:"-"`
	ObjectRefID int64           `json:"-"`
	RefCode     string          `json:"ref_code"`
	Module      string          `json:"module"`
	ObjectType  string          `json:"object_type"`
	Title       string          `json:"title"`
	Status      string          `json:"status"`
	Tags        []string        `json:"tags"`
	PayloadJSON json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"-"`
}

type RequestDeletionTarget struct {
	ID      int64
	RefCode string
}

type SessionPage struct {
	Sessions []Session
	Limit    int
	Offset   int
	HasMore  bool
}

type SessionDetail struct {
	Session  Session
	Requests []Request
}

type CreateSessionInput struct {
	Title string
	Tags  []string
}

type CreateRequestInput struct {
	Prompt     string
	References []string
	Model      string
	MaxTokens  int
	Tags       []string
}

type PersistedRequestInput struct {
	ActorUserID int64
	Prompt      string
	Model       string
	MaxTokens   int
	ContextJSON json.RawMessage
	RequestJSON json.RawMessage
}

type CompleteResponseInput struct {
	Status       ResponseStatus
	Content      string
	ErrorCode    string
	ErrorMessage string
	ResponseJSON json.RawMessage
}

type ClientResult struct {
	Content string
	RawJSON json.RawMessage
}

type ResolvedReference struct {
	ObjectRefID int64           `json:"-"`
	RefCode     string          `json:"ref_code"`
	Module      string          `json:"module"`
	ObjectType  string          `json:"object_type"`
	Title       string          `json:"title"`
	Status      string          `json:"status"`
	Tags        []string        `json:"tags"`
	PayloadJSON json.RawMessage `json:"payload"`
}

const (
	DefaultLimit     = 25
	MaxLimit         = 100
	DefaultMaxTokens = 1024
)
