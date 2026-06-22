// This file defines authorization scopes without embedding business SQL.
package auth

type Scope struct {
	All           bool
	OwnerID       int64
	IncludeShared bool
}

func ScopeForPrincipal(principal Principal) Scope {
	if principal.IsSuperuser() {
		return Scope{All: true}
	}
	return Scope{OwnerID: principal.ID, IncludeShared: true}
}
