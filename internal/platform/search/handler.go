// This file exposes the owner-only reference metadata endpoint for global search.
package search

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/httpx"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

type ReferenceMetadataResolver interface {
	ResolveMetadata(ctx context.Context, actor auth.Principal, code string) (ref.Metadata, error)
	ListRecentMetadata(ctx context.Context, actor auth.Principal, limit int) ([]ref.Metadata, error)
	SearchMetadata(ctx context.Context, actor auth.Principal, query ref.MetadataSearchQuery) ([]ref.Metadata, error)
}

type Handler struct {
	references ReferenceMetadataResolver
}

func NewHandler(references ReferenceMetadataResolver) *Handler {
	return &Handler{references: references}
}

func (h *Handler) Metadata(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("ref_code"))
	if code == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "ref_code is required")
		return
	}
	metadata, err := h.references.ResolveMetadata(r.Context(), principal, code)
	if errors.Is(err, ref.ErrInvalidCode) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return
	}
	if errors.Is(err, ref.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "search_unavailable", "Search is unavailable")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, metadata)
}

func (h *Handler) ObjectRefMetadata(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	code := ref.NormalizeCode(r.PathValue("ref_code"))
	if !ref.ValidCode(code) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return
	}
	metadata, err := h.references.ResolveMetadata(r.Context(), principal, code)
	if errors.Is(err, ref.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Resource not found")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "object_refs_unavailable", "Object refs are unavailable")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, metadata)
}

func (h *Handler) SearchObjectRefs(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	var request objectRefSearchRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid object ref search request")
		return
	}
	query, err := objectRefSearchQuery(request)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid object ref search request")
		return
	}
	objects, err := h.references.SearchMetadata(r.Context(), principal, query)
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	case errors.Is(err, ref.ErrInvalidMetadataSearchQuery):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid object ref search request")
		return
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "object_refs_unavailable", "Object refs are unavailable")
		return
	}
	if objects == nil {
		objects = []ref.Metadata{}
	}
	httpx.WriteJSON(w, http.StatusOK, objects)
}

func (h *Handler) RecentObjects(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	limit, err := bindRecentMetadataLimit(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid recent objects query")
		return
	}
	objects, err := h.references.ListRecentMetadata(r.Context(), principal, limit)
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	case errors.Is(err, ref.ErrInvalidRecentMetadataLimit):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid recent objects query")
		return
	case err != nil:
		httpx.WriteError(w, http.StatusInternalServerError, "recent_objects_unavailable", "Recent objects are unavailable")
		return
	}
	if objects == nil {
		objects = []ref.Metadata{}
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Objects []ref.Metadata `json:"objects"`
		Limit   int            `json:"limit"`
	}{
		Objects: objects,
		Limit:   limit,
	})
}

func bindRecentMetadataLimit(values url.Values) (int, error) {
	for key := range values {
		if key != "limit" {
			return 0, errors.New("unsupported query parameter")
		}
	}
	limit := ref.DefaultRecentMetadataLimit
	rawLimits, ok := values["limit"]
	if !ok {
		return limit, nil
	}
	if len(rawLimits) != 1 || strings.TrimSpace(rawLimits[0]) == "" {
		return 0, errors.New("invalid limit")
	}
	var err error
	limit, err = strconv.Atoi(strings.TrimSpace(rawLimits[0]))
	if err != nil {
		return 0, errors.New("invalid limit")
	}
	if limit < 1 || limit > ref.MaxRecentMetadataLimit {
		return 0, errors.New("invalid limit")
	}
	return limit, nil
}

type objectRefSearchRequest struct {
	Modules     []ref.Module        `json:"modules"`
	ObjectTypes []ref.ObjectType    `json:"object_types"`
	Statuses    []string            `json:"statuses"`
	Tags        []string            `json:"tags"`
	CreatedAt   objectRefTimeRange  `json:"created_at"`
	UpdatedAt   objectRefTimeRange  `json:"updated_at"`
	Sort        objectRefSearchSort `json:"sort"`
	Limit       int                 `json:"limit"`
}

type objectRefTimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type objectRefSearchSort struct {
	Field     ref.MetadataSearchSortField     `json:"field"`
	Direction ref.MetadataSearchSortDirection `json:"direction"`
}

func objectRefSearchQuery(request objectRefSearchRequest) (ref.MetadataSearchQuery, error) {
	createdFrom, err := parseOptionalSearchTime(request.CreatedAt.From)
	if err != nil {
		return ref.MetadataSearchQuery{}, err
	}
	createdTo, err := parseOptionalSearchTime(request.CreatedAt.To)
	if err != nil {
		return ref.MetadataSearchQuery{}, err
	}
	updatedFrom, err := parseOptionalSearchTime(request.UpdatedAt.From)
	if err != nil {
		return ref.MetadataSearchQuery{}, err
	}
	updatedTo, err := parseOptionalSearchTime(request.UpdatedAt.To)
	if err != nil {
		return ref.MetadataSearchQuery{}, err
	}
	return ref.MetadataSearchQuery{
		Modules:     request.Modules,
		ObjectTypes: request.ObjectTypes,
		Statuses:    request.Statuses,
		Tags:        request.Tags,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		UpdatedFrom: updatedFrom,
		UpdatedTo:   updatedTo,
		Sort: ref.MetadataSearchSort{
			Field:     request.Sort.Field,
			Direction: request.Sort.Direction,
		},
		Limit: request.Limit,
	}, nil
}

func parseOptionalSearchTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
