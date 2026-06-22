// This file contains Calendar event behavior helpers.
package calendar

func (e Event) Empty() bool {
	return e.ID == 0 && e.Metadata.Title == ""
}
