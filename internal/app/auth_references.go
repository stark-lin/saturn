// This file adapts system-owned auth identities to the platform ObjectRef registry.
package app

import (
	"context"
	"fmt"

	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

type authIdentityReferenceRegistry struct {
	references *ref.Service
}

func (r authIdentityReferenceRegistry) ClaimIdentityCode(ctx context.Context, kind auth.IdentityReferenceKind) (string, error) {
	objectType, err := authIdentityObjectType(kind)
	if err != nil {
		return "", err
	}
	return r.references.ClaimCode(ctx, objectType)
}

func (r authIdentityReferenceRegistry) RegisterIdentity(ctx context.Context, registration auth.IdentityReferenceRegistration) error {
	objectType, err := authIdentityObjectType(registration.Kind)
	if err != nil {
		return err
	}
	_, err = r.references.Register(ctx, ref.Registration{
		OwnerID: ref.SystemOwnerID, RefCode: registration.RefCode,
		ObjectType: objectType, ObjectID: registration.ObjectID,
		Title: registration.Title, Status: registration.Status,
	})
	return err
}

func (r authIdentityReferenceRegistry) UpdateIdentityProjection(ctx context.Context, projection auth.IdentityReferenceProjection) error {
	objectType, err := authIdentityObjectType(projection.Kind)
	if err != nil {
		return err
	}
	_, err = r.references.UpdateProjection(ctx, ref.ProjectionUpdate{
		OwnerID: ref.SystemOwnerID, ObjectType: objectType, ObjectID: projection.ObjectID,
		Title: projection.Title, Status: projection.Status,
	})
	return err
}

func authIdentityObjectType(kind auth.IdentityReferenceKind) (ref.ObjectType, error) {
	switch kind {
	case auth.IdentityReferenceUser:
		return ref.ObjectTypeUser, nil
	case auth.IdentityReferenceAPIKey:
		return ref.ObjectTypeAPIKey, nil
	default:
		return "", fmt.Errorf("unsupported auth identity reference kind %q", kind)
	}
}
