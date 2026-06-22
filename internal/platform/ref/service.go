// This file registers and resolves Saturn object reference codes.
package ref

import (
	"context"
	"errors"
	"strings"

	"github.com/stark-lin/go-proj/internal/platform/auth"
)

var (
	ErrInvalidObjectRef           = errors.New("invalid object ref")
	ErrInvalidRecentMetadataLimit = errors.New("invalid recent metadata limit")
	ErrInvalidMetadataSearchQuery = errors.New("invalid metadata search query")
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, registration Registration) (ObjectRef, error) {
	if err := validateProjection(registration.OwnerID, registration.ObjectType, registration.ObjectID, registration.Title, registration.Status); err != nil {
		return ObjectRef{}, err
	}
	code := NormalizeCode(registration.RefCode)
	if code == "" {
		var err error
		code, err = s.ClaimCode(ctx, registration.ObjectType)
		if err != nil {
			return ObjectRef{}, err
		}
	}
	if !CodeMatchesObjectType(code, registration.ObjectType) {
		return ObjectRef{}, ErrInvalidObjectRef
	}
	return s.repo.Register(ctx, ObjectRef{
		OwnerID:    registration.OwnerID,
		RefCode:    code,
		ObjectType: registration.ObjectType,
		ObjectID:   registration.ObjectID,
		Title:      strings.TrimSpace(registration.Title),
		Tags:       normalizeTags(registration.Tags),
		Status:     registration.Status,
	})
}

func (s *Service) ClaimCode(ctx context.Context, objectType ObjectType) (string, error) {
	if _, ok := objectDefinitions[objectType]; !ok {
		return "", ErrUnsupportedObjectType
	}
	sequence, err := s.repo.NextSequence(ctx)
	if err != nil {
		return "", err
	}
	return FormatCode(objectType, sequence)
}

func (s *Service) Resolve(ctx context.Context, code string) (ObjectRef, error) {
	resolver := NewResolver(s.repo)
	return resolver.Resolve(ctx, code)
}

func (s *Service) UpdateProjection(ctx context.Context, update ProjectionUpdate) (ObjectRef, error) {
	if err := validateProjection(update.OwnerID, update.ObjectType, update.ObjectID, update.Title, update.Status); err != nil {
		return ObjectRef{}, err
	}
	update.Tags = normalizeTags(update.Tags)
	return s.repo.UpdateProjection(ctx, update)
}

func (s *Service) Delete(ctx context.Context, ownerID int64, objectType ObjectType, objectID int64) error {
	if ownerID < 1 || objectID < 1 {
		return ErrInvalidObjectRef
	}
	if _, ok := objectDefinitions[objectType]; !ok {
		return ErrUnsupportedObjectType
	}
	return s.repo.Delete(ctx, ownerID, objectType, objectID)
}

func (s *Service) ResolveMetadata(ctx context.Context, actor auth.Principal, code string) (Metadata, error) {
	if actor.IsZero() {
		return Metadata{}, auth.ErrUnauthenticated
	}
	object, err := s.Resolve(ctx, code)
	if err != nil {
		return Metadata{}, err
	}
	if !CodeMatchesObjectType(object.RefCode, object.ObjectType) {
		return Metadata{}, ErrInvalidObjectRef
	}
	if actor.ID != object.OwnerID {
		return Metadata{}, ErrNotFound
	}
	return metadataFromObjectRef(object)
}

func (s *Service) ListRecentMetadata(ctx context.Context, actor auth.Principal, limit int) ([]Metadata, error) {
	if actor.IsZero() {
		return nil, auth.ErrUnauthenticated
	}
	if limit < 1 || limit > MaxRecentMetadataLimit {
		return nil, ErrInvalidRecentMetadataLimit
	}
	objects, err := s.repo.ListRecentByOwner(ctx, actor.ID, limit)
	if err != nil {
		return nil, err
	}
	metadata := make([]Metadata, 0, len(objects))
	for _, object := range objects {
		next, err := metadataFromObjectRef(object)
		if err != nil {
			return nil, err
		}
		metadata = append(metadata, next)
	}
	return metadata, nil
}

func (s *Service) SearchMetadata(ctx context.Context, actor auth.Principal, query MetadataSearchQuery) ([]Metadata, error) {
	if actor.IsZero() {
		return nil, auth.ErrUnauthenticated
	}
	query, emptyResult, err := normalizeMetadataSearchQuery(query)
	if err != nil {
		return nil, err
	}
	if emptyResult {
		return []Metadata{}, nil
	}
	objects, err := s.repo.SearchByOwner(ctx, actor.ID, query)
	if err != nil {
		return nil, err
	}
	metadata := make([]Metadata, 0, len(objects))
	for _, object := range objects {
		next, err := metadataFromObjectRef(object)
		if err != nil {
			return nil, err
		}
		metadata = append(metadata, next)
	}
	return metadata, nil
}

func validateProjection(ownerID int64, objectType ObjectType, objectID int64, title string, status string) error {
	if ownerID < 1 || objectID < 1 || strings.TrimSpace(title) == "" || strings.TrimSpace(status) == "" {
		return ErrInvalidObjectRef
	}
	if _, ok := objectDefinitions[objectType]; !ok {
		return ErrUnsupportedObjectType
	}
	return nil
}

func normalizeTags(names []string) []string {
	tags := make([]string, 0, len(names))
	seen := make(map[string]struct{})
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, name)
	}
	return tags
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func metadataFromObjectRef(object ObjectRef) (Metadata, error) {
	if !CodeMatchesObjectType(object.RefCode, object.ObjectType) {
		return Metadata{}, ErrInvalidObjectRef
	}
	module, err := ModuleForObjectType(object.ObjectType)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{
		RefCode:    object.RefCode,
		Module:     module,
		ObjectType: object.ObjectType,
		Title:      object.Title,
		Tags:       nonNilTags(object.Tags),
		Status:     object.Status,
		CreatedAt:  object.CreatedAt,
		UpdatedAt:  object.UpdatedAt,
	}, nil
}
