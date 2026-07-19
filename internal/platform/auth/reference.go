// This file defines the auth service boundary for system-owned identity references.
package auth

import "context"

type IdentityReferenceKind string

const (
	IdentityReferenceUser   IdentityReferenceKind = "user"
	IdentityReferenceAPIKey IdentityReferenceKind = "api_key"
)

type IdentityReferenceRegistration struct {
	RefCode  string
	Kind     IdentityReferenceKind
	ObjectID int64
	Title    string
	Status   string
}

type IdentityReferenceProjection struct {
	Kind     IdentityReferenceKind
	ObjectID int64
	Title    string
	Status   string
}

type IdentityReferenceRegistry interface {
	ClaimIdentityCode(ctx context.Context, kind IdentityReferenceKind) (string, error)
	RegisterIdentity(ctx context.Context, registration IdentityReferenceRegistration) error
	UpdateIdentityProjection(ctx context.Context, projection IdentityReferenceProjection) error
}
