// This file defines authenticated Saturn principals.
package auth

import "slices"

// AdministratorRefCode is the clean-schema administrator's first global claim.
// Runtime authorization accepts any valid USR RefCode loaded from the registry.
const AdministratorRefCode = "USR-00000001"

type PrincipalKind string

const (
	PrincipalKindAdministrator PrincipalKind = "administrator"
	PrincipalKindAPIKey        PrincipalKind = "api_key"
)

type ScopeName string

const (
	ScopeDataRead  ScopeName = "data:read"
	ScopeDataWrite ScopeName = "data:write"
)

var SupportedAPIKeyScopes = []ScopeName{ScopeDataRead, ScopeDataWrite}

type Principal struct {
	ID      int64         `json:"-"`
	RefCode string        `json:"refcode"`
	Kind    PrincipalKind `json:"kind"`
	Email   string        `json:"email,omitempty"`
	Name    string        `json:"name,omitempty"`
	Scopes  []ScopeName   `json:"scopes,omitempty"`
}

func (p Principal) IsZero() bool {
	return p.ID == 0 || p.RefCode == "" || (p.Kind != PrincipalKindAdministrator && p.Kind != PrincipalKindAPIKey)
}

func (p Principal) ActorRefCode() string {
	return p.RefCode
}

func (p Principal) IsAdministrator() bool {
	return p.Kind == PrincipalKindAdministrator
}

func (p Principal) Allows(scope ScopeName) bool {
	if p.Kind != PrincipalKindAPIKey {
		return true
	}
	if scope == ScopeDataRead && slices.Contains(p.Scopes, ScopeDataWrite) {
		return true
	}
	return slices.Contains(p.Scopes, scope)
}
