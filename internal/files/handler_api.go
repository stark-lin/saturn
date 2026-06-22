// This file exposes authenticated Files collection and file API handlers.
package files

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/stark-lin/go-proj/internal/platform/auth"
	"github.com/stark-lin/go-proj/internal/platform/httpx"
	"github.com/stark-lin/go-proj/internal/platform/ref"
)

func (h *Handler) ListCollections(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	query, err := bindCollectionQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid files collection query")
		return
	}
	page, err := h.service.ListCollections(r.Context(), principal, query)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, collectionsResponse(page))
}

func (h *Handler) CreateCollection(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	var request CreateCollectionRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid files collection request")
		return
	}
	collection, err := h.service.CreateCollection(r.Context(), principal, CreateCollectionInput{
		Name: request.Name, Description: request.Description, Tags: request.Tags,
	})
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/files/collections/"+collection.RefCode)
	httpx.WriteJSON(w, http.StatusCreated, collectionResponse(collection))
}

func (h *Handler) GetCollection(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := collectionResourceRequest(w, r)
	if !ok {
		return
	}
	collection, err := h.service.GetCollection(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, collectionResponse(collection))
}

func (h *Handler) DeleteCollection(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := collectionResourceRequest(w, r)
	if !ok {
		return
	}
	if h.writeServiceError(w, h.service.DeleteCollection(r.Context(), principal, refCode)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return
	}
	query, err := bindFileQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid files query")
		return
	}
	page, err := h.service.ListFiles(r.Context(), principal, query)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, filesResponse(page))
}

func (h *Handler) CreateFile(w http.ResponseWriter, r *http.Request) {
	principal, collectionRefCode, ok := collectionResourceRequest(w, r)
	if !ok {
		return
	}
	fileBody, fileHeader, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Multipart field file is required")
		return
	}
	defer fileBody.Close()

	mimeType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	file, err := h.service.CreateFile(r.Context(), principal, CreateFileInput{
		CollectionRefCode: collectionRefCode,
		OriginalName:      fileHeader.Filename,
		MimeType:          mimeType,
		SizeBytes:         fileHeader.Size,
		Body:              fileBody,
		Tags:              multipartTags(r, "tags"),
	})
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/files/"+file.RefCode)
	httpx.WriteJSON(w, http.StatusCreated, fileResponse(file))
}

func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := fileResourceRequest(w, r)
	if !ok {
		return
	}
	file, err := h.service.GetFile(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, fileResponse(file))
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := fileResourceRequest(w, r)
	if !ok {
		return
	}
	download, err := h.service.OpenVerifiedDownload(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	defer download.Body.Close()

	w.Header().Set("Content-Type", download.File.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(download.File.SizeBytes, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": download.File.OriginalName}))
	w.Header().Set("X-Content-SHA256", download.File.SHA256)
	w.Header().Set("X-Content-BLAKE3", download.File.BLAKE3)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, download.Body)
}

func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := fileResourceRequest(w, r)
	if !ok {
		return
	}
	if h.writeServiceError(w, h.service.DeleteFile(r.Context(), principal, refCode)) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
	case errors.Is(err, ErrInvalidCollection), errors.Is(err, ErrInvalidFile), errors.Is(err, ErrInvalidQuery):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid files request")
	case errors.Is(err, ErrIntegrityCheckFailed):
		httpx.WriteError(w, http.StatusConflict, "integrity_check_failed", "File integrity check failed")
	case errors.Is(err, ErrCollectionNotFound), errors.Is(err, ErrFileNotFound),
		errors.Is(err, auth.ErrForbidden), errors.Is(err, ref.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Files resource not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "files_unavailable", "Files service is unavailable")
	}
	return true
}

func authenticatedPrincipal(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return auth.Principal{}, false
	}
	return principal, true
}

func collectionResourceRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, string, bool) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return auth.Principal{}, "", false
	}
	refCode := ref.NormalizeCode(r.PathValue("ref_code"))
	if !ref.ValidCode(refCode) || !ref.CodeMatchesObjectType(refCode, ref.ObjectTypeFileCollection) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return auth.Principal{}, "", false
	}
	return principal, refCode, true
}

func fileResourceRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, string, bool) {
	principal, ok := authenticatedPrincipal(w, r)
	if !ok {
		return auth.Principal{}, "", false
	}
	refCode := ref.NormalizeCode(r.PathValue("ref_code"))
	if !ref.ValidCode(refCode) || !ref.CodeMatchesObjectType(refCode, ref.ObjectTypeFile) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return auth.Principal{}, "", false
	}
	return principal, refCode, true
}

func bindCollectionQuery(values url.Values) (CollectionQuery, error) {
	for key := range values {
		if key != "limit" && key != "offset" {
			return CollectionQuery{}, ErrInvalidQuery
		}
	}
	query := CollectionQuery{Limit: DefaultLimit}
	if err := bindPagination(values, &query.Limit, &query.Offset); err != nil {
		return CollectionQuery{}, err
	}
	return query, nil
}

func bindFileQuery(values url.Values) (FileQuery, error) {
	for key := range values {
		switch key {
		case "collection_ref_code", "tag", "limit", "offset":
		default:
			return FileQuery{}, ErrInvalidQuery
		}
	}
	query := FileQuery{
		CollectionRefCode: strings.TrimSpace(values.Get("collection_ref_code")),
		Tag:               strings.TrimSpace(values.Get("tag")),
		Limit:             DefaultLimit,
	}
	if err := bindPagination(values, &query.Limit, &query.Offset); err != nil {
		return FileQuery{}, err
	}
	return query, nil
}

func multipartTags(r *http.Request, field string) []string {
	if r.MultipartForm == nil {
		return nil
	}
	values := r.MultipartForm.Value[field]
	tags := make([]string, 0, len(values))
	for _, value := range values {
		for _, tag := range strings.Split(value, ",") {
			tags = append(tags, tag)
		}
	}
	return tags
}

func bindPagination(values url.Values, limit *int, offset *int) error {
	if value := strings.TrimSpace(values.Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return ErrInvalidQuery
		}
		*limit = parsed
	}
	if value := strings.TrimSpace(values.Get("offset")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return ErrInvalidQuery
		}
		*offset = parsed
	}
	return nil
}
