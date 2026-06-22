// This file contains Files file behavior helpers.
package files

func (f File) Empty() bool {
	return f.ID == 0 && f.OriginalName == ""
}
