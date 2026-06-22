// This file tests immutable Files service invariants.
package files

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/stark-lin/go-proj/internal/platform/audit"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/ref"
	"github.com/stark-lin/go-proj/internal/platform/storage"
	"lukechampine.com/blake3"
)

func TestNewModuleBuildsFilesDependencies(t *testing.T) {
	module := NewModule(nil, nil, nil, nil, nil)
	if module.Service == nil {
		t.Fatal("expected service")
	}
	if module.Handler == nil {
		t.Fatal("expected handler")
	}
}

func TestServiceCreatesCollectionAndFileWithHashes(t *testing.T) {
	service, repo, references, _, storage := newTestService()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}

	collection, err := service.CreateCollection(context.Background(), actor, CreateCollectionInput{
		Name: "Receipts", Description: "Tax year", Tags: []string{" tax ", "", "tax", "receipt"},
	})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if collection.RefCode != "FIL-00000001" || collection.Status != CollectionStatusActive {
		t.Fatalf("collection = %#v", collection)
	}
	if len(collection.Tags) != 2 || collection.Tags[0] != "tax" || collection.Tags[1] != "receipt" {
		t.Fatalf("collection tags = %#v", collection.Tags)
	}

	file, err := service.CreateFile(context.Background(), actor, CreateFileInput{
		CollectionRefCode: collection.RefCode,
		OriginalName:      "receipt.txt",
		MimeType:          "text/plain",
		SizeBytes:         int64(len("hello")),
		Body:              bytes.NewBufferString("hello"),
		Tags:              []string{" receipt ", "pdf", "receipt"},
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if file.RefCode != "FIL-00000002" || file.CollectionRefCode != collection.RefCode ||
		file.ObjectKey != "FIL-00000002/blob" {
		t.Fatalf("file = %#v", file)
	}
	if file.SHA256 != sha256Hex("hello") || file.BLAKE3 != blake3Hex("hello") {
		t.Fatalf("hashes = sha256 %q blake3 %q", file.SHA256, file.BLAKE3)
	}
	if len(file.Tags) != 2 || file.Tags[0] != "receipt" || file.Tags[1] != "pdf" {
		t.Fatalf("file tags = %#v", file.Tags)
	}
	detail := fileDetail(file)
	if detail.SHA256 == "" || detail.BLAKE3 == "" || detail.Metadata.SHA256 != detail.SHA256 || detail.Metadata.BLAKE3 != detail.BLAKE3 {
		t.Fatalf("file API detail does not expose hashes: %#v", detail)
	}
	if len(detail.Tags) != 2 || detail.Tags[0] != "receipt" || detail.Tags[1] != "pdf" {
		t.Fatalf("file API detail tags = %#v", detail.Tags)
	}
	if string(storage.objects[file.ObjectKey]) != "hello" {
		t.Fatalf("stored object = %q", storage.objects[file.ObjectKey])
	}
	if references.registrations[0].ObjectType != ref.ObjectTypeFileCollection ||
		references.registrations[1].ObjectType != ref.ObjectTypeFile {
		t.Fatalf("reference registrations = %#v", references.registrations)
	}
	if repo.lockedCollectionRef != collection.RefCode {
		t.Fatalf("locked collection = %q", repo.lockedCollectionRef)
	}
	if len(references.registrations[0].Tags) != 2 ||
		references.registrations[0].Tags[0] != "tax" ||
		len(references.registrations[1].Tags) != 2 ||
		references.registrations[1].Tags[1] != "pdf" {
		t.Fatalf("registration tags = %#v", references.registrations)
	}
}

func TestServiceAuditsSuccessfulDownload(t *testing.T) {
	service, _, _, audits, _ := newTestService()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}
	collection, err := service.CreateCollection(context.Background(), actor, CreateCollectionInput{Name: "Receipts"})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	file, err := service.CreateFile(context.Background(), actor, CreateFileInput{
		CollectionRefCode: collection.RefCode, OriginalName: "receipt.txt", SizeBytes: 5,
		Body: bytes.NewBufferString("hello"),
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	download, err := service.OpenVerifiedDownload(context.Background(), actor, file.RefCode)
	if err != nil {
		t.Fatalf("open download: %v", err)
	}
	defer download.Body.Close()
	content, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("download body = %q", content)
	}
	if len(audits.standalones) != 1 ||
		audits.standalones[0].Action != audit.ActionExport ||
		audits.standalones[0].Result != audit.ResultSuccess ||
		audits.standalones[0].TargetRefCode != file.RefCode ||
		audits.standalones[0].Reason != "download" {
		t.Fatalf("audit standalones = %#v", audits.standalones)
	}
}

func TestServiceRejectsDownloadWhenStoredBlobHashDiffers(t *testing.T) {
	service, _, _, audits, storage := newTestService()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}
	collection, err := service.CreateCollection(context.Background(), actor, CreateCollectionInput{Name: "Receipts"})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	file, err := service.CreateFile(context.Background(), actor, CreateFileInput{
		CollectionRefCode: collection.RefCode, OriginalName: "receipt.txt", SizeBytes: 5,
		Body: bytes.NewBufferString("hello"),
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	storage.objects[file.ObjectKey] = []byte("HELLO")

	if _, err := service.OpenVerifiedDownload(context.Background(), actor, file.RefCode); !errors.Is(err, ErrIntegrityCheckFailed) {
		t.Fatalf("download error = %v, want integrity failure", err)
	}
	if len(audits.standalones) != 1 ||
		audits.standalones[0].Action != audit.ActionExport ||
		audits.standalones[0].Result != audit.ResultFailed ||
		audits.standalones[0].TargetRefCode != file.RefCode ||
		audits.standalones[0].Reason != "integrity_check_failed" {
		t.Fatalf("audit standalones = %#v", audits.standalones)
	}
}

func TestServiceRejectsDownloadWhenEitherStoredHashDiffers(t *testing.T) {
	tests := []struct {
		name       string
		corruptOne func(*File)
	}{
		{
			name: "sha256",
			corruptOne: func(file *File) {
				file.SHA256 = strings.Repeat("0", 64)
			},
		},
		{
			name: "blake3",
			corruptOne: func(file *File) {
				file.BLAKE3 = strings.Repeat("0", 64)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repo, _, audits, _ := newTestService()
			actor := auth.Principal{ID: 7, Role: auth.RoleUser}
			collection, err := service.CreateCollection(context.Background(), actor, CreateCollectionInput{Name: "Receipts"})
			if err != nil {
				t.Fatalf("create collection: %v", err)
			}
			file, err := service.CreateFile(context.Background(), actor, CreateFileInput{
				CollectionRefCode: collection.RefCode, OriginalName: "receipt.txt", SizeBytes: 5,
				Body: bytes.NewBufferString("hello"),
			})
			if err != nil {
				t.Fatalf("create file: %v", err)
			}
			stored := repo.files[file.ID]
			tt.corruptOne(&stored)
			repo.files[file.ID] = stored

			if _, err := service.OpenVerifiedDownload(context.Background(), actor, file.RefCode); !errors.Is(err, ErrIntegrityCheckFailed) {
				t.Fatalf("download error = %v, want integrity failure", err)
			}
			if len(audits.standalones) != 1 || audits.standalones[0].Reason != "integrity_check_failed" {
				t.Fatalf("audit standalones = %#v", audits.standalones)
			}
		})
	}
}

func TestServiceDeletesCollectionThroughUnifiedFileDeleteReason(t *testing.T) {
	service, repo, references, audits, storage := newTestService()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}
	collection, err := service.CreateCollection(context.Background(), actor, CreateCollectionInput{Name: "Receipts"})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	first, err := service.CreateFile(context.Background(), actor, CreateFileInput{
		CollectionRefCode: collection.RefCode, OriginalName: "a.txt", SizeBytes: 1, Body: bytes.NewBufferString("a"),
	})
	if err != nil {
		t.Fatalf("create first file: %v", err)
	}
	second, err := service.CreateFile(context.Background(), actor, CreateFileInput{
		CollectionRefCode: collection.RefCode, OriginalName: "b.txt", SizeBytes: 1, Body: bytes.NewBufferString("b"),
	})
	if err != nil {
		t.Fatalf("create second file: %v", err)
	}

	if err := service.DeleteCollection(context.Background(), actor, collection.RefCode); err != nil {
		t.Fatalf("delete collection: %v", err)
	}
	if len(repo.collections) != 0 || len(repo.files) != 0 {
		t.Fatalf("repo after delete: collections %#v files %#v", repo.collections, repo.files)
	}
	if !storage.deleted[first.ObjectKey] || !storage.deleted[second.ObjectKey] {
		t.Fatalf("deleted objects = %#v", storage.deleted)
	}
	if countAuditReason(audits.successes, string(DeleteReasonCascadeCollectionDelete)) != 2 ||
		countAuditReason(audits.successes, string(DeleteReasonCollectionDelete)) != 1 {
		t.Fatalf("audit successes = %#v", audits.successes)
	}
	if len(references.deletes) != 3 ||
		references.deletes[0].objectType != ref.ObjectTypeFile ||
		references.deletes[2].objectType != ref.ObjectTypeFileCollection {
		t.Fatalf("reference deletes = %#v", references.deletes)
	}
}

func TestServiceDeletesSingleFileWithDirectReason(t *testing.T) {
	service, repo, _, audits, storage := newTestService()
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}
	collection, err := service.CreateCollection(context.Background(), actor, CreateCollectionInput{Name: "Receipts"})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	file, err := service.CreateFile(context.Background(), actor, CreateFileInput{
		CollectionRefCode: collection.RefCode, OriginalName: "a.txt", SizeBytes: 1, Body: bytes.NewBufferString("a"),
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := service.DeleteFile(context.Background(), actor, file.RefCode); err != nil {
		t.Fatalf("delete file: %v", err)
	}
	if _, exists := repo.files[file.ID]; exists {
		t.Fatalf("file still exists: %#v", repo.files[file.ID])
	}
	if !storage.deleted[file.ObjectKey] || countAuditReason(audits.successes, string(DeleteReasonDirectFileDelete)) != 1 {
		t.Fatalf("storage deleted = %#v audits = %#v", storage.deleted, audits.successes)
	}
}

func TestBindFileQueryAcceptsTagFilter(t *testing.T) {
	query, err := bindFileQuery(url.Values{
		"collection_ref_code": {" FIL-00000001 "},
		"tag":                 {" receipt "},
		"limit":               {"10"},
	})
	if err != nil {
		t.Fatalf("bind query: %v", err)
	}
	if query.CollectionRefCode != "FIL-00000001" || query.Tag != "receipt" || query.Limit != 10 {
		t.Fatalf("query = %#v", query)
	}
}

func TestMultipartTagsSplitsCommaSeparatedValues(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("tags", " receipt, tax "); err != nil {
		t.Fatalf("write tags: %v", err)
	}
	if err := writer.WriteField("tags", "manual"); err != nil {
		t.Fatalf("write tags: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1024); err != nil {
		t.Fatalf("parse multipart: %v", err)
	}
	tags := normalizedTags(multipartTags(request, "tags"))
	if len(tags) != 3 || tags[0] != "receipt" || tags[1] != "tax" || tags[2] != "manual" {
		t.Fatalf("tags = %#v", tags)
	}
}

func newTestService() (*Service, *fakeRepository, *fakeReferences, *fakeAudits, *fakeStorage) {
	repo := &fakeRepository{collections: make(map[int64]Collection), files: make(map[int64]File)}
	references := &fakeReferences{repo: repo}
	audits := &fakeAudits{}
	storage := &fakeStorage{objects: make(map[string][]byte), deleted: make(map[string]bool)}
	return NewService(repo, nil, references, audits, storage), repo, references, audits, storage
}

type fakeRepository struct {
	nextID              int64
	collections         map[int64]Collection
	files               map[int64]File
	lockedCollectionRef string
}

func (r *fakeRepository) ListCollections(_ context.Context, _ auth.Scope, query CollectionQuery) (CollectionPage, error) {
	collections := make([]Collection, 0, len(r.collections))
	for _, collection := range r.collections {
		collections = append(collections, collection)
	}
	sort.Slice(collections, func(left int, right int) bool {
		return collections[left].ID < collections[right].ID
	})
	return CollectionPage{Collections: collections, Limit: query.Limit, Offset: query.Offset}, nil
}

func (r *fakeRepository) CreateCollection(_ context.Context, ownerID int64, input CreateCollectionInput) (Collection, error) {
	r.nextID++
	collection := Collection{ID: r.nextID, OwnerID: ownerID, Name: input.Name, Description: input.Description}
	r.collections[collection.ID] = collection
	return collection, nil
}

func (r *fakeRepository) FindCollectionByRefCode(_ context.Context, _ auth.Scope, refCode string) (Collection, error) {
	for _, collection := range r.collections {
		if collection.RefCode == refCode {
			return collection, nil
		}
	}
	return Collection{}, ErrCollectionNotFound
}

func (r *fakeRepository) LockCollectionByRefCode(ctx context.Context, refCode string) (Collection, error) {
	r.lockedCollectionRef = refCode
	return r.FindCollectionByRefCode(ctx, auth.Scope{All: true}, refCode)
}

func (r *fakeRepository) DeleteCollection(_ context.Context, ownerID int64, collectionID int64) error {
	collection, exists := r.collections[collectionID]
	if !exists || collection.OwnerID != ownerID {
		return ErrCollectionNotFound
	}
	delete(r.collections, collectionID)
	for id, file := range r.files {
		if file.CollectionID == collectionID {
			delete(r.files, id)
		}
	}
	return nil
}

func (r *fakeRepository) ListFileIDsForCollection(_ context.Context, ownerID int64, collectionID int64) ([]int64, error) {
	ids := make([]int64, 0)
	for id, file := range r.files {
		if file.OwnerID == ownerID && file.CollectionID == collectionID {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *fakeRepository) ListFiles(_ context.Context, _ auth.Scope, query FileQuery) (FilePage, error) {
	listedFiles := make([]File, 0, len(r.files))
	for _, file := range r.files {
		if query.CollectionRefCode != "" && file.CollectionRefCode != query.CollectionRefCode {
			continue
		}
		listedFiles = append(listedFiles, file)
	}
	sort.Slice(listedFiles, func(left int, right int) bool {
		return listedFiles[left].ID < listedFiles[right].ID
	})
	return FilePage{Files: listedFiles, Limit: query.Limit, Offset: query.Offset}, nil
}

func (r *fakeRepository) CreateFile(_ context.Context, ownerID int64, collectionID int64, input CreateFileInput, stored StoredFile) (File, error) {
	r.nextID++
	file := File{
		ID: r.nextID, OwnerID: ownerID, CollectionID: collectionID, CollectionRefCode: input.CollectionRefCode,
		ObjectKey: stored.ObjectKey, OriginalName: input.OriginalName, MimeType: input.MimeType,
		SizeBytes: stored.SizeBytes, SHA256: stored.SHA256, BLAKE3: stored.BLAKE3,
	}
	r.files[file.ID] = file
	return file, nil
}

func (r *fakeRepository) FindFileByRefCode(_ context.Context, _ auth.Scope, refCode string) (File, error) {
	for _, file := range r.files {
		if file.RefCode == refCode {
			return file, nil
		}
	}
	return File{}, ErrFileNotFound
}

func (r *fakeRepository) LockFileByRefCode(ctx context.Context, refCode string) (File, error) {
	return r.FindFileByRefCode(ctx, auth.Scope{All: true}, refCode)
}

func (r *fakeRepository) LockFileByID(_ context.Context, ownerID int64, fileID int64) (File, error) {
	file, exists := r.files[fileID]
	if !exists || file.OwnerID != ownerID {
		return File{}, ErrFileNotFound
	}
	return file, nil
}

func (r *fakeRepository) DeleteFile(_ context.Context, ownerID int64, fileID int64) error {
	file, exists := r.files[fileID]
	if !exists || file.OwnerID != ownerID {
		return ErrFileNotFound
	}
	delete(r.files, fileID)
	return nil
}

type fakeReferences struct {
	repo          *fakeRepository
	sequence      int64
	registrations []ref.Registration
	deletes       []fakeReferenceDelete
}

type fakeReferenceDelete struct {
	objectType ref.ObjectType
	objectID   int64
}

func (r *fakeReferences) ClaimCode(_ context.Context, objectType ref.ObjectType) (string, error) {
	r.sequence++
	return ref.FormatCode(objectType, r.sequence)
}

func (r *fakeReferences) Register(_ context.Context, registration ref.Registration) (ref.ObjectRef, error) {
	r.registrations = append(r.registrations, registration)
	object := ref.ObjectRef{
		ID: int64(len(r.registrations)), OwnerID: registration.OwnerID, RefCode: registration.RefCode,
		ObjectType: registration.ObjectType, ObjectID: registration.ObjectID,
		Title: registration.Title, Status: registration.Status,
	}
	switch registration.ObjectType {
	case ref.ObjectTypeFileCollection:
		collection := r.repo.collections[registration.ObjectID]
		collection.ObjectRefID = object.ID
		collection.RefCode = object.RefCode
		collection.Status = object.Status
		collection.Tags = registration.Tags
		r.repo.collections[registration.ObjectID] = collection
	case ref.ObjectTypeFile:
		file := r.repo.files[registration.ObjectID]
		file.ObjectRefID = object.ID
		file.RefCode = object.RefCode
		file.Status = object.Status
		file.Tags = registration.Tags
		r.repo.files[registration.ObjectID] = file
	}
	return object, nil
}

func (r *fakeReferences) Delete(_ context.Context, _ int64, objectType ref.ObjectType, objectID int64) error {
	r.deletes = append(r.deletes, fakeReferenceDelete{objectType: objectType, objectID: objectID})
	return nil
}

type fakeAudits struct {
	successes   []audit.Event
	standalones []audit.Event
}

func (a *fakeAudits) Record(_ context.Context, event audit.Event) (audit.Event, error) {
	a.successes = append(a.successes, event)
	return event, nil
}

func (a *fakeAudits) RecordStandalone(_ context.Context, event audit.Event) error {
	a.standalones = append(a.standalones, event)
	return nil
}

type fakeStorage struct {
	objects map[string][]byte
	deleted map[string]bool
}

func (s *fakeStorage) Put(_ context.Context, object storage.Object, body io.Reader) (storage.Object, error) {
	content, err := io.ReadAll(body)
	if err != nil {
		return storage.Object{}, err
	}
	if int64(len(content)) != object.Size {
		return storage.Object{}, errors.New("size mismatch")
	}
	object.SHA256 = sha256Hex(string(content))
	object.BLAKE3 = blake3Hex(string(content))
	s.objects[object.Key] = content
	return object, nil
}

func (s *fakeStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	content, exists := s.objects[key]
	if !exists {
		return nil, ErrFileNotFound
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (s *fakeStorage) Delete(_ context.Context, key string) error {
	delete(s.objects, key)
	s.deleted[key] = true
	return nil
}

func (s *fakeStorage) PathForKey(key string) (string, error) {
	return "objects/" + key, nil
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func blake3Hex(content string) string {
	sum := blake3.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func countAuditReason(events []audit.Event, reason string) int {
	count := 0
	for _, event := range events {
		if event.Reason == reason {
			count++
		}
	}
	return count
}
