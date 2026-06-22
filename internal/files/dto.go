// This file defines Files HTTP request and response payloads.
package files

import "time"

type CreateCollectionRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type CollectionDetail struct {
	RefCode     string    `json:"ref_code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FileMetadataDetail struct {
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	BLAKE3       string `json:"blake3"`
}

type FileDetail struct {
	RefCode           string             `json:"ref_code"`
	CollectionRefCode string             `json:"collection_ref_code"`
	OriginalName      string             `json:"original_name"`
	MimeType          string             `json:"mime_type"`
	SizeBytes         int64              `json:"size_bytes"`
	SHA256            string             `json:"sha256"`
	BLAKE3            string             `json:"blake3"`
	Status            string             `json:"status"`
	Tags              []string           `json:"tags"`
	Metadata          FileMetadataDetail `json:"metadata"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

type CollectionResponse struct {
	Collection CollectionDetail `json:"collection"`
}

type CollectionsResponse struct {
	Collections []CollectionDetail `json:"collections"`
	Pagination  Pagination         `json:"pagination"`
}

type FileResponse struct {
	File FileDetail `json:"file"`
}

type FilesResponse struct {
	Files      []FileDetail `json:"files"`
	Pagination Pagination   `json:"pagination"`
}

func collectionResponse(collection Collection) CollectionResponse {
	return CollectionResponse{Collection: collectionDetail(collection)}
}

func collectionsResponse(page CollectionPage) CollectionsResponse {
	collections := make([]CollectionDetail, 0, len(page.Collections))
	for _, collection := range page.Collections {
		collections = append(collections, collectionDetail(collection))
	}
	return CollectionsResponse{
		Collections: collections,
		Pagination:  Pagination{Limit: page.Limit, Offset: page.Offset, HasMore: page.HasMore},
	}
}

func collectionDetail(collection Collection) CollectionDetail {
	tags := collection.Tags
	if tags == nil {
		tags = []string{}
	}
	return CollectionDetail{
		RefCode: collection.RefCode, Name: collection.Name, Description: collection.Description,
		Status: collection.Status, Tags: tags, CreatedAt: collection.CreatedAt, UpdatedAt: collection.UpdatedAt,
	}
}

func fileResponse(file File) FileResponse {
	return FileResponse{File: fileDetail(file)}
}

func filesResponse(page FilePage) FilesResponse {
	files := make([]FileDetail, 0, len(page.Files))
	for _, file := range page.Files {
		files = append(files, fileDetail(file))
	}
	return FilesResponse{
		Files:      files,
		Pagination: Pagination{Limit: page.Limit, Offset: page.Offset, HasMore: page.HasMore},
	}
}

func fileDetail(file File) FileDetail {
	tags := file.Tags
	if tags == nil {
		tags = []string{}
	}
	metadata := FileMetadataDetail{
		OriginalName: file.OriginalName, MimeType: file.MimeType, SizeBytes: file.SizeBytes,
		SHA256: file.SHA256, BLAKE3: file.BLAKE3,
	}
	return FileDetail{
		RefCode: file.RefCode, CollectionRefCode: file.CollectionRefCode,
		OriginalName: file.OriginalName, MimeType: file.MimeType, SizeBytes: file.SizeBytes,
		SHA256: file.SHA256, BLAKE3: file.BLAKE3, Status: file.Status, Tags: tags, Metadata: metadata,
		CreatedAt: file.CreatedAt, UpdatedAt: file.UpdatedAt,
	}
}
