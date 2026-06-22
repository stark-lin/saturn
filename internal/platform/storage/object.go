// This file defines object storage metadata tracked by Saturn.
package storage

import "time"

type Object struct {
	Key       string
	Path      string
	Size      int64
	SHA256    string
	BLAKE3    string
	CreatedAt time.Time
}
