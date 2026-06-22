// This file enforces immutable Files collection and file rules.
package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/stark-lin/go-proj/internal/platform/audit"
	"github.com/stark-lin/go-proj/internal/platform/auth"
	platformdb "github.com/stark-lin/go-proj/internal/platform/db"
	"github.com/stark-lin/go-proj/internal/platform/ref"
	"github.com/stark-lin/go-proj/internal/platform/storage"
	"lukechampine.com/blake3"
)

var (
	ErrDependencyUnavailable = errors.New("files dependency is not wired")
	ErrInvalidCollection     = errors.New("invalid file collection")
	ErrInvalidFile           = errors.New("invalid file")
	ErrInvalidQuery          = errors.New("invalid files query")
	ErrIntegrityCheckFailed  = errors.New("file integrity check failed")
)

type ObjectReferenceService interface {
	ClaimCode(ctx context.Context, objectType ref.ObjectType) (string, error)
	Register(ctx context.Context, registration ref.Registration) (ref.ObjectRef, error)
	Delete(ctx context.Context, ownerID int64, objectType ref.ObjectType, objectID int64) error
}

type AuditService interface {
	Record(ctx context.Context, event audit.Event) (audit.Event, error)
	RecordStandalone(ctx context.Context, event audit.Event) error
}

type StorageService interface {
	Put(ctx context.Context, object storage.Object, body io.Reader) (storage.Object, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	PathForKey(key string) (string, error)
}

type Service struct {
	repo         Repository
	transactions platformdb.TransactionRunner
	references   ObjectReferenceService
	audit        AuditService
	storage      StorageService
	authorizer   *auth.Authorizer
}

func NewService(
	repo Repository,
	transactions platformdb.TransactionRunner,
	references ObjectReferenceService,
	auditService AuditService,
	storageService StorageService,
) *Service {
	if transactions == nil {
		transactions = platformdb.NoopTransactionRunner{}
	}
	return &Service{
		repo: repo, transactions: transactions, references: references, audit: auditService,
		storage: storageService, authorizer: auth.NewAuthorizer(),
	}
}

func (s *Service) ListCollections(ctx context.Context, actor auth.Principal, query CollectionQuery) (CollectionPage, error) {
	if actor.IsZero() {
		return CollectionPage{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return CollectionPage{}, ErrRepositoryUnavailable
	}
	query, err := normalizeCollectionQuery(query)
	if err != nil {
		return CollectionPage{}, err
	}
	return s.repo.ListCollections(ctx, auth.ScopeForPrincipal(actor), query)
}

func (s *Service) CreateCollection(ctx context.Context, actor auth.Principal, input CreateCollectionInput) (Collection, error) {
	if actor.IsZero() {
		return Collection{}, auth.ErrUnauthenticated
	}
	input, err := normalizeCollectionInput(input)
	if err != nil {
		return Collection{}, err
	}
	if err := s.requireWriteDependencies(); err != nil {
		return Collection{}, err
	}
	refCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeFileCollection)
	if err != nil {
		return Collection{}, err
	}
	var created Collection
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		collection, err := s.repo.CreateCollection(txCtx, actor.ID, input)
		if err != nil {
			return err
		}
		object, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: actor.ID, RefCode: refCode, ObjectType: ref.ObjectTypeFileCollection,
			ObjectID: collection.ID, Title: collection.Name, Tags: input.Tags, Status: CollectionStatusActive,
		})
		if err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
			TargetRefCode: object.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		collection.ObjectRefID = object.ID
		collection.RefCode = object.RefCode
		collection.Status = object.Status
		collection.Tags = input.Tags
		created = collection
		return nil
	})
	if err != nil {
		return Collection{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, refCode, err)
	}
	return created, nil
}

func (s *Service) GetCollection(ctx context.Context, actor auth.Principal, refCode string) (Collection, error) {
	if actor.IsZero() {
		return Collection{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return Collection{}, ErrRepositoryUnavailable
	}
	return s.repo.FindCollectionByRefCode(ctx, auth.ScopeForPrincipal(actor), ref.NormalizeCode(refCode))
}

func (s *Service) DeleteCollection(ctx context.Context, actor auth.Principal, refCode string) error {
	if actor.IsZero() {
		return auth.ErrUnauthenticated
	}
	if err := s.requireWriteDependencies(); err != nil {
		return err
	}
	refCode = ref.NormalizeCode(refCode)
	objectKeys := make([]string, 0)
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		collection, err := s.repo.LockCollectionByRefCode(txCtx, refCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionDelete, "file_collection", collection.ID, collection.OwnerID); err != nil {
			return err
		}
		fileIDs, err := s.repo.ListFileIDsForCollection(txCtx, collection.OwnerID, collection.ID)
		if err != nil {
			return err
		}
		for _, fileID := range fileIDs {
			objectKey, err := s.deleteFileByID(txCtx, actor, collection.OwnerID, fileID, DeleteReasonCascadeCollectionDelete)
			if err != nil {
				return err
			}
			objectKeys = append(objectKeys, objectKey)
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
			TargetRefCode: collection.RefCode, Result: audit.ResultSuccess, Reason: string(DeleteReasonCollectionDelete),
		}); err != nil {
			return err
		}
		if err := s.references.Delete(txCtx, collection.OwnerID, ref.ObjectTypeFileCollection, collection.ID); err != nil {
			return err
		}
		return s.repo.DeleteCollection(txCtx, collection.OwnerID, collection.ID)
	})
	if err != nil {
		return s.recordWriteFailure(ctx, actor, audit.ActionDelete, refCode, err)
	}
	return s.deleteStoredObjects(ctx, objectKeys)
}

func (s *Service) ListFiles(ctx context.Context, actor auth.Principal, query FileQuery) (FilePage, error) {
	if actor.IsZero() {
		return FilePage{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return FilePage{}, ErrRepositoryUnavailable
	}
	query, err := normalizeFileQuery(query)
	if err != nil {
		return FilePage{}, err
	}
	return s.repo.ListFiles(ctx, auth.ScopeForPrincipal(actor), query)
}

func (s *Service) CreateFile(ctx context.Context, actor auth.Principal, input CreateFileInput) (File, error) {
	if actor.IsZero() {
		return File{}, auth.ErrUnauthenticated
	}
	input, err := normalizeFileInput(input)
	if err != nil {
		return File{}, err
	}
	if err := s.requireWriteDependencies(); err != nil {
		return File{}, err
	}
	refCode, err := s.references.ClaimCode(ctx, ref.ObjectTypeFile)
	if err != nil {
		return File{}, err
	}
	objectKey := fmt.Sprintf("%s/blob", refCode)
	var objectStored bool
	var created File
	err = s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		collection, err := s.repo.LockCollectionByRefCode(txCtx, input.CollectionRefCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionUpdate, "file_collection", collection.ID, collection.OwnerID); err != nil {
			return err
		}
		path, err := s.storage.PathForKey(objectKey)
		if err != nil {
			return err
		}
		object, err := s.storage.Put(txCtx, storage.Object{
			Key: objectKey, Path: path, Size: input.SizeBytes,
		}, input.Body)
		if err != nil {
			return err
		}
		objectStored = true
		file, err := s.repo.CreateFile(txCtx, collection.OwnerID, collection.ID, input, StoredFile{
			ObjectKey: object.Key, SizeBytes: object.Size, SHA256: object.SHA256, BLAKE3: object.BLAKE3,
		})
		if err != nil {
			return err
		}
		objectRef, err := s.references.Register(txCtx, ref.Registration{
			OwnerID: collection.OwnerID, RefCode: refCode, ObjectType: ref.ObjectTypeFile,
			ObjectID: file.ID, Title: file.OriginalName, Tags: input.Tags, Status: FileStatusActive,
		})
		if err != nil {
			return err
		}
		if _, err := s.audit.Record(txCtx, audit.Event{
			ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionCreate,
			TargetRefCode: objectRef.RefCode, Result: audit.ResultSuccess,
		}); err != nil {
			return err
		}
		file.ObjectRefID = objectRef.ID
		file.RefCode = objectRef.RefCode
		file.CollectionRefCode = collection.RefCode
		file.Status = objectRef.Status
		file.Tags = input.Tags
		created = file
		return nil
	})
	if err != nil {
		if objectStored {
			_ = s.storage.Delete(context.Background(), objectKey)
		}
		return File{}, s.recordWriteFailure(ctx, actor, audit.ActionCreate, refCode, err)
	}
	return created, nil
}

func (s *Service) GetFile(ctx context.Context, actor auth.Principal, refCode string) (File, error) {
	if actor.IsZero() {
		return File{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return File{}, ErrRepositoryUnavailable
	}
	return s.repo.FindFileByRefCode(ctx, auth.ScopeForPrincipal(actor), ref.NormalizeCode(refCode))
}

func (s *Service) DeleteFile(ctx context.Context, actor auth.Principal, refCode string) error {
	if actor.IsZero() {
		return auth.ErrUnauthenticated
	}
	if err := s.requireWriteDependencies(); err != nil {
		return err
	}
	refCode = ref.NormalizeCode(refCode)
	var objectKey string
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		file, err := s.repo.LockFileByRefCode(txCtx, refCode)
		if err != nil {
			return err
		}
		if err := s.can(actor, auth.ActionDelete, "file", file.ID, file.OwnerID); err != nil {
			return err
		}
		objectKey, err = s.deleteLockedFile(txCtx, actor, file, DeleteReasonDirectFileDelete)
		return err
	})
	if err != nil {
		return s.recordWriteFailure(ctx, actor, audit.ActionDelete, refCode, err)
	}
	return s.deleteStoredObjects(ctx, []string{objectKey})
}

func (s *Service) OpenVerifiedDownload(ctx context.Context, actor auth.Principal, refCode string) (VerifiedDownload, error) {
	if actor.IsZero() {
		return VerifiedDownload{}, auth.ErrUnauthenticated
	}
	if s.repo == nil {
		return VerifiedDownload{}, ErrRepositoryUnavailable
	}
	if s.storage == nil {
		return VerifiedDownload{}, ErrDependencyUnavailable
	}
	if s.audit == nil {
		return VerifiedDownload{}, ErrDependencyUnavailable
	}
	file, err := s.repo.FindFileByRefCode(ctx, auth.ScopeForPrincipal(actor), ref.NormalizeCode(refCode))
	if err != nil {
		return VerifiedDownload{}, err
	}
	if err := s.can(actor, auth.ActionRead, "file", file.ID, file.OwnerID); err != nil {
		return VerifiedDownload{}, err
	}
	body, err := s.storage.Get(ctx, file.ObjectKey)
	if err != nil {
		return VerifiedDownload{}, err
	}
	defer body.Close()

	staged, err := stageVerifiedDownload(body, file)
	if err != nil {
		if errors.Is(err, ErrIntegrityCheckFailed) {
			return VerifiedDownload{}, errors.Join(err, s.recordDownloadIntegrityFailure(ctx, actor, file.RefCode))
		}
		return VerifiedDownload{}, err
	}
	if err := s.recordDownloadAudit(ctx, actor, file.RefCode, audit.ResultSuccess, "download"); err != nil {
		_ = staged.Close()
		return VerifiedDownload{}, err
	}
	return VerifiedDownload{File: file, Body: staged}, nil
}

func (s *Service) deleteFileByID(ctx context.Context, actor auth.Principal, ownerID int64, fileID int64, reason DeleteReason) (string, error) {
	file, err := s.repo.LockFileByID(ctx, ownerID, fileID)
	if err != nil {
		return "", err
	}
	return s.deleteLockedFile(ctx, actor, file, reason)
}

func (s *Service) deleteLockedFile(ctx context.Context, actor auth.Principal, file File, reason DeleteReason) (string, error) {
	if _, err := s.audit.Record(ctx, audit.Event{
		ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: audit.ActionDelete,
		TargetRefCode: file.RefCode, Result: audit.ResultSuccess, Reason: string(reason),
	}); err != nil {
		return "", err
	}
	if err := s.references.Delete(ctx, file.OwnerID, ref.ObjectTypeFile, file.ID); err != nil {
		return "", err
	}
	if err := s.repo.DeleteFile(ctx, file.OwnerID, file.ID); err != nil {
		return "", err
	}
	return file.ObjectKey, nil
}

func (s *Service) deleteStoredObjects(ctx context.Context, objectKeys []string) error {
	for _, objectKey := range objectKeys {
		if strings.TrimSpace(objectKey) == "" {
			continue
		}
		if err := s.storage.Delete(ctx, objectKey); err != nil {
			return err
		}
	}
	return nil
}

func stageVerifiedDownload(body io.Reader, file File) (io.ReadCloser, error) {
	temp, err := os.CreateTemp("", "saturn-file-download-*")
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = temp.Close()
			_ = os.Remove(temp.Name())
		}
	}()

	sha256Hash := sha256.New()
	blake3Hash := blake3.New(32, nil)
	written, err := io.Copy(io.MultiWriter(temp, sha256Hash, blake3Hash), body)
	if err != nil {
		return nil, err
	}
	if written != file.SizeBytes ||
		hex.EncodeToString(sha256Hash.Sum(nil)) != file.SHA256 ||
		hex.EncodeToString(blake3Hash.Sum(nil)) != file.BLAKE3 {
		return nil, ErrIntegrityCheckFailed
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	cleanup = false
	return removeOnCloseFile{File: temp}, nil
}

type removeOnCloseFile struct {
	*os.File
}

func (f removeOnCloseFile) Close() error {
	name := f.Name()
	err := f.File.Close()
	removeErr := os.Remove(name)
	if err != nil {
		return err
	}
	return removeErr
}

func (s *Service) recordWriteFailure(ctx context.Context, actor auth.Principal, action audit.Action, refCode string, operationErr error) error {
	result := audit.ResultFailed
	reason := "operation_failed"
	if errors.Is(operationErr, ErrCollectionNotFound) || errors.Is(operationErr, ErrFileNotFound) ||
		errors.Is(operationErr, auth.ErrForbidden) || errors.Is(operationErr, ref.ErrNotFound) {
		result = audit.ResultDenied
		reason = "not_found"
	}
	if s.audit == nil {
		return operationErr
	}
	auditErr := s.audit.RecordStandalone(ctx, audit.Event{
		ActorType: audit.ActorTypeUser, ActorUserID: actor.ID, Action: action,
		TargetRefCode: refCode, Result: result, Reason: reason,
	})
	if auditErr != nil {
		return errors.Join(operationErr, auditErr)
	}
	return operationErr
}

func (s *Service) recordDownloadIntegrityFailure(ctx context.Context, actor auth.Principal, refCode string) error {
	return s.recordDownloadAudit(ctx, actor, refCode, audit.ResultFailed, "integrity_check_failed")
}

func (s *Service) recordDownloadAudit(ctx context.Context, actor auth.Principal, refCode string, result audit.Result, reason string) error {
	return s.audit.RecordStandalone(ctx, audit.Event{
		ActorType:     audit.ActorTypeUser,
		ActorUserID:   actor.ID,
		Action:        audit.ActionExport,
		TargetRefCode: refCode,
		Result:        result,
		Reason:        reason,
	})
}

func (s *Service) requireWriteDependencies() error {
	if s.repo == nil {
		return ErrRepositoryUnavailable
	}
	if s.references == nil || s.audit == nil || s.storage == nil {
		return ErrDependencyUnavailable
	}
	return nil
}

func (s *Service) can(actor auth.Principal, action auth.Action, resourceType string, resourceID int64, ownerID int64) error {
	return s.authorizer.Can(actor, action, auth.Resource{Type: resourceType, ID: resourceID, OwnerID: ownerID})
}

func normalizeCollectionInput(input CreateCollectionInput) (CreateCollectionInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Tags = normalizedTags(input.Tags)
	if input.Name == "" {
		return CreateCollectionInput{}, ErrInvalidCollection
	}
	return input, nil
}

func normalizeFileInput(input CreateFileInput) (CreateFileInput, error) {
	input.CollectionRefCode = ref.NormalizeCode(input.CollectionRefCode)
	input.OriginalName = strings.TrimSpace(input.OriginalName)
	input.MimeType = strings.TrimSpace(input.MimeType)
	input.Tags = normalizedTags(input.Tags)
	if input.MimeType == "" {
		input.MimeType = "application/octet-stream"
	}
	if !ref.ValidCode(input.CollectionRefCode) ||
		!ref.CodeMatchesObjectType(input.CollectionRefCode, ref.ObjectTypeFileCollection) ||
		input.OriginalName == "" || input.SizeBytes < 0 || input.Body == nil {
		return CreateFileInput{}, ErrInvalidFile
	}
	return input, nil
}

func normalizeCollectionQuery(query CollectionQuery) (CollectionQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	if query.Limit < 1 || query.Limit > MaxLimit || query.Offset < 0 {
		return CollectionQuery{}, ErrInvalidQuery
	}
	return query, nil
}

func normalizeFileQuery(query FileQuery) (FileQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultLimit
	}
	query.CollectionRefCode = ref.NormalizeCode(query.CollectionRefCode)
	query.Tag = strings.TrimSpace(query.Tag)
	if query.Limit < 1 || query.Limit > MaxLimit || query.Offset < 0 ||
		(query.CollectionRefCode != "" && (!ref.ValidCode(query.CollectionRefCode) ||
			!ref.CodeMatchesObjectType(query.CollectionRefCode, ref.ObjectTypeFileCollection))) {
		return FileQuery{}, ErrInvalidQuery
	}
	return query, nil
}

func normalizedTags(names []string) []string {
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
