// This file exposes authenticated owner-only Notes API handlers.
package notes

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/stark-lin/saturn/internal/platform/auth"
	"github.com/stark-lin/saturn/internal/platform/httpx"
	"github.com/stark-lin/saturn/internal/platform/ref"
)

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	query, err := bindListQuery(r.URL.Query())
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid notes query")
		return
	}
	page, err := h.service.ListNotes(r.Context(), principal, query)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, summariesResponse(page))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return
	}
	var request CreateNoteRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid note request")
		return
	}
	note, err := h.service.CreateNote(r.Context(), principal, request.Markdown)
	if h.writeServiceError(w, err) {
		return
	}
	w.Header().Set("Location", "/api/notes/"+note.RefCode)
	httpx.WriteJSON(w, http.StatusCreated, detailResponse(note))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := bindOwnedNoteRequest(w, r)
	if !ok {
		return
	}
	note, err := h.service.GetNote(r.Context(), principal, refCode)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detailResponse(note))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := bindOwnedNoteRequest(w, r)
	if !ok {
		return
	}
	var request UpdateNoteRequest
	if err := httpx.BindJSON(r, &request); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid note request")
		return
	}
	note, err := h.service.UpdateNote(r.Context(), principal, refCode, request.Markdown)
	if h.writeServiceError(w, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detailResponse(note))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	principal, refCode, ok := bindOwnedNoteRequest(w, r)
	if !ok {
		return
	}
	if h.writeServiceError(w, h.service.DeleteNote(r.Context(), principal, refCode)) {
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
	case errors.Is(err, ErrInvalidMarkdown):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_markdown", "Invalid note markdown")
	case errors.Is(err, ErrNoteNotFound), errors.Is(err, ref.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Note not found")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "notes_unavailable", "Notes service is unavailable")
	}
	return true
}

func bindOwnedNoteRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, string, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required")
		return auth.Principal{}, "", false
	}
	refCode := strings.TrimSpace(r.PathValue("ref_code"))
	if !ref.ValidCode(refCode) || !ref.CodeMatchesObjectType(refCode, ref.ObjectTypeNote) {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid ref_code")
		return auth.Principal{}, "", false
	}
	return principal, refCode, true
}

func bindListQuery(values url.Values) (Query, error) {
	for key := range values {
		switch key {
		case "text", "tag", "limit", "offset":
		default:
			return Query{}, errors.New("unsupported query parameter")
		}
	}
	query := Query{
		Text:  strings.TrimSpace(values.Get("text")),
		Tag:   strings.TrimSpace(values.Get("tag")),
		Limit: DefaultLimit,
	}
	var err error
	if value := values.Get("limit"); value != "" {
		query.Limit, err = strconv.Atoi(value)
		if err != nil || query.Limit < 1 || query.Limit > MaxLimit {
			return Query{}, errors.New("invalid limit")
		}
	}
	if value := values.Get("offset"); value != "" {
		query.Offset, err = strconv.Atoi(value)
		if err != nil || query.Offset < 0 {
			return Query{}, errors.New("invalid offset")
		}
	}
	return query, nil
}
