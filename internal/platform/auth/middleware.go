// This file provides route-level authentication middleware.
package auth

import (
	"net/http"
	"strings"

	"github.com/stark-lin/saturn/internal/platform/httpx"
)

func AuthenticateBearer(service *Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := BearerToken(r)
		if err != nil || service == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
			return
		}
		principal, err := service.Authenticate(r.Context(), token)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
			return
		}
		next.ServeHTTP(w, r.WithContext(ContextWithPrincipal(r.Context(), principal)))
	})
}

func RequireScope(scope ScopeName, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
			return
		}
		if !principal.Allows(scope) {
			httpx.WriteError(w, http.StatusForbidden, "insufficient_scope", "The credential does not allow this operation")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func BearerToken(r *http.Request) (string, error) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", ErrUnauthenticated
	}
	return parts[1], nil
}
