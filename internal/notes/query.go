// This file defines Notes list query parameters.
package notes

type Query struct {
	Text   string
	Tag    string
	Limit  int
	Offset int
}

const (
	DefaultLimit = 20
	MaxLimit     = 100
)
