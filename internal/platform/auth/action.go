// This file defines authorization actions used by services.
package auth

type Action string

const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionAdmin  Action = "admin"
)
