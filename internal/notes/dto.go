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
	RefCode   string     `json:"ref_code"`
	Title     string     `json:"title"`
	Markdown  string     `json:"markdown"`
	Tags      []string   `json:"tags"`
	Status    NoteStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type NoteSummary struct {
	RefCode   string     `json:"ref_code"`
	Title     string     `json:"title"`
	Tags      []string   `json:"tags"`
	Status    NoteStatus `json:"status"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type NoteResponse struct {
	Note NoteDetail `json:"note"`
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
		RefCode:   note.RefCode,
		Title:     note.Title,
		Markdown:  note.Markdown,
		Tags:      nonNilTags(note.Tags),
		Status:    note.Status,
		CreatedAt: note.CreatedAt,
		UpdatedAt: note.UpdatedAt,
	}}
}

func summariesResponse(page Page) NotesResponse {
	summaries := make([]NoteSummary, 0, len(page.Notes))
	for _, note := range page.Notes {
		summaries = append(summaries, NoteSummary{
			RefCode:   note.RefCode,
			Title:     note.Title,
			Tags:      nonNilTags(note.Tags),
			Status:    note.Status,
			UpdatedAt: note.UpdatedAt,
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
