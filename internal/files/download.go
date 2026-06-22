// This file defines Files download workflow data.
package files

type DownloadTicket struct {
	FileRefCode string
	SizeBytes   int64
	SHA256      string
	BLAKE3      string
}
