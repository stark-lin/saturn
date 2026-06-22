// This file defines resource facts passed into authorization decisions.
package auth

type Resource struct {
	Type    string
	ID      int64
	OwnerID int64
}
