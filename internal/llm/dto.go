// This file defines LLM HTTP request and response payloads.
package llm

import "time"

type CreateSessionRequest struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

type CreateRequestRequest struct {
	Prompt     string   `json:"prompt"`
	References []string `json:"references"`
	Model      string   `json:"model"`
	MaxTokens  int      `json:"max_tokens"`
	Tags       []string `json:"tags"`
}

type SessionResponse struct {
	Session SessionView `json:"session"`
}

type SessionsResponse struct {
	Sessions []SessionView `json:"sessions"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	HasMore  bool          `json:"has_more"`
}

type SessionDetailResponse struct {
	Session  SessionView   `json:"session"`
	Requests []RequestView `json:"requests"`
}

type RequestResponse struct {
	Request RequestView `json:"request"`
}

type SessionView struct {
	RefCode   string        `json:"ref_code"`
	Title     string        `json:"title"`
	Status    SessionStatus `json:"status"`
	Tags      []string      `json:"tags"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type RequestView struct {
	RefCode              string                 `json:"ref_code"`
	Prompt               string                 `json:"prompt"`
	Model                string                 `json:"model"`
	MaxTokens            int                    `json:"max_tokens"`
	References           []RequestReferenceView `json:"references"`
	ResponseStatus       ResponseStatus         `json:"response_status"`
	ResponseContent      string                 `json:"content"`
	ResponseErrorCode    string                 `json:"error_code,omitempty"`
	ResponseErrorMessage string                 `json:"error_message,omitempty"`
	Tags                 []string               `json:"tags"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	CompletedAt          *time.Time             `json:"completed_at,omitempty"`
}

type RequestReferenceView struct {
	RefCode    string   `json:"ref_code"`
	Module     string   `json:"module"`
	ObjectType string   `json:"object_type"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Tags       []string `json:"tags"`
}
