// This file defines append-only audit records captured by Saturn services.
package audit

import "time"

const SystemTargetRefCode = "SYS-00000000"

type Action string

const (
	ActionCreate Action = "CREATE"
	ActionRead   Action = "READ"
	ActionUpdate Action = "UPDATE"
	ActionDelete Action = "DELETE"
	ActionExport Action = "EXPORT"
	ActionLogin  Action = "LOGIN"
	ActionLogout Action = "LOGOUT"
)

type Result string

const (
	ResultSuccess Result = "SUCCESS"
	ResultFailed  Result = "FAILED"
	ResultDenied  Result = "DENIED"
)

type Event struct {
	ID            int64     `json:"id"`
	ActorRefCode  string    `json:"actor_ref_code"`
	Action        Action    `json:"action"`
	TargetRefCode string    `json:"target_ref_code"`
	Result        Result    `json:"result"`
	Reason        string    `json:"reason,omitempty"`
	SourceIP      string    `json:"source_ip"`
	UserAgent     string    `json:"user_agent,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type Query struct {
	TargetRefCode string
	ActorRefCode  string
	Action        Action
	Result        Result
	Limit         int
	Offset        int
}

const (
	DefaultLimit = 50
	MaxLimit     = 100
)
