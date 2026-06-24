// This file tests object storage service coordination.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"lukechampine.com/blake3"
)

func TestServicePutHashesObjectAndSavesMetadata(t *testing.T) {
	client := &fakeStorageClient{}
	repo := &fakeStorageRepository{}
	service := NewService(client, repo)
	body := "stored content"

	object, err := service.Put(context.Background(), Object{Key: "objects/1", Size: int64(len(body))}, strings.NewReader(body))
	if err != nil {
		t.Fatalf("put object: %v", err)
	}

	wantSHA256 := sha256.Sum256([]byte(body))
	blake3Hash := blake3.New(32, nil)
	if _, err := blake3Hash.Write([]byte(body)); err != nil {
		t.Fatalf("write blake3 hash: %v", err)
	}
	wantBLAKE3 := hex.EncodeToString(blake3Hash.Sum(nil))

	if object.SHA256 != hex.EncodeToString(wantSHA256[:]) || object.BLAKE3 != wantBLAKE3 {
		t.Fatalf("hashes = SHA256 %q BLAKE3 %q", object.SHA256, object.BLAKE3)
	}
	if client.putKey != "objects/1" || client.putSize != int64(len(body)) || client.putBody != body {
		t.Fatalf("client put = key %q size %d body %q", client.putKey, client.putSize, client.putBody)
	}
	if len(repo.saved) != 1 || repo.saved[0].SHA256 != object.SHA256 || repo.saved[0].BLAKE3 != object.BLAKE3 {
		t.Fatalf("saved objects = %#v", repo.saved)
	}
}

func TestServiceStageHashesObjectWithoutSavingMetadata(t *testing.T) {
	client := &fakeStorageClient{}
	repo := &fakeStorageRepository{}
	service := NewService(client, repo)
	body := "stored content"

	object, err := service.Stage(context.Background(), Object{Key: "staging/1", Size: int64(len(body))}, strings.NewReader(body))
	if err != nil {
		t.Fatalf("stage object: %v", err)
	}
	if object.Key != "staging/1" || object.SHA256 != sha256HexForTest(body) || object.BLAKE3 != blake3HexForTest(body) {
		t.Fatalf("staged object = %#v", object)
	}
	if client.putKey != "staging/1" || client.putBody != body {
		t.Fatalf("client put = key %q body %q", client.putKey, client.putBody)
	}
	if len(repo.saved) != 0 {
		t.Fatalf("saved objects = %#v, want none", repo.saved)
	}
}

func TestServicePromoteMovesClientObjectAndSavesFinalMetadata(t *testing.T) {
	client := &fakeStorageClient{}
	repo := &fakeStorageRepository{}
	service := NewService(client, repo)
	staged := Object{
		Key: "staging/1", Path: "/objects/staging/1", Size: 5,
		SHA256: strings.Repeat("a", 64), BLAKE3: strings.Repeat("b", 64),
	}

	final, err := service.Promote(context.Background(), staged, Object{Key: "objects/1", Path: "/objects/1"})
	if err != nil {
		t.Fatalf("promote object: %v", err)
	}
	if client.promotedFrom != staged.Key || client.promotedTo != final.Key {
		t.Fatalf("promote = from %q to %q", client.promotedFrom, client.promotedTo)
	}
	if final.Key != "objects/1" || final.Path != "/objects/1" ||
		final.Size != staged.Size || final.SHA256 != staged.SHA256 || final.BLAKE3 != staged.BLAKE3 {
		t.Fatalf("final object = %#v", final)
	}
	if len(repo.saved) != 1 || repo.saved[0].Key != "objects/1" {
		t.Fatalf("saved objects = %#v", repo.saved)
	}
}

func TestServiceDeleteRemovesClientObjectBeforeMetadata(t *testing.T) {
	var operations []string
	client := &fakeStorageClient{operations: &operations}
	repo := &fakeStorageRepository{operations: &operations}
	service := NewService(client, repo)

	if err := service.Delete(context.Background(), "objects/1"); err != nil {
		t.Fatalf("delete object: %v", err)
	}

	want := []string{"client delete objects/1", "repo delete objects/1"}
	if len(operations) != len(want) {
		t.Fatalf("operations = %#v, want %#v", operations, want)
	}
	for i := range want {
		if operations[i] != want[i] {
			t.Fatalf("operations = %#v, want %#v", operations, want)
		}
	}
}

func TestServiceDeleteStopsWhenClientFails(t *testing.T) {
	var operations []string
	client := &fakeStorageClient{deleteErr: errors.New("client delete failed"), operations: &operations}
	repo := &fakeStorageRepository{operations: &operations}
	service := NewService(client, repo)

	err := service.Delete(context.Background(), "objects/1")
	if err == nil {
		t.Fatal("delete error = nil, want client error")
	}
	if len(operations) != 1 || operations[0] != "client delete objects/1" {
		t.Fatalf("operations = %#v, want only client delete", operations)
	}
}

func TestServiceGetAndPathForKeyDelegateToClient(t *testing.T) {
	client := &fakePathStorageClient{fakeStorageClient: fakeStorageClient{getBody: "content"}, path: "/tmp/object"}
	service := NewService(client, nil)

	body, err := service.Get(context.Background(), "objects/1")
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	defer body.Close()
	content, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if string(content) != "content" {
		t.Fatalf("content = %q, want content", string(content))
	}
	path, err := service.PathForKey("objects/1")
	if err != nil {
		t.Fatalf("path for key: %v", err)
	}
	if path != "/tmp/object" || client.pathKey != "objects/1" {
		t.Fatalf("path = %q key = %q", path, client.pathKey)
	}

	fallbackPath, err := NewService(&fakeStorageClient{}, nil).PathForKey("objects/2")
	if err != nil {
		t.Fatalf("fallback path for key: %v", err)
	}
	if fallbackPath != "objects/2" {
		t.Fatalf("fallback path = %q, want key", fallbackPath)
	}
}

func sha256HexForTest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func blake3HexForTest(content string) string {
	blake3Hash := blake3.New(32, nil)
	if _, err := blake3Hash.Write([]byte(content)); err != nil {
		panic(err)
	}
	return hex.EncodeToString(blake3Hash.Sum(nil))
}

type fakeStorageClient struct {
	putKey       string
	putSize      int64
	putBody      string
	putErr       error
	getBody      string
	getErr       error
	deleteErr    error
	promotedFrom string
	promotedTo   string
	promoteErr   error
	operations   *[]string
}

func (c *fakeStorageClient) Put(_ context.Context, key string, body io.Reader, size int64) error {
	content, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	c.putKey = key
	c.putSize = size
	c.putBody = string(content)
	return c.putErr
}

func (c *fakeStorageClient) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	return io.NopCloser(strings.NewReader(c.getBody)), nil
}

func (c *fakeStorageClient) Delete(_ context.Context, key string) error {
	if c.operations != nil {
		*c.operations = append(*c.operations, "client delete "+key)
	}
	return c.deleteErr
}

func (c *fakeStorageClient) Promote(_ context.Context, stagedKey string, finalKey string) error {
	c.promotedFrom = stagedKey
	c.promotedTo = finalKey
	return c.promoteErr
}

type fakePathStorageClient struct {
	fakeStorageClient
	path    string
	pathKey string
	pathErr error
}

func (c *fakePathStorageClient) PathForKey(key string) (string, error) {
	c.pathKey = key
	return c.path, c.pathErr
}

type fakeStorageRepository struct {
	saved      []Object
	deleted    []string
	saveErr    error
	deleteErr  error
	operations *[]string
}

func (r *fakeStorageRepository) Save(_ context.Context, object Object) error {
	r.saved = append(r.saved, object)
	return r.saveErr
}

func (r *fakeStorageRepository) Find(_ context.Context, _ string) (Object, error) {
	return Object{}, nil
}

func (r *fakeStorageRepository) Delete(_ context.Context, key string) error {
	r.deleted = append(r.deleted, key)
	if r.operations != nil {
		*r.operations = append(*r.operations, "repo delete "+key)
	}
	return r.deleteErr
}
