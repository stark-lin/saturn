// This file defines authenticated Saturn principals.
package auth

type Role string

const (
	RoleSuperuser Role = "superuser"
	RoleUser      Role = "user"
)

type Principal struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     Role   `json:"role"`
}

func (p Principal) IsZero() bool {
	return p.ID == 0
}

func (p Principal) IsSuperuser() bool {
	return p.Role == RoleSuperuser
}
