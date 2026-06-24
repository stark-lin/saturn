// This file defines the object storage client boundary.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Client interface {
	Put(ctx context.Context, key string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type LocalFSClient struct {
	root string
}

func NewLocalFSClient(root string) *LocalFSClient {
	return &LocalFSClient{root: strings.TrimSpace(root)}
}

func (c *LocalFSClient) Put(_ context.Context, key string, body io.Reader, size int64) error {
	path, err := c.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create object directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".blob-*")
	if err != nil {
		return fmt.Errorf("create temporary object: %w", err)
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()

	written, copyErr := io.Copy(temp, body)
	closeErr := temp.Close()
	if copyErr != nil {
		return fmt.Errorf("write object: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close object: %w", closeErr)
	}
	if size >= 0 && written != size {
		return fmt.Errorf("object size mismatch: wrote %d bytes, expected %d", written, size)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("commit object: %w", err)
	}
	committed = true
	return nil
}

func (c *LocalFSClient) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := c.pathForKey(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open object: %w", err)
	}
	return file, nil
}

func (c *LocalFSClient) Delete(_ context.Context, key string) error {
	path, err := c.pathForKey(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete object: %w", err)
	}
	_ = os.Remove(filepath.Dir(path))
	return nil
}

func (c *LocalFSClient) Promote(_ context.Context, stagedKey string, finalKey string) error {
	stagedPath, err := c.pathForKey(stagedKey)
	if err != nil {
		return err
	}
	finalPath, err := c.pathForKey(finalKey)
	if err != nil {
		return err
	}
	if _, err := os.Stat(finalPath); err == nil {
		return fmt.Errorf("promote object: destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("promote object: inspect destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return fmt.Errorf("create object directory: %w", err)
	}
	if err := os.Rename(stagedPath, finalPath); err != nil {
		return fmt.Errorf("promote object: %w", err)
	}
	_ = os.Remove(filepath.Dir(stagedPath))
	return nil
}

func (c *LocalFSClient) PathForKey(key string) (string, error) {
	return c.pathForKey(key)
}

func (c *LocalFSClient) pathForKey(key string) (string, error) {
	root := strings.TrimSpace(c.root)
	if root == "" {
		return "", errors.New("storage root is required")
	}
	cleanKey := filepath.Clean(strings.TrimSpace(key))
	if cleanKey == "." || cleanKey == "" || filepath.IsAbs(cleanKey) || strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) || cleanKey == ".." {
		return "", errors.New("invalid object key")
	}
	return filepath.Join(root, cleanKey), nil
}
