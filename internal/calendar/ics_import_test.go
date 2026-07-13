// This file tests iCalendar parsing and recurrence expansion.
package calendar

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseICSImportExpandsRecurrenceDatesExclusionsAndOverrides(t *testing.T) {
	input := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Saturn Test//EN
X-WR-TIMEZONE:Australia/Sydney
BEGIN:VEVENT
UID:team-calendar
DTSTAMP:20260701T000000Z
DTSTART;TZID=Australia/Sydney:20260706T090000
DTEND;TZID=Australia/Sydney:20260706T100000
RRULE:FREQ=WEEKLY;COUNT=6;BYDAY=MO,WE
RDATE;TZID=Australia/Sydney:20260710T090000
EXDATE;TZID=Australia/Sydney:20260713T090000
SUMMARY:Team planning
DESCRIPTION:Weekly planning
LOCATION:Room A
END:VEVENT
BEGIN:VEVENT
UID:team-calendar
DTSTAMP:20260701T000000Z
RECURRENCE-ID;TZID=Australia/Sydney:20260708T090000
DTSTART;TZID=Australia/Sydney:20260708T110000
DTEND;TZID=Australia/Sydney:20260708T123000
SUMMARY:Updated planning
LOCATION:Room B
END:VEVENT
BEGIN:VEVENT
UID:team-calendar
DTSTAMP:20260701T000000Z
RECURRENCE-ID;TZID=Australia/Sydney:20260715T090000
STATUS:CANCELLED
END:VEVENT
END:VCALENDAR
`

	parsed, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
	if err != nil {
		t.Fatalf("parse ICS import: %v", err)
	}
	if parsed.Timezone != "Australia/Sydney" {
		t.Fatalf("timezone = %q, want Australia/Sydney", parsed.Timezone)
	}
	if len(parsed.Events) != 5 {
		t.Fatalf("event count = %d, want 5: %#v", len(parsed.Events), parsed.Events)
	}
	wantStarts := []string{
		"2026-07-06T09:00:00+10:00",
		"2026-07-08T11:00:00+10:00",
		"2026-07-10T09:00:00+10:00",
		"2026-07-20T09:00:00+10:00",
		"2026-07-22T09:00:00+10:00",
	}
	for index, event := range parsed.Events {
		if got := event.StartsAt.Format(time.RFC3339); got != wantStarts[index] {
			t.Fatalf("event %d starts_at = %s, want %s", index, got, wantStarts[index])
		}
		if event.Recurrence.Kind != RecurrenceKindNone {
			t.Fatalf("event %d recurrence = %#v, want concrete event", index, event.Recurrence)
		}
	}
	if parsed.Events[1].Metadata.Title != "Updated planning" ||
		parsed.Events[1].Metadata.Description != "Weekly planning" ||
		parsed.Events[1].Metadata.Location != "Room B" ||
		parsed.Events[1].EndsAt.Format(time.RFC3339) != "2026-07-08T12:30:00+10:00" {
		t.Fatalf("overridden event = %#v", parsed.Events[1])
	}
}

func TestParseICSImportUsesRFCMonthDayExpansion(t *testing.T) {
	input := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Saturn Test//EN
BEGIN:VEVENT
UID:month-end
DTSTAMP:20260101T000000Z
DTSTART:20260131T090000Z
DTEND:20260131T100000Z
RRULE:FREQ=MONTHLY;COUNT=3;BYMONTHDAY=31
SUMMARY:Month end
END:VEVENT
END:VCALENDAR
`

	parsed, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
	if err != nil {
		t.Fatalf("parse ICS import: %v", err)
	}
	want := []string{"2026-01-31", "2026-03-31", "2026-05-31"}
	if len(parsed.Events) != len(want) {
		t.Fatalf("event count = %d, want %d", len(parsed.Events), len(want))
	}
	for index, event := range parsed.Events {
		if got := event.StartsAt.Format(time.DateOnly); got != want[index] {
			t.Fatalf("event %d date = %s, want %s", index, got, want[index])
		}
	}
}

func TestParseICSImportAppliesThisAndFutureOverride(t *testing.T) {
	input := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Saturn Test//EN
BEGIN:VEVENT
UID:daily-review
DTSTAMP:20260701T000000Z
DTSTART:20260701T090000Z
DTEND:20260701T100000Z
RRULE:FREQ=DAILY;COUNT=4
SUMMARY:Daily review
LOCATION:Room A
END:VEVENT
BEGIN:VEVENT
UID:daily-review
DTSTAMP:20260701T000000Z
RECURRENCE-ID;RANGE=THISANDFUTURE:20260702T090000Z
DTSTART:20260702T110000Z
DTEND:20260702T123000Z
LOCATION:Room B
END:VEVENT
BEGIN:VEVENT
UID:daily-review
DTSTAMP:20260701T000000Z
RECURRENCE-ID:20260703T090000Z
DTSTART:20260703T150000Z
DTEND:20260703T160000Z
SUMMARY:Special review
END:VEVENT
END:VCALENDAR
`

	parsed, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
	if err != nil {
		t.Fatalf("parse ICS import: %v", err)
	}
	wantStarts := []string{
		"2026-07-01T09:00:00Z",
		"2026-07-02T11:00:00Z",
		"2026-07-03T15:00:00Z",
		"2026-07-04T11:00:00Z",
	}
	wantEnds := []string{
		"2026-07-01T10:00:00Z",
		"2026-07-02T12:30:00Z",
		"2026-07-03T16:00:00Z",
		"2026-07-04T12:30:00Z",
	}
	for index, event := range parsed.Events {
		if event.StartsAt.Format(time.RFC3339) != wantStarts[index] || event.EndsAt.Format(time.RFC3339) != wantEnds[index] {
			t.Fatalf("event %d = %s to %s, want %s to %s", index, event.StartsAt, event.EndsAt, wantStarts[index], wantEnds[index])
		}
	}
	if parsed.Events[3].Metadata.Location != "Room B" || parsed.Events[2].Metadata.Title != "Special review" {
		t.Fatalf("override metadata = %#v", parsed.Events)
	}
}

func TestParseICSImportSupportsAllDayAndRDatePeriodDurations(t *testing.T) {
	input := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Saturn Test//EN
BEGIN:VEVENT
UID:all-day
DTSTAMP:20260701T000000Z
DTSTART;VALUE=DATE:20260705
SUMMARY:All day
END:VEVENT
BEGIN:VEVENT
UID:periods
DTSTAMP:20260701T000000Z
DTSTART:20260706T090000Z
DURATION:PT1H
RDATE;VALUE=PERIOD:20260707T090000Z/PT2H
SUMMARY:Timed
END:VEVENT
END:VCALENDAR
`

	parsed, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
	if err != nil {
		t.Fatalf("parse ICS import: %v", err)
	}
	if len(parsed.Events) != 3 {
		t.Fatalf("event count = %d, want 3", len(parsed.Events))
	}
	if parsed.Events[0].EndsAt.Sub(parsed.Events[0].StartsAt) != 24*time.Hour {
		t.Fatalf("all-day event = %s to %s", parsed.Events[0].StartsAt, parsed.Events[0].EndsAt)
	}
	if parsed.Events[2].EndsAt.Sub(parsed.Events[2].StartsAt) != 2*time.Hour {
		t.Fatalf("RDATE period = %s to %s", parsed.Events[2].StartsAt, parsed.Events[2].EndsAt)
	}
}

func TestParseICSImportPreservesUnsupportedEventContentInDescription(t *testing.T) {
	input := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Saturn Test//EN
BEGIN:VEVENT
UID:unsupported-content
DTSTAMP:20260701T000000Z
DTSTART:20260706T090000Z
DTEND:20260706T100000Z
SUMMARY;LANGUAGE=en:Planning
DESCRIPTION:Agenda
ORGANIZER;CN=Alice:mailto:alice@example.com
ATTENDEE;ROLE=REQ-PARTICIPANT;CN=Bob:mailto:bob@example.com
X-SATURN-COLOR:blue
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:Reminder
END:VALARM
END:VEVENT
END:VCALENDAR
`

	parsed, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
	if err != nil {
		t.Fatalf("parse ICS import: %v", err)
	}
	if len(parsed.Events) != 1 {
		t.Fatalf("event count = %d, want 1", len(parsed.Events))
	}
	wantDescription := `Agenda

Unsupported iCalendar content:
SUMMARY;LANGUAGE=en:Planning
ORGANIZER;CN=Alice:mailto:alice@example.com
ATTENDEE;CN=Bob;ROLE=REQ-PARTICIPANT:mailto:bob@example.com
X-SATURN-COLOR:blue
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:Reminder
END:VALARM`
	if got := parsed.Events[0].Metadata.Description; got != wantDescription {
		t.Fatalf("description = %q, want %q", got, wantDescription)
	}
}

func TestParseICSImportPreservesMasterAndOverrideUnsupportedContent(t *testing.T) {
	input := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Saturn Test//EN
BEGIN:VEVENT
UID:unsupported-override
DTSTAMP:20260701T000000Z
DTSTART:20260706T090000Z
DTEND:20260706T100000Z
RRULE:FREQ=DAILY;COUNT=2
SUMMARY:Planning
DESCRIPTION:Master notes
ORGANIZER:mailto:alice@example.com
END:VEVENT
BEGIN:VEVENT
UID:unsupported-override
DTSTAMP:20260701T000000Z
RECURRENCE-ID:20260707T090000Z
DESCRIPTION:Override notes
COMMENT:Bring documents
END:VEVENT
END:VCALENDAR
`

	parsed, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
	if err != nil {
		t.Fatalf("parse ICS import: %v", err)
	}
	if len(parsed.Events) != 2 {
		t.Fatalf("event count = %d, want 2", len(parsed.Events))
	}
	wantFirst := "Master notes\n\nUnsupported iCalendar content:\nORGANIZER:mailto:alice@example.com"
	wantSecond := "Override notes\n\nUnsupported iCalendar content:\nORGANIZER:mailto:alice@example.com\nCOMMENT:Bring documents"
	if parsed.Events[0].Metadata.Description != wantFirst {
		t.Fatalf("first description = %q, want %q", parsed.Events[0].Metadata.Description, wantFirst)
	}
	if parsed.Events[1].Metadata.Description != wantSecond {
		t.Fatalf("second description = %q, want %q", parsed.Events[1].Metadata.Description, wantSecond)
	}
}

func TestParseICSImportAcceptsPublishedCalendarWithRedundantUTCTimezones(t *testing.T) {
	input := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//GenericGenerator//TestSchedule//EN
CALSCALE:GREGORIAN
METHOD:PUBLISH
X-WR-CALNAME:Dummy Calendar Name
X-WR-RELCALID:dummy-id-fake@domain.com
BEGIN:VTIMEZONE
TZID:Custom/Timezone
X-LIC-LOCATION:Custom/Loc
BEGIN:STANDARD
TZOFFSETFROM:+9900
TZOFFSETTO:+8900
TZNAME:DST
DTSTART:19700405T030000
RRULE:FREQ=DAILY;BYMONTH=4;BYDAY=1SU
END:STANDARD
BEGIN:DAYLIGHT
TZOFFSETFROM:+8900
TZOFFSETTO:+9900
TZNAME:DST2
DTSTART:19701004T020000
RRULE:FREQ=DAILY;BYMONTH=10;BYDAY=1SU
END:DAYLIGHT
END:VTIMEZONE
BEGIN:VEVENT
UID:dummy-uid-event-one@domain.com
DTSTAMP:20991231T100000Z
SUMMARY:DUMMY EVENT TITLE
DESCRIPTION:Event Description Placeholder Text Here
COMMENT:Marker to show last update timestamp
DTSTART;TZID=UTC:20991231T090000Z
DTEND;TZID=UTC:20991231T090000Z
CATEGORIES:AcadCalendar
SEQUENCE:0
STATUS:CONFIRMED
TRANSP:OPAQUE
END:VEVENT
BEGIN:VEVENT
UID:dummy-uid-event-two@domain.com
DTSTAMP:20991231T100000Z
SUMMARY:DUMMY EVENT TITLE
DESCRIPTION:Event Description Placeholder Text Here
COMMENT:Class meeting placeholder
DTSTART;TZID=UTC:20991231T130000Z
DTEND;TZID=UTC:20991231T150000Z
RRULE:FREQ=WEEKLY;UNTIL=20991231T135900Z;BYDAY=WE
LOCATION:Generic Location
GEO:-00.0000000;000.0000000
CATEGORIES:Classes
SEQUENCE:0
STATUS:CONFIRMED
TRANSP:OPAQUE
END:VEVENT
END:VCALENDAR
`

	parsed, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
	if err != nil {
		t.Fatalf("parse ICS import: %v", err)
	}
	if len(parsed.Events) != 2 {
		t.Fatalf("event count = %d, want 2", len(parsed.Events))
	}
	if got := parsed.Events[0].EndsAt.Sub(parsed.Events[0].StartsAt); got != time.Second {
		t.Fatalf("zero-duration compatibility duration = %s, want 1s", got)
	}
	for _, content := range []string{
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"X-WR-CALNAME:Dummy Calendar Name",
		"BEGIN:VTIMEZONE",
		"TZID:Custom/Timezone",
		"END:VTIMEZONE",
	} {
		if !strings.Contains(parsed.Description, content) {
			t.Fatalf("aggregate description %q does not contain %q", parsed.Description, content)
		}
	}
	for _, content := range []string{
		"DTSTART;TZID=UTC:20991231T090000Z",
		"DTEND;TZID=UTC:20991231T090000Z",
		"COMMENT:Marker to show last update timestamp",
		"CATEGORIES:AcadCalendar",
		"STATUS:CONFIRMED",
	} {
		if !strings.Contains(parsed.Events[0].Metadata.Description, content) {
			t.Fatalf("event description %q does not contain %q", parsed.Events[0].Metadata.Description, content)
		}
	}
}

func TestParseICSImportRejectsUnboundedAndMoreThan512Events(t *testing.T) {
	for _, test := range []struct {
		name string
		rule string
	}{
		{name: "unbounded", rule: "FREQ=DAILY"},
		{name: "more than limit", rule: fmt.Sprintf("FREQ=DAILY;COUNT=%d", maxICSImportedEvents+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Saturn Test//EN
BEGIN:VEVENT
UID:bounded
DTSTAMP:20260701T000000Z
DTSTART:20260701T090000Z
DTEND:20260701T100000Z
RRULE:%s
SUMMARY:Bounded
END:VEVENT
END:VCALENDAR
`, test.rule)
			_, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
			if !errors.Is(err, ErrInvalidICSImport) {
				t.Fatalf("parse error = %v, want invalid ICS import", err)
			}
		})
	}
}

func TestParseICSImportAcceptsExactly512Events(t *testing.T) {
	input := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Saturn Test//EN
BEGIN:VEVENT
UID:at-limit
DTSTAMP:20260701T000000Z
DTSTART:20260701T090000Z
DTEND:20260701T100000Z
RRULE:FREQ=DAILY;COUNT=%d
SUMMARY:At limit
END:VEVENT
END:VCALENDAR
`, maxICSImportedEvents)

	parsed, err := parseICSImport(ImportEventAggregateInput{Title: "Imported", Body: strings.NewReader(input)})
	if err != nil {
		t.Fatalf("parse ICS import: %v", err)
	}
	if len(parsed.Events) != maxICSImportedEvents {
		t.Fatalf("event count = %d, want %d", len(parsed.Events), maxICSImportedEvents)
	}
}
