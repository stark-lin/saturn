// This file defines Saturn user records.
package auth

import "time"

type User struct {
	ID           int64
	Username     string
	Email        string
	DisplayName  string
	Role         Role
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
