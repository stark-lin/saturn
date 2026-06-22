// This file coordinates object storage operations and metadata.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"lukechampine.com/blake3"
)

type Service struct {
	client Client
	repo   Repository
}

func NewService(client Client, repo Repository) *Service {
	return &Service{client: client, repo: repo}
}

func (s *Service) Put(ctx context.Context, object Object, body io.Reader) (Object, error) {
	sha256Hash := sha256.New()
	blake3Hash := blake3.New(32, nil)
	hashingBody := io.TeeReader(body, io.MultiWriter(sha256Hash, blake3Hash))
	if err := s.client.Put(ctx, object.Key, hashingBody, object.Size); err != nil {
		return Object{}, err
	}
	object.SHA256 = hex.EncodeToString(sha256Hash.Sum(nil))
	object.BLAKE3 = hex.EncodeToString(blake3Hash.Sum(nil))
	if s.repo == nil {
		return object, nil
	}
	return object, s.repo.Save(ctx, object)
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
