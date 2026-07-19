// This file defines Saturn user records.
package auth

import "time"

type User struct {
	ID           int64
	RefCode      string
	Username     string
	Email        string
	DisplayName  string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
