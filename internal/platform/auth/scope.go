// This file defines authorization scopes without embedding business SQL.
package auth

type Scope struct {
	All     bool
	OwnerID int64
}

func ScopeForPrincipal(principal Principal) Scope {
	return Scope{All: !principal.IsZero()}
}
