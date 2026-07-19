// This file defines Saturn user records.
package auth

import (
	"regexp"
	"time"
)

var userRefCodePattern = regexp.MustCompile(`^USR-[0-9A-F]{8}$`)

func ValidUserRefCode(value string) bool {
	return userRefCodePattern.MatchString(value)
}

type User struct {
	ID           int64
	RefCode      string
	Email        string
	DisplayName  string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
