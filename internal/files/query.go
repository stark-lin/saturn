// This file defines Files pagination and immutable file filters.
package files

const (
	DefaultLimit = 25
	MaxLimit     = 100
)

type CollectionQuery struct {
	Limit  int
	Offset int
}

type FileQuery struct {
	CollectionRefCode string
	Tag               string
	Limit             int
	Offset            int
}

type CollectionPage struct {
	Collections []Collection
	Limit       int
	Offset      int
	HasMore     bool
}

type FilePage struct {
	Files   []File
	Limit   int
	Offset  int
	HasMore bool
}
