// This file coordinates object storage operations and metadata.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"

	"lukechampine.com/blake3"
)

var ErrPromoteUnsupported = errors.New("storage promote is unsupported")

type Service struct {
	client Client
	repo   Repository
}

func NewService(client Client, repo Repository) *Service {
	return &Service{client: client, repo: repo}
}

func (s *Service) Put(ctx context.Context, object Object, body io.Reader) (Object, error) {
	object, err := s.Stage(ctx, object, body)
	if err != nil {
		return object, err
	}
	if s.repo == nil {
		return object, nil
	}
	return object, s.repo.Save(ctx, object)
}

func (s *Service) Stage(ctx context.Context, object Object, body io.Reader) (Object, error) {
	sha256Hash := sha256.New()
	blake3Hash := blake3.New(32, nil)
	hashingBody := io.TeeReader(body, io.MultiWriter(sha256Hash, blake3Hash))
	if err := s.client.Put(ctx, object.Key, hashingBody, object.Size); err != nil {
		return Object{}, err
	}
	object.SHA256 = hex.EncodeToString(sha256Hash.Sum(nil))
	object.BLAKE3 = hex.EncodeToString(blake3Hash.Sum(nil))
	return object, nil
}

func (s *Service) Promote(ctx context.Context, staged Object, final Object) (Object, error) {
	promoter, ok := s.client.(interface {
		Promote(context.Context, string, string) error
	})
	if !ok {
		return Object{}, ErrPromoteUnsupported
	}
	if strings.TrimSpace(staged.Key) == "" || strings.TrimSpace(final.Key) == "" {
		return Object{}, errors.New("storage object keys are required")
	}
	if strings.TrimSpace(final.Path) == "" {
		path, err := s.PathForKey(final.Key)
		if err != nil {
			return Object{}, err
		}
		final.Path = path
	}
	final.Size = staged.Size
	final.SHA256 = staged.SHA256
	final.BLAKE3 = staged.BLAKE3
	if err := promoter.Promote(ctx, staged.Key, final.Key); err != nil {
		return Object{}, err
	}
	if s.repo == nil {
		return final, nil
	}
	return final, s.repo.Save(ctx, final)
}

func (s *Service) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.client.Get(ctx, key)
}

func (s *Service) Delete(ctx context.Context, key string) error {
	if err := s.client.Delete(ctx, key); err != nil {
		return err
	}
	if s.repo == nil {
		return nil
	}
	return s.repo.Delete(ctx, key)
}

func (s *Service) PathForKey(key string) (string, error) {
	pathResolver, ok := s.client.(interface {
		PathForKey(string) (string, error)
	})
	if !ok {
		return key, nil
	}
	return pathResolver.PathForKey(key)
}
