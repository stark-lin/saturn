// This file tests the Files HTTP contract for immutable file flows.
package files

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stark-lin/saturn/internal/platform/auth"
)

func TestHandlerCreatesListsGetsDownloadsAndDeletesFile(t *testing.T) {
	service, _, _, _, _ := newTestService()
	handler := NewHandler(service)

	collectionRequest := authenticatedFilesRequest(http.MethodPost, "/api/files/collections",
		`{"name":"Receipts","description":"Tax year","tags":["tax"]}`)
	collectionResponseRecorder := httptest.NewRecorder()
	handler.CreateCollection(collectionResponseRecorder, collectionRequest)
	if collectionResponseRecorder.Code != http.StatusCreated ||
		collectionResponseRecorder.Header().Get("Location") != "/api/files/collections/FIL-00000001" {
		t.Fatalf("create collection response = %d location %q", collectionResponseRecorder.Code, collectionResponseRecorder.Header().Get("Location"))
	}
	var createdCollection CollectionResponse
	if err := json.NewDecoder(collectionResponseRecorder.Body).Decode(&createdCollection); err != nil {
		t.Fatalf("decode collection response: %v", err)
	}
	if createdCollection.Collection.RefCode != "FIL-00000001" || createdCollection.Collection.Name != "Receipts" {
		t.Fatalf("created collection = %#v", createdCollection.Collection)
	}

	fileRequest := authenticatedFilesMultipartRequest(t, "/api/files/collections/FIL-00000001/files", "receipt.txt", "hello", "receipt,tax")
	fileRequest.SetPathValue("ref_code", createdCollection.Collection.RefCode)
	fileResponseRecorder := httptest.NewRecorder()
	handler.CreateFile(fileResponseRecorder, fileRequest)
	if fileResponseRecorder.Code != http.StatusCreated ||
		fileResponseRecorder.Header().Get("Location") != "/api/files/FIL-00000002" {
		t.Fatalf("create file response = %d location %q", fileResponseRecorder.Code, fileResponseRecorder.Header().Get("Location"))
	}
	var createdFile FileResponse
	if err := json.NewDecoder(fileResponseRecorder.Body).Decode(&createdFile); err != nil {
		t.Fatalf("decode file response: %v", err)
	}
	if createdFile.File.Metadata.SHA256 == "" || createdFile.File.Metadata.BLAKE3 == "" ||
		createdFile.File.Metadata.OriginalName != "receipt.txt" {
		t.Fatalf("created file metadata = %#v", createdFile.File.Metadata)
	}

	listCollectionsRequest := authenticatedFilesRequest(http.MethodGet, "/api/files/collections?limit=10", "")
	listCollectionsResponse := httptest.NewRecorder()
	handler.ListCollections(listCollectionsResponse, listCollectionsRequest)
	if listCollectionsResponse.Code != http.StatusOK {
		t.Fatalf("list collections status = %d", listCollectionsResponse.Code)
	}
	var collections CollectionsResponse
	if err := json.NewDecoder(listCollectionsResponse.Body).Decode(&collections); err != nil {
		t.Fatalf("decode collections response: %v", err)
	}
	if len(collections.Collections) != 1 || collections.Collections[0].RefCode != createdCollection.Collection.RefCode {
		t.Fatalf("collections response = %#v", collections.Collections)
	}

	getCollectionRequest := authenticatedFilesRequest(http.MethodGet, "/api/files/collections/FIL-00000001", "")
	getCollectionRequest.SetPathValue("ref_code", createdCollection.Collection.RefCode)
	getCollectionResponse := httptest.NewRecorder()
	handler.GetCollection(getCollectionResponse, getCollectionRequest)
	if getCollectionResponse.Code != http.StatusOK {
		t.Fatalf("get collection status = %d", getCollectionResponse.Code)
	}

	listFilesRequest := authenticatedFilesRequest(http.MethodGet, "/api/files?collection_ref_code=FIL-00000001&limit=10", "")
	listFilesResponse := httptest.NewRecorder()
	handler.ListFiles(listFilesResponse, listFilesRequest)
	if listFilesResponse.Code != http.StatusOK {
		t.Fatalf("list files status = %d", listFilesResponse.Code)
	}
	var files FilesResponse
	if err := json.NewDecoder(listFilesResponse.Body).Decode(&files); err != nil {
		t.Fatalf("decode files response: %v", err)
	}
	if len(files.Files) != 1 || files.Files[0].RefCode != createdFile.File.RefCode {
		t.Fatalf("files response = %#v", files.Files)
	}

	getFileRequest := authenticatedFilesRequest(http.MethodGet, "/api/files/FIL-00000002", "")
	getFileRequest.SetPathValue("ref_code", createdFile.File.RefCode)
	getFileResponse := httptest.NewRecorder()
	handler.GetFile(getFileResponse, getFileRequest)
	if getFileResponse.Code != http.StatusOK {
		t.Fatalf("get file status = %d", getFileResponse.Code)
	}

	downloadRequest := authenticatedFilesRequest(http.MethodGet, "/api/files/FIL-00000002/download", "")
	downloadRequest.SetPathValue("ref_code", createdFile.File.RefCode)
	downloadResponse := httptest.NewRecorder()
	handler.DownloadFile(downloadResponse, downloadRequest)
	if downloadResponse.Code != http.StatusOK {
		t.Fatalf("download status = %d", downloadResponse.Code)
	}
	if got := downloadResponse.Header().Get("X-Content-SHA256"); got != createdFile.File.SHA256 {
		t.Fatalf("download sha256 = %q, want %q", got, createdFile.File.SHA256)
	}
	if disposition := downloadResponse.Header().Get("Content-Disposition"); !strings.Contains(disposition, "receipt.txt") {
		t.Fatalf("content disposition = %q", disposition)
	}
	if body := downloadResponse.Body.String(); body != "hello" {
		t.Fatalf("download body = %q", body)
	}

	deleteFileRequest := authenticatedFilesRequest(http.MethodDelete, "/api/files/FIL-00000002", "")
	deleteFileRequest.SetPathValue("ref_code", createdFile.File.RefCode)
	deleteFileResponse := httptest.NewRecorder()
	handler.DeleteFile(deleteFileResponse, deleteFileRequest)
	if deleteFileResponse.Code != http.StatusNoContent {
		t.Fatalf("delete file status = %d", deleteFileResponse.Code)
	}

	deleteCollectionRequest := authenticatedFilesRequest(http.MethodDelete, "/api/files/collections/FIL-00000001", "")
	deleteCollectionRequest.SetPathValue("ref_code", createdCollection.Collection.RefCode)
	deleteCollectionResponse := httptest.NewRecorder()
	handler.DeleteCollection(deleteCollectionResponse, deleteCollectionRequest)
	if deleteCollectionResponse.Code != http.StatusNoContent {
		t.Fatalf("delete collection status = %d", deleteCollectionResponse.Code)
	}
}

func TestHandlerMapsFilesErrorsAndInvalidRequests(t *testing.T) {
	service, _, _, _, storage := newTestService()
	handler := NewHandler(service)
	actor := auth.Principal{ID: 7, Role: auth.RoleUser}
	collection, err := service.CreateCollection(context.Background(), actor, CreateCollectionInput{Name: "Receipts"})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	file, err := service.CreateFile(context.Background(), actor, CreateFileInput{
		CollectionRefCode: collection.RefCode, OriginalName: "receipt.txt", SizeBytes: 5, Body: bytes.NewBufferString("hello"),
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	storage.objects[file.ObjectKey] = []byte("HELLO")

	downloadRequest := authenticatedFilesRequest(http.MethodGet, "/api/files/FIL-00000002/download", "")
	downloadRequest.SetPathValue("ref_code", file.RefCode)
	downloadResponse := httptest.NewRecorder()
	handler.DownloadFile(downloadResponse, downloadRequest)
	if downloadResponse.Code != http.StatusConflict {
		t.Fatalf("download status = %d, want %d", downloadResponse.Code, http.StatusConflict)
	}

	invalidCollectionRequest := authenticatedFilesRequest(http.MethodGet, "/api/files/collections/NTE-00000001", "")
	invalidCollectionRequest.SetPathValue("ref_code", "NTE-00000001")
	invalidCollectionResponse := httptest.NewRecorder()
	handler.GetCollection(invalidCollectionResponse, invalidCollectionRequest)
	if invalidCollectionResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid collection ref status = %d, want %d", invalidCollectionResponse.Code, http.StatusBadRequest)
	}

	missingFileUploadRequest := authenticatedFilesRequest(http.MethodPost, "/api/files/collections/FIL-00000001/files", "")
	missingFileUploadRequest.SetPathValue("ref_code", collection.RefCode)
	missingFileUploadResponse := httptest.NewRecorder()
	handler.CreateFile(missingFileUploadResponse, missingFileUploadRequest)
	if missingFileUploadResponse.Code != http.StatusBadRequest {
		t.Fatalf("missing file status = %d, want %d", missingFileUploadResponse.Code, http.StatusBadRequest)
	}

	unauthenticatedRequest := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ListFiles(unauthenticatedResponse, unauthenticatedRequest)
	if unauthenticatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", unauthenticatedResponse.Code, http.StatusUnauthorized)
	}
}

func authenticatedFilesRequest(method string, target string, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = http.NoBody
	}
	request := httptest.NewRequest(method, target, reader)
	return request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
}

func authenticatedFilesMultipartRequest(t *testing.T, target string, filename string, content string, tags string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	filePart, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := filePart.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if tags != "" {
		if err := writer.WriteField("tags", tags); err != nil {
			t.Fatalf("write multipart tags: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request.WithContext(auth.ContextWithPrincipal(request.Context(), auth.Principal{ID: 7, Role: auth.RoleUser}))
}
