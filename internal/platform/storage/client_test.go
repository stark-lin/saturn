// This file tests local filesystem object storage behavior.
package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFSClientPutGetDeleteAndResolvePath(t *testing.T) {
	root := t.TempDir()
	client := NewLocalFSClient(root)
	ctx := context.Background()

	if err := client.Put(ctx, "notes/blob.txt", strings.NewReader("hello"), int64(len("hello"))); err != nil {
		t.Fatalf("put object: %v", err)
	}
	path, err := client.PathForKey("notes/blob.txt")
	if err != nil {
		t.Fatalf("path for key: %v", err)
	}
	if path != filepath.Join(root, "notes", "blob.txt") {
		t.Fatalf("path = %q, want path under root", path)
	}

	body, err := client.Get(ctx, "notes/blob.txt")
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	defer body.Close()
	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q, want hello", string(content))
	}

	if err := client.Delete(ctx, "notes/blob.txt"); err != nil {
		t.Fatalf("delete object: %v", err)
	}
	if _, err := client.Get(ctx, "notes/blob.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("get deleted object error = %v, want not exist", err)
	}
}

func TestLocalFSClientRejectsInvalidKeys(t *testing.T) {
	client := NewLocalFSClient(t.TempDir())
	tests := []string{"", " ", "..", "../escape", "/absolute"}

	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			if _, err := client.PathForKey(key); err == nil {
				t.Fatalf("PathForKey(%q) error = nil, want error", key)
			}
		})
	}

	if _, err := NewLocalFSClient("").PathForKey("object"); err == nil {
		t.Fatal("PathForKey with empty root error = nil, want error")
	}
}

func TestLocalFSClientRejectsSizeMismatchWithoutCommittingObject(t *testing.T) {
	root := t.TempDir()
	client := NewLocalFSClient(root)
	ctx := context.Background()

	err := client.Put(ctx, "bad/blob.txt", strings.NewReader("hello"), int64(len("hello")+1))
	if err == nil {
		t.Fatal("put size mismatch error = nil, want error")
	}
	path, pathErr := client.PathForKey("bad/blob.txt")
	if pathErr != nil {
		t.Fatalf("path for key: %v", pathErr)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("stat uncommitted object error = %v, want not exist", statErr)
	}
}
