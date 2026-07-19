// This file evaluates resource-level authorization decisions.
package auth

import "errors"

var (
	ErrUnauthenticated = errors.New("principal is required")
	ErrForbidden       = errors.New("action is forbidden")
)

type Authorizer struct{}

func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

func (a *Authorizer) Can(principal Principal, action Action, resource Resource) error {
	if principal.IsZero() {
		return ErrUnauthenticated
	}
	requiredScope := ScopeDataRead
	if action != ActionRead {
		requiredScope = ScopeDataWrite
	}
	if principal.Allows(requiredScope) {
		return nil
	}
	return ErrForbidden
}
