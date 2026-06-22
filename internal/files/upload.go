// This file defines Files upload workflow data.
package files

type UploadRequest struct {
	OriginalName      string
	CollectionRefCode string
	Size              int64
}
