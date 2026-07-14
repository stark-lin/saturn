// This file defines Notes HTTP request and response payloads.
package notes

import "time"

type CreateNoteRequest struct {
	Markdown string `json:"markdown"`
}

type UpdateNoteRequest struct {
	Markdown string `json:"markdown"`
}

type NoteDetail struct {
	RefCode                 string           `json:"ref_code"`
	CurrentVersionRef       string           `json:"current_version_ref"`
	CurrentVersionNumber    int64            `json:"version_number"`
	CurrentVersionOperation VersionOperation `json:"operation"`
	Title                   string           `json:"title"`
	Markdown                string           `json:"markdown"`
	ContentType             string           `json:"content_type"`
	Tags                    []string         `json:"tags"`
	Status                  NoteStatus       `json:"status"`
	CreatedAt               time.Time        `json:"created_at"`
	UpdatedAt               time.Time        `json:"updated_at"`
}

type NoteSummary struct {
	RefCode              string     `json:"ref_code"`
	CurrentVersionRef    string     `json:"current_version_ref"`
	CurrentVersionNumber int64      `json:"version_number"`
	Title                string     `json:"title"`
	Tags                 []string   `json:"tags"`
	Status               NoteStatus `json:"status"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type VersionDetail struct {
	RefCode          string           `json:"ref_code"`
	NoteRefCode      string           `json:"nte_ref"`
	ParentVersionRef string           `json:"parent_version_ref,omitempty"`
	VersionNumber    int64            `json:"version_number"`
	Title            string           `json:"title"`
	Content          string           `json:"content"`
	ContentType      string           `json:"content_type"`
	Operation        VersionOperation `json:"operation"`
	Tags             []string         `json:"tags"`
	CreatedAt        time.Time        `json:"created_at"`
}

type VersionSummary struct {
	RefCode          string           `json:"ref_code"`
	NoteRefCode      string           `json:"nte_ref"`
	ParentVersionRef string           `json:"parent_version_ref,omitempty"`
	VersionNumber    int64            `json:"version_number"`
	Title            string           `json:"title"`
	ContentType      string           `json:"content_type"`
	Operation        VersionOperation `json:"operation"`
	Tags             []string         `json:"tags"`
	CreatedAt        time.Time        `json:"created_at"`
}

type NoteResponse struct {
	Note NoteDetail `json:"note"`
}

type VersionResponse struct {
	Version VersionDetail `json:"version"`
}

type VersionsResponse struct {
	Versions []VersionSummary `json:"versions"`
}

type NotesResponse struct {
	Notes      []NoteSummary `json:"notes"`
	Pagination Pagination    `json:"pagination"`
}

type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

func detailResponse(note Note) NoteResponse {
	return NoteResponse{Note: NoteDetail{
		RefCode:                 note.RefCode,
		CurrentVersionRef:       note.CurrentVersionRef,
		CurrentVersionNumber:    note.CurrentVersionNumber,
		CurrentVersionOperation: note.CurrentVersionOperation,
		Title:                   note.Title,
		Markdown:                note.Markdown,
		ContentType:             note.ContentType,
		Tags:                    nonNilTags(note.Tags),
		Status:                  note.Status,
		CreatedAt:               note.CreatedAt,
		UpdatedAt:               note.UpdatedAt,
	}}
}

func versionResponse(version Version) VersionResponse {
	return VersionResponse{Version: VersionDetail{
		RefCode: version.RefCode, NoteRefCode: version.NoteRefCode, ParentVersionRef: version.ParentVersionRef,
		VersionNumber: version.VersionNumber, Title: version.Title, Content: version.Content,
		ContentType: version.ContentType, Operation: version.Operation, Tags: nonNilTags(version.Tags), CreatedAt: version.CreatedAt,
	}}
}

func versionsResponse(versions []Version) VersionsResponse {
	summaries := make([]VersionSummary, 0, len(versions))
	for _, version := range versions {
		summaries = append(summaries, VersionSummary{
			RefCode: version.RefCode, NoteRefCode: version.NoteRefCode, ParentVersionRef: version.ParentVersionRef,
			VersionNumber: version.VersionNumber, Title: version.Title, ContentType: version.ContentType,
			Operation: version.Operation, Tags: nonNilTags(version.Tags), CreatedAt: version.CreatedAt,
		})
	}
	return VersionsResponse{Versions: summaries}
}

func summariesResponse(page Page) NotesResponse {
	summaries := make([]NoteSummary, 0, len(page.Notes))
	for _, note := range page.Notes {
		summaries = append(summaries, NoteSummary{
			RefCode:              note.RefCode,
			CurrentVersionRef:    note.CurrentVersionRef,
			CurrentVersionNumber: note.CurrentVersionNumber,
			Title:                note.Title,
			Tags:                 nonNilTags(note.Tags),
			Status:               note.Status,
			UpdatedAt:            note.UpdatedAt,
		})
	}
	return NotesResponse{
		Notes: summaries,
		Pagination: Pagination{
			Limit:   page.Limit,
			Offset:  page.Offset,
			HasMore: page.HasMore,
		},
	}
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
