// This file signs and verifies JWT bearer tokens for authenticated requests.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid authentication token")

type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

type IssuedToken struct {
	Value     string
	ID        string
	ExpiresAt time.Time
}

type TokenClaims struct {
	UserID    int64
	Username  string
	Role      Role
	TokenID   string
	ExpiresAt time.Time
}

type jwtPayload struct {
	Subject   string `json:"sub"`
	Username  string `json:"username"`
	Role      Role   `json:"role"`
	TokenID   string `json:"jti"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("jwt secret is required")
	}
	if ttl <= 0 {
		return nil, errors.New("jwt ttl must be positive")
	}
	return &TokenManager{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

func (m *TokenManager) Issue(principal Principal) (IssuedToken, error) {
	if m == nil || principal.IsZero() || strings.TrimSpace(principal.Username) == "" {
		return IssuedToken{}, ErrInvalidToken
	}
	if principal.Role != RoleSuperuser && principal.Role != RoleUser {
		return IssuedToken{}, ErrInvalidToken
	}

	tokenID, err := newTokenID()
	if err != nil {
		return IssuedToken{}, err
	}
	now := m.now().UTC()
	expiresAt := now.Add(m.ttl)
	payload := jwtPayload{
		Subject:   strconv.FormatInt(principal.ID, 10),
		Username:  principal.Username,
		Role:      principal.Role,
		TokenID:   tokenID,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
	}
	token, err := m.sign(payload)
	if err != nil {
		return IssuedToken{}, err
	}
	return IssuedToken{Value: token, ID: tokenID, ExpiresAt: expiresAt}, nil
}

func (m *TokenManager) Verify(token string) (TokenClaims, error) {
	if m == nil {
		return TokenClaims{}, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TokenClaims{}, ErrInvalidToken
	}

	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := decodeJWTPart(parts[0], &header); err != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return TokenClaims{}, ErrInvalidToken
	}
	expected := m.signature(parts[0] + "." + parts[1])
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(expected, provided) {
		return TokenClaims{}, ErrInvalidToken
	}

	var payload jwtPayload
	if err := decodeJWTPart(parts[1], &payload); err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	userID, err := strconv.ParseInt(payload.Subject, 10, 64)
	if err != nil || userID <= 0 || strings.TrimSpace(payload.TokenID) == "" || strings.TrimSpace(payload.Username) == "" {
		return TokenClaims{}, ErrInvalidToken
	}
	if payload.Role != RoleSuperuser && payload.Role != RoleUser {
		return TokenClaims{}, ErrInvalidToken
	}
	expiresAt := time.Unix(payload.ExpiresAt, 0).UTC()
	if !m.now().UTC().Before(expiresAt) {
		return TokenClaims{}, ErrInvalidToken
	}
	return TokenClaims{
		UserID:    userID,
		Username:  payload.Username,
		Role:      payload.Role,
		TokenID:   payload.TokenID,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *TokenManager) sign(payload jwtPayload) (string, error) {
	headerJSON, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	body := base64.RawURLEncoding.EncodeToString(payloadJSON)
	unsigned := header + "." + body
	signature := base64.RawURLEncoding.EncodeToString(m.signature(unsigned))
	return unsigned + "." + signature, nil
}

func (m *TokenManager) signature(unsigned string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(unsigned))
	return mac.Sum(nil)
}

func decodeJWTPart(encoded string, target any) error {
	content, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("decode jwt json: %w", err)
	}
	return nil
}

func newTokenID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
