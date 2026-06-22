// This file contains Notes note behavior helpers.
package notes

func (n Note) IsDraft() bool {
	return n.Status == NoteDraft
}
