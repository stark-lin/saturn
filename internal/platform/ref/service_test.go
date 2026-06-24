// This file tests object reference registration and authorized navigation resolution.
package ref

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stark-lin/saturn/internal/platform/auth"
)

func TestServiceRegistersAndResolvesCanonicalReference(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo)

	object, err := service.Register(context.Background(), Registration{
		OwnerID:    7,
		ObjectType: ObjectTypeNote,
		ObjectID:   18,
		Title:      " Release notes ",
		Tags:       []string{" release ", "", "go", "release"},
		Status:     "draft",
	})
	if err != nil {
		t.Fatalf("register object ref: %v", err)
	}
	if object.RefCode != "NTE-00000001" || object.Title != "Release notes" {
		t.Fatalf("registered object = %#v", object)
	}
	if len(object.Tags) != 2 || object.Tags[0] != "release" || object.Tags[1] != "go" {
		t.Fatalf("registered tags = %#v, want release and go", object.Tags)
	}

	resolved, err := service.Resolve(context.Background(), " nte-00000001 ")
	if err != nil {
		t.Fatalf("resolve normalized object ref: %v", err)
	}
	if resolved.RefCode != object.RefCode {
		t.Fatalf("resolved ref code = %q, want %q", resolved.RefCode, object.RefCode)
	}
}

func TestServiceResolveMetadataIsOwnerOnly(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo)
	file := mustRegister(t, service, Registration{
		OwnerID:    7,
		ObjectType: ObjectTypeFile,
		ObjectID:   2,
		Title:      "Receipt",
		Status:     "active",
	})
	note := mustRegister(t, service, Registration{
		OwnerID:    7,
		ObjectType: ObjectTypeNote,
		ObjectID:   3,
		Title:      "Draft note",
		Status:     "draft",
	})
	owner := auth.Principal{ID: 7, Role: auth.RoleUser}
	metadata, err := service.ResolveMetadata(context.Background(), owner, file.RefCode)
	if err != nil {
		t.Fatalf("owner resolve private metadata: %v", err)
	}
	if metadata.Module != ModuleFiles || metadata.ObjectType != ObjectTypeFile || metadata.Status != "active" {
		t.Fatalf("metadata = %#v, want files/file", metadata)
	}
	if len(metadata.Tags) != 0 {
		t.Fatalf("metadata tags = %#v, want empty tags", metadata.Tags)
	}
	reader := auth.Principal{ID: 8, Role: auth.RoleUser}
	if _, err := service.ResolveMetadata(context.Background(), reader, file.RefCode); !errors.Is(err, ErrNotFound) {
		t.Fatalf("file non-owner error = %v, want not found", err)
	}
	if _, err := service.ResolveMetadata(context.Background(), reader, note.RefCode); !errors.Is(err, ErrNotFound) {
		t.Fatalf("draft note non-owner error = %v, want not found", err)
	}
	superuser := auth.Principal{ID: 1, Role: auth.RoleSuperuser}
	if _, err := service.ResolveMetadata(context.Background(), superuser, file.RefCode); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-owner superuser error = %v, want not found", err)
	}
}

func TestServiceListRecentMetadataIsOwnerOnly(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo)
	mustRegister(t, service, Registration{
		OwnerID:    7,
		ObjectType: ObjectTypeNote,
		ObjectID:   3,
		Title:      "Owner note",
		Status:     "draft",
	})
	mustRegister(t, service, Registration{
		OwnerID:    8,
		ObjectType: ObjectTypeFile,
		ObjectID:   4,
		Title:      "Other owner's file",
		Status:     "active",
	})
	mustRegister(t, service, Registration{
		OwnerID:    7,
		ObjectType: ObjectTypeEventAggregate,
		ObjectID:   5,
		Title:      "Owner event aggregate",
		Status:     "active",
	})

	metadata, err := service.ListRecentMetadata(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, 10)
	if err != nil {
		t.Fatalf("list recent metadata: %v", err)
	}
	if repo.recentOwnerID != 7 || repo.recentLimit != 10 {
		t.Fatalf("recent query owner = %d, limit = %d", repo.recentOwnerID, repo.recentLimit)
	}
	if len(metadata) != 2 || metadata[0].Module != ModuleNotes || metadata[1].Module != ModuleCalendar {
		t.Fatalf("recent metadata = %#v, want only owner records in reference order", metadata)
	}
	if metadata[0].Tags == nil || metadata[1].Tags == nil {
		t.Fatalf("recent metadata tags must be non-nil: %#v", metadata)
	}
	superuserMetadata, err := service.ListRecentMetadata(context.Background(), auth.Principal{ID: 1, Role: auth.RoleSuperuser}, 10)
	if err != nil {
		t.Fatalf("list recent metadata as non-owner superuser: %v", err)
	}
	if len(superuserMetadata) != 0 {
		t.Fatalf("non-owner superuser metadata = %#v, want no owner-only records", superuserMetadata)
	}
}

func TestServiceListRecentMetadataRejectsInvalidLimit(t *testing.T) {
	service := NewService(newFakeRepository())

	if _, err := service.ListRecentMetadata(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, 0); !errors.Is(err, ErrInvalidRecentMetadataLimit) {
		t.Fatalf("invalid recent metadata limit error = %v", err)
	}
}

func TestServiceSearchMetadataNormalizesFiltersAndReturnsOwnerMetadata(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo)
	file := mustRegister(t, service, Registration{
		OwnerID:    7,
		ObjectType: ObjectTypeFile,
		ObjectID:   4,
		Title:      "Receipt",
		Tags:       []string{"backend", "release"},
		Status:     "active",
	})
	mustRegister(t, service, Registration{
		OwnerID:    7,
		ObjectType: ObjectTypeNote,
		ObjectID:   5,
		Title:      "Draft note",
		Tags:       []string{"backend"},
		Status:     "draft",
	})
	mustRegister(t, service, Registration{
		OwnerID:    8,
		ObjectType: ObjectTypeFile,
		ObjectID:   6,
		Title:      "Other owner file",
		Tags:       []string{"backend", "release"},
		Status:     "active",
	})

	metadata, err := service.SearchMetadata(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, MetadataSearchQuery{
		Modules:     []Module{ModuleFiles},
		ObjectTypes: []ObjectType{ObjectTypeFile, ObjectTypeNote},
		Statuses:    []string{"active", "active"},
		Tags:        []string{"backend", "release", "backend"},
		Sort: MetadataSearchSort{
			Field:     MetadataSearchSortRefCode,
			Direction: MetadataSearchSortAscending,
		},
		Limit: 25,
	})
	if err != nil {
		t.Fatalf("search metadata: %v", err)
	}
	if len(metadata) != 1 || metadata[0].RefCode != file.RefCode || metadata[0].Module != ModuleFiles {
		t.Fatalf("search metadata = %#v, want owner file metadata", metadata)
	}
	if repo.searchOwnerID != 7 || repo.searchCalls != 1 {
		t.Fatalf("search owner = %d calls = %d", repo.searchOwnerID, repo.searchCalls)
	}
	if len(repo.searchQuery.ObjectTypes) != 1 || repo.searchQuery.ObjectTypes[0] != ObjectTypeFile {
		t.Fatalf("normalized object types = %#v, want file only", repo.searchQuery.ObjectTypes)
	}
	if len(repo.searchQuery.Statuses) != 1 || repo.searchQuery.Statuses[0] != "active" {
		t.Fatalf("normalized statuses = %#v", repo.searchQuery.Statuses)
	}
	if len(repo.searchQuery.Tags) != 2 || repo.searchQuery.Tags[0] != "backend" || repo.searchQuery.Tags[1] != "release" {
		t.Fatalf("normalized tags = %#v", repo.searchQuery.Tags)
	}
	if repo.searchQuery.Sort.Field != MetadataSearchSortRefCode || repo.searchQuery.Sort.Direction != MetadataSearchSortAscending || repo.searchQuery.Limit != 25 {
		t.Fatalf("normalized sort/limit = %#v limit %d", repo.searchQuery.Sort, repo.searchQuery.Limit)
	}
}

func TestServiceSearchMetadataSkipsRepositoryWhenModuleObjectTypeIntersectionIsEmpty(t *testing.T) {
	repo := newFakeRepository()
	service := NewService(repo)

	metadata, err := service.SearchMetadata(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, MetadataSearchQuery{
		Modules:     []Module{ModuleFiles},
		ObjectTypes: []ObjectType{ObjectTypeNote},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("search empty intersection: %v", err)
	}
	if len(metadata) != 0 || repo.searchCalls != 0 {
		t.Fatalf("metadata = %#v search calls = %d, want empty without repo call", metadata, repo.searchCalls)
	}
}

func TestServiceSearchMetadataRejectsInvalidQuery(t *testing.T) {
	service := NewService(newFakeRepository())
	now := time.Now().UTC()
	earlier := now.Add(-time.Hour)

	for _, query := range []MetadataSearchQuery{
		{Modules: []Module{"unknown"}, Limit: 10},
		{ObjectTypes: []ObjectType{"unknown"}, Limit: 10},
		{Statuses: []string{""}, Limit: 10},
		{Tags: []string{" "}, Limit: 10},
		{Sort: MetadataSearchSort{Field: "title"}, Limit: 10},
		{Limit: MaxMetadataSearchLimit + 1},
		{CreatedFrom: &now, CreatedTo: &earlier, Limit: 10},
	} {
		if _, err := service.SearchMetadata(context.Background(), auth.Principal{ID: 7, Role: auth.RoleUser}, query); !errors.Is(err, ErrInvalidMetadataSearchQuery) {
			t.Fatalf("invalid metadata search query %#v error = %v, want invalid query", query, err)
		}
	}
}

func mustRegister(t *testing.T, service *Service, registration Registration) ObjectRef {
	t.Helper()
	object, err := service.Register(context.Background(), registration)
	if err != nil {
		t.Fatalf("register object: %v", err)
	}
	return object
}

type fakeRepository struct {
	sequence      int64
	objects       map[string]ObjectRef
	recentOwnerID int64
	recentLimit   int
	searchCalls   int
	searchOwnerID int64
	searchQuery   MetadataSearchQuery
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{objects: make(map[string]ObjectRef)}
}

func (r *fakeRepository) NextSequence(_ context.Context) (int64, error) {
	r.sequence++
	return r.sequence, nil
}

func (r *fakeRepository) Register(_ context.Context, object ObjectRef) (ObjectRef, error) {
	object.ID = int64(len(r.objects) + 1)
	r.objects[object.RefCode] = object
	return object, nil
}

func (r *fakeRepository) FindByCode(_ context.Context, code string) (ObjectRef, error) {
	object, ok := r.objects[code]
	if !ok {
		return ObjectRef{}, ErrNotFound
	}
	return object, nil
}

func (r *fakeRepository) ListRecentByOwner(_ context.Context, ownerID int64, limit int) ([]ObjectRef, error) {
	r.recentOwnerID = ownerID
	r.recentLimit = limit
	objects := make([]ObjectRef, 0, len(r.objects))
	for _, object := range r.objects {
		if object.OwnerID == ownerID {
			objects = append(objects, object)
		}
	}
	sort.Slice(objects, func(i, j int) bool {
		if objects[i].UpdatedAt.Equal(objects[j].UpdatedAt) {
			return objects[i].RefCode > objects[j].RefCode
		}
		return objects[i].UpdatedAt.After(objects[j].UpdatedAt)
	})
	if len(objects) > limit {
		objects = objects[:limit]
	}
	return objects, nil
}

func (r *fakeRepository) SearchByOwner(_ context.Context, ownerID int64, query MetadataSearchQuery) ([]ObjectRef, error) {
	r.searchCalls++
	r.searchOwnerID = ownerID
	r.searchQuery = query
	objects := make([]ObjectRef, 0, len(r.objects))
	for _, object := range r.objects {
		if object.OwnerID != ownerID || !fakeObjectMatchesQuery(object, query) {
			continue
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool {
		switch query.Sort.Field {
		case MetadataSearchSortRefCode:
			if query.Sort.Direction == MetadataSearchSortAscending {
				return objects[i].RefCode < objects[j].RefCode
			}
			return objects[i].RefCode > objects[j].RefCode
		default:
			if query.Sort.Direction == MetadataSearchSortAscending {
				return objects[i].UpdatedAt.Before(objects[j].UpdatedAt)
			}
			return objects[i].UpdatedAt.After(objects[j].UpdatedAt)
		}
	})
	if len(objects) > query.Limit {
		objects = objects[:query.Limit]
	}
	return objects, nil
}

func (r *fakeRepository) UpdateProjection(_ context.Context, update ProjectionUpdate) (ObjectRef, error) {
	for code, object := range r.objects {
		if object.OwnerID == update.OwnerID && object.ObjectType == update.ObjectType && object.ObjectID == update.ObjectID {
			object.Title = update.Title
			object.Tags = update.Tags
			object.Status = update.Status
			r.objects[code] = object
			return object, nil
		}
	}
	return ObjectRef{}, ErrNotFound
}

func (r *fakeRepository) Delete(_ context.Context, ownerID int64, objectType ObjectType, objectID int64) error {
	for code, object := range r.objects {
		if object.OwnerID == ownerID && object.ObjectType == objectType && object.ObjectID == objectID {
			delete(r.objects, code)
			return nil
		}
	}
	return ErrNotFound
}

func fakeObjectMatchesQuery(object ObjectRef, query MetadataSearchQuery) bool {
	if len(query.ObjectTypes) > 0 && !fakeContainsObjectType(query.ObjectTypes, object.ObjectType) {
		return false
	}
	if len(query.Statuses) > 0 && !fakeContainsString(query.Statuses, object.Status) {
		return false
	}
	for _, tag := range query.Tags {
		if !fakeContainsString(object.Tags, tag) {
			return false
		}
	}
	return true
}

func fakeContainsObjectType(values []ObjectType, target ObjectType) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func fakeContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
