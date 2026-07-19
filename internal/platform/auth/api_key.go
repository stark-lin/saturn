// This file defines API key lifecycle records and public representations.
package auth

import (
	"regexp"
	"time"
)

const apiKeySecretPrefix = "sat_sk_"

var apiKeyRefCodePattern = regexp.MustCompile(`^KEY-[0-9A-F]{8}$`)

func ValidAPIKeyRefCode(value string) bool {
	return apiKeyRefCodePattern.MatchString(value)
}

type APIKeyStatus string

const (
	APIKeyStatusActive  APIKeyStatus = "ACTIVE"
	APIKeyStatusRevoked APIKeyStatus = "REVOKED"
	APIKeyStatusExpired APIKeyStatus = "EXPIRED"
)

type APIKey struct {
	ID         int64        `json:"-"`
	RefCode    string       `json:"refcode"`
	Name       string       `json:"name"`
	KeyPrefix  string       `json:"key_prefix"`
	KeyHash    string       `json:"-"`
	Scopes     []ScopeName  `json:"scopes"`
	Status     APIKeyStatus `json:"status"`
	CreatedAt  time.Time    `json:"created_at"`
	LastUsedAt *time.Time   `json:"last_used_at"`
	ExpiresAt  *time.Time   `json:"expires_at"`
	RevokedAt  *time.Time   `json:"revoked_at"`
}

func (k APIKey) EffectiveStatus(now time.Time) APIKeyStatus {
	if k.RevokedAt != nil {
		return APIKeyStatusRevoked
	}
	if k.ExpiresAt != nil && !now.Before(*k.ExpiresAt) {
		return APIKeyStatusExpired
	}
	return APIKeyStatusActive
}
