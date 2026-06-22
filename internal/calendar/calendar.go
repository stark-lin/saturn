// This file contains Calendar view behavior helpers.
package calendar

func (v CalendarView) Empty() bool {
	return len(v.Events) == 0
}
