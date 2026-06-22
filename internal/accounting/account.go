// This file contains minimal Accounting account behavior helpers.
package accounting

func (a Account) Empty() bool {
	return a.ID == 0 && a.Name == ""
}
