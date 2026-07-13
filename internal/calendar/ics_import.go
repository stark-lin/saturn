// This file parses iCalendar uploads into concrete Calendar events.
package calendar

import (
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"
)

const (
	MaxICSFileBytes            int64 = 1 << 20
	maxICSImportedEvents             = 512
	maxICSEvaluatedOccurrences       = 65536
)

var (
	ErrInvalidICSImport = errors.New("invalid ICS import")
	icsDurationPattern  = regexp.MustCompile(`^\+?P(?:(\d+)W|(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?)$`)
)

type ImportEventAggregateInput struct {
	Title string
	Body  io.Reader
}

type parsedICSImport struct {
	Timezone string
	Events   []CreateEventInput
}

type icsEventGroup struct {
	Master    *ics.VEvent
	Overrides []*ics.VEvent
}

type icsTimeValue struct {
	Time   time.Time
	AllDay bool
}

type icsOccurrenceDuration struct {
	CalendarDays int
	Elapsed      time.Duration
}

type icsOccurrence struct {
	OriginalStart time.Time
	StartsAt      time.Time
	Duration      icsOccurrenceDuration
	Metadata      EventMetadata
}

type icsRDate struct {
	Start    icsTimeValue
	Duration *icsOccurrenceDuration
}

type icsStringPatch struct {
	Value string
	Set   bool
}

type icsMetadataPatch struct {
	Title       icsStringPatch
	Description icsStringPatch
	Location    icsStringPatch
}

type icsOccurrenceOverride struct {
	RecurrenceID icsTimeValue
	FutureRange  bool
	Cancelled    bool
	Start        *icsTimeValue
	Duration     *icsOccurrenceDuration
	Metadata     icsMetadataPatch
	Matched      bool
}

func parseICSImport(input ImportEventAggregateInput) (parsedICSImport, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" || input.Body == nil {
		return parsedICSImport{}, ErrInvalidICSImport
	}

	limited := &io.LimitedReader{R: input.Body, N: MaxICSFileBytes + 1}
	calendar, err := ics.ParseCalendar(limited)
	if err != nil || limited.N <= 0 {
		return parsedICSImport{}, ErrInvalidICSImport
	}

	defaultLocation, timezoneName, err := icsCalendarLocation(calendar)
	if err != nil {
		return parsedICSImport{}, ErrInvalidICSImport
	}
	groups, err := groupICSEvents(calendar.Events())
	if err != nil {
		return parsedICSImport{}, ErrInvalidICSImport
	}

	events := make([]CreateEventInput, 0)
	inferredTimezones := make(map[string]struct{})
	for _, group := range groups {
		groupEvents, inferredTimezone, err := expandICSEventGroup(group, defaultLocation)
		if err != nil {
			return parsedICSImport{}, ErrInvalidICSImport
		}
		if inferredTimezone != "" {
			inferredTimezones[inferredTimezone] = struct{}{}
		}
		if len(events)+len(groupEvents) > maxICSImportedEvents {
			return parsedICSImport{}, ErrInvalidICSImport
		}
		events = append(events, groupEvents...)
	}
	if len(events) == 0 {
		return parsedICSImport{}, ErrInvalidICSImport
	}
	if timezoneName == "" && len(inferredTimezones) == 1 {
		for name := range inferredTimezones {
			timezoneName = name
		}
	}

	sort.SliceStable(events, func(i int, j int) bool {
		if events[i].StartsAt.Equal(events[j].StartsAt) {
			return events[i].Metadata.Title < events[j].Metadata.Title
		}
		return events[i].StartsAt.Before(events[j].StartsAt)
	})
	return parsedICSImport{Timezone: timezoneName, Events: events}, nil
}

func icsCalendarLocation(calendar *ics.Calendar) (*time.Location, string, error) {
	for _, property := range calendar.CalendarProperties {
		if property.IANAToken != string(ics.PropertyXWRTimezone) && property.IANAToken != string(ics.PropertyTimezoneId) {
			continue
		}
		name := strings.TrimSpace(property.Value)
		if name == "" {
			continue
		}
		location, err := time.LoadLocation(name)
		if err != nil {
			return nil, "", err
		}
		return location, name, nil
	}
	return time.UTC, "", nil
}

func groupICSEvents(events []*ics.VEvent) ([]icsEventGroup, error) {
	order := make([]string, 0)
	groups := make(map[string]*icsEventGroup)
	for _, event := range events {
		uid := strings.TrimSpace(event.Id())
		if uid == "" {
			return nil, ErrInvalidICSImport
		}
		group, exists := groups[uid]
		if !exists {
			group = &icsEventGroup{}
			groups[uid] = group
			order = append(order, uid)
		}
		if event.GetProperty(ics.ComponentPropertyRecurrenceId) == nil {
			if group.Master != nil {
				return nil, ErrInvalidICSImport
			}
			group.Master = event
			continue
		}
		group.Overrides = append(group.Overrides, event)
	}

	result := make([]icsEventGroup, 0, len(order))
	for _, uid := range order {
		group := groups[uid]
		if group.Master == nil {
			return nil, ErrInvalidICSImport
		}
		result = append(result, *group)
	}
	return result, nil
}

func expandICSEventGroup(group icsEventGroup, defaultLocation *time.Location) ([]CreateEventInput, string, error) {
	if icsEventCancelled(group.Master) {
		return []CreateEventInput{}, "", nil
	}
	masterStart, err := parseRequiredICSTime(group.Master.GetProperty(ics.ComponentPropertyDtStart), defaultLocation)
	if err != nil {
		return nil, "", err
	}
	masterDuration, present, err := parseICSEventDuration(group.Master, masterStart, defaultLocation)
	if err != nil {
		return nil, "", err
	}
	if !present {
		if !masterStart.AllDay {
			return nil, "", ErrInvalidICSImport
		}
		masterDuration = icsOccurrenceDuration{CalendarDays: 1}
	}
	masterMetadata, err := requiredICSEventMetadata(group.Master)
	if err != nil {
		return nil, "", err
	}

	overrides, err := parseICSOverrides(group.Overrides, masterStart.AllDay, defaultLocation)
	if err != nil {
		return nil, "", err
	}
	recurrenceSet, rdateDurations, err := buildICSRecurrenceSet(group.Master, masterStart, defaultLocation)
	if err != nil {
		return nil, "", err
	}

	iterator := recurrenceSet.Iterator()
	events := make([]CreateEventInput, 0)
	evaluated := 0
	for {
		originalStart, ok := iterator()
		if !ok {
			break
		}
		evaluated++
		if evaluated > maxICSEvaluatedOccurrences {
			return nil, "", ErrInvalidICSImport
		}
		occurrence := icsOccurrence{
			OriginalStart: originalStart,
			StartsAt:      originalStart,
			Duration:      masterDuration,
			Metadata:      masterMetadata,
		}
		if duration, exists := rdateDurations[icsOccurrenceKey(originalStart)]; exists {
			occurrence.Duration = duration
		}
		occurrence, cancelled, err := applyICSOverrides(occurrence, overrides)
		if err != nil {
			return nil, "", err
		}
		if cancelled {
			continue
		}
		endsAt := occurrence.Duration.EndAt(occurrence.StartsAt)
		if occurrence.Metadata.Title == "" || !endsAt.After(occurrence.StartsAt) {
			return nil, "", ErrInvalidICSImport
		}
		events = append(events, CreateEventInput{
			Metadata: occurrence.Metadata,
			StartsAt: occurrence.StartsAt,
			EndsAt:   endsAt,
			Recurrence: RecurrenceInput{
				Kind: RecurrenceKindNone,
			},
		})
		if len(events) > maxICSImportedEvents {
			return nil, "", ErrInvalidICSImport
		}
	}
	for _, override := range overrides {
		if !override.Matched {
			return nil, "", ErrInvalidICSImport
		}
	}

	return events, icsPropertyTimezone(group.Master.GetProperty(ics.ComponentPropertyDtStart)), nil
}

func buildICSRecurrenceSet(
	event *ics.VEvent,
	masterStart icsTimeValue,
	defaultLocation *time.Location,
) (*rrule.Set, map[int64]icsOccurrenceDuration, error) {
	set := &rrule.Set{}
	set.DTStart(masterStart.Time)
	set.RDate(masterStart.Time)

	ruleProperties := event.GetProperties(ics.ComponentPropertyRrule)
	if len(ruleProperties) > 1 {
		return nil, nil, ErrInvalidICSImport
	}
	if len(ruleProperties) == 1 {
		option, err := rrule.StrToROptionInLocation(ruleProperties[0].Value, masterStart.Time.Location())
		if err != nil || option.Count == 0 && option.Until.IsZero() || option.Count != 0 && !option.Until.IsZero() {
			return nil, nil, ErrInvalidICSImport
		}
		option.Dtstart = masterStart.Time
		rule, err := rrule.NewRRule(*option)
		if err != nil {
			return nil, nil, ErrInvalidICSImport
		}
		set.RRule(rule)
	}

	rdates, err := parseICSRDates(event.GetProperties(ics.ComponentPropertyRdate), masterStart.AllDay, defaultLocation)
	if err != nil {
		return nil, nil, err
	}
	rdateDurations := make(map[int64]icsOccurrenceDuration)
	for _, rdate := range rdates {
		set.RDate(rdate.Start.Time)
		if rdate.Duration == nil {
			continue
		}
		key := icsOccurrenceKey(rdate.Start.Time)
		if _, exists := rdateDurations[key]; exists {
			return nil, nil, ErrInvalidICSImport
		}
		rdateDurations[key] = *rdate.Duration
	}

	exdates, err := parseICSTimeList(event.GetProperties(ics.ComponentPropertyExdate), masterStart.AllDay, defaultLocation)
	if err != nil {
		return nil, nil, err
	}
	for _, exdate := range exdates {
		set.ExDate(exdate.Time)
	}
	return set, rdateDurations, nil
}

func parseICSOverrides(events []*ics.VEvent, masterAllDay bool, defaultLocation *time.Location) ([]*icsOccurrenceOverride, error) {
	overrides := make([]*icsOccurrenceOverride, 0, len(events))
	seen := make(map[int64]struct{})
	for _, event := range events {
		recurrenceProperty := event.GetProperty(ics.ComponentPropertyRecurrenceId)
		recurrenceID, err := parseRequiredICSTime(recurrenceProperty, defaultLocation)
		if err != nil || recurrenceID.AllDay != masterAllDay {
			return nil, ErrInvalidICSImport
		}
		key := icsOccurrenceKey(recurrenceID.Time)
		if _, exists := seen[key]; exists {
			return nil, ErrInvalidICSImport
		}
		seen[key] = struct{}{}

		override := &icsOccurrenceOverride{
			RecurrenceID: recurrenceID,
			Cancelled:    icsEventCancelled(event),
			Metadata:     icsEventMetadataPatch(event),
		}
		rangeValue, err := icsSingleParameter(recurrenceProperty, string(ics.ParameterRange))
		if err != nil {
			return nil, ErrInvalidICSImport
		}
		if rangeValue != "" {
			if !strings.EqualFold(rangeValue, "THISANDFUTURE") {
				return nil, ErrInvalidICSImport
			}
			override.FutureRange = true
		}
		if event.GetProperty(ics.ComponentPropertyDtStart) != nil {
			start, err := parseRequiredICSTime(event.GetProperty(ics.ComponentPropertyDtStart), defaultLocation)
			if err != nil || start.AllDay != masterAllDay {
				return nil, ErrInvalidICSImport
			}
			override.Start = &start
		}
		durationStart := recurrenceID
		if override.Start != nil {
			durationStart = *override.Start
		}
		duration, present, err := parseICSEventDuration(event, durationStart, defaultLocation)
		if err != nil {
			return nil, err
		}
		if present {
			override.Duration = &duration
		}
		if len(event.GetProperties(ics.ComponentPropertyRrule)) != 0 ||
			len(event.GetProperties(ics.ComponentPropertyRdate)) != 0 ||
			len(event.GetProperties(ics.ComponentPropertyExdate)) != 0 {
			return nil, ErrInvalidICSImport
		}
		overrides = append(overrides, override)
	}
	sort.Slice(overrides, func(i int, j int) bool {
		return overrides[i].RecurrenceID.Time.Before(overrides[j].RecurrenceID.Time)
	})
	return overrides, nil
}

func applyICSOverrides(occurrence icsOccurrence, overrides []*icsOccurrenceOverride) (icsOccurrence, bool, error) {
	cancelled := false
	for _, override := range overrides {
		comparison := occurrence.OriginalStart.Compare(override.RecurrenceID.Time)
		if comparison == 0 {
			override.Matched = true
		}
		if override.FutureRange {
			if comparison < 0 {
				continue
			}
			if override.Start != nil {
				occurrence.StartsAt = occurrence.OriginalStart.Add(override.Start.Time.Sub(override.RecurrenceID.Time))
			}
			if override.Duration != nil {
				occurrence.Duration = *override.Duration
			}
			occurrence.Metadata = applyICSMetadataPatch(occurrence.Metadata, override.Metadata)
			cancelled = override.Cancelled
			continue
		}
		if comparison != 0 {
			continue
		}
		if override.Start != nil {
			occurrence.StartsAt = override.Start.Time
		}
		if override.Duration != nil {
			occurrence.Duration = *override.Duration
		}
		occurrence.Metadata = applyICSMetadataPatch(occurrence.Metadata, override.Metadata)
		cancelled = override.Cancelled
	}
	return occurrence, cancelled, nil
}

func parseICSRDates(properties []*ics.IANAProperty, masterAllDay bool, defaultLocation *time.Location) ([]icsRDate, error) {
	result := make([]icsRDate, 0)
	for _, property := range properties {
		valueType, err := icsSingleParameter(property, string(ics.ParameterValue))
		if err != nil {
			return nil, err
		}
		for _, value := range strings.Split(property.Value, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if strings.EqualFold(valueType, string(ics.ValueDataTypePeriod)) || strings.Contains(value, "/") {
				if masterAllDay {
					return nil, ErrInvalidICSImport
				}
				period, err := parseICSRDatePeriod(property, value, defaultLocation)
				if err != nil {
					return nil, err
				}
				result = append(result, period)
				continue
			}
			start, err := parseICSTimeValue(property, value, defaultLocation)
			if err != nil || start.AllDay != masterAllDay {
				return nil, ErrInvalidICSImport
			}
			result = append(result, icsRDate{Start: start})
		}
	}
	return result, nil
}

func parseICSRDatePeriod(property *ics.IANAProperty, value string, defaultLocation *time.Location) (icsRDate, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return icsRDate{}, ErrInvalidICSImport
	}
	start, err := parseICSTimeValue(property, parts[0], defaultLocation)
	if err != nil || start.AllDay {
		return icsRDate{}, ErrInvalidICSImport
	}
	var duration icsOccurrenceDuration
	if strings.HasPrefix(parts[1], "P") || strings.HasPrefix(parts[1], "+P") || strings.HasPrefix(parts[1], "-P") {
		duration, err = parseICSDuration(parts[1])
	} else {
		var end icsTimeValue
		end, err = parseICSTimeValue(property, parts[1], defaultLocation)
		if err == nil && !end.AllDay && end.Time.After(start.Time) {
			duration = icsOccurrenceDuration{Elapsed: end.Time.Sub(start.Time)}
		} else if err == nil {
			err = ErrInvalidICSImport
		}
	}
	if err != nil {
		return icsRDate{}, ErrInvalidICSImport
	}
	return icsRDate{Start: start, Duration: &duration}, nil
}

func parseICSTimeList(properties []*ics.IANAProperty, masterAllDay bool, defaultLocation *time.Location) ([]icsTimeValue, error) {
	result := make([]icsTimeValue, 0)
	for _, property := range properties {
		for _, value := range strings.Split(property.Value, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			parsed, err := parseICSTimeValue(property, value, defaultLocation)
			if err != nil || parsed.AllDay != masterAllDay {
				return nil, ErrInvalidICSImport
			}
			result = append(result, parsed)
		}
	}
	return result, nil
}

func parseRequiredICSTime(property *ics.IANAProperty, defaultLocation *time.Location) (icsTimeValue, error) {
	if property == nil || strings.TrimSpace(property.Value) == "" {
		return icsTimeValue{}, ErrInvalidICSImport
	}
	return parseICSTimeValue(property, property.Value, defaultLocation)
}

func parseICSTimeValue(property *ics.IANAProperty, value string, defaultLocation *time.Location) (icsTimeValue, error) {
	value = strings.TrimSpace(value)
	valueType, err := icsSingleParameter(property, string(ics.ParameterValue))
	if err != nil {
		return icsTimeValue{}, err
	}
	allDay := strings.EqualFold(valueType, string(ics.ValueDataTypeDate)) || len(value) == len("20060102")
	timezone, err := icsSingleParameter(property, string(ics.ParameterTzid))
	if err != nil {
		return icsTimeValue{}, err
	}
	location := defaultLocation
	if location == nil {
		location = time.UTC
	}
	if timezone != "" {
		if strings.HasSuffix(value, "Z") {
			return icsTimeValue{}, ErrInvalidICSImport
		}
		location, err = time.LoadLocation(timezone)
		if err != nil {
			return icsTimeValue{}, ErrInvalidICSImport
		}
	}

	var parsed time.Time
	switch {
	case allDay && len(value) == len("20060102"):
		parsed, err = time.ParseInLocation("20060102", value, location)
	case !allDay && strings.HasSuffix(value, "Z"):
		parsed, err = time.Parse("20060102T150405Z", value)
	case !allDay:
		parsed, err = time.ParseInLocation("20060102T150405", value, location)
	default:
		err = ErrInvalidICSImport
	}
	if err != nil {
		return icsTimeValue{}, ErrInvalidICSImport
	}
	return icsTimeValue{Time: parsed.Truncate(time.Second), AllDay: allDay}, nil
}

func parseICSEventDuration(event *ics.VEvent, start icsTimeValue, defaultLocation *time.Location) (icsOccurrenceDuration, bool, error) {
	endProperty := event.GetProperty(ics.ComponentPropertyDtEnd)
	durationProperty := event.GetProperty(ics.ComponentPropertyDuration)
	if endProperty != nil && durationProperty != nil {
		return icsOccurrenceDuration{}, false, ErrInvalidICSImport
	}
	if endProperty != nil {
		end, err := parseRequiredICSTime(endProperty, defaultLocation)
		if err != nil || end.AllDay != start.AllDay || !end.Time.After(start.Time) {
			return icsOccurrenceDuration{}, false, ErrInvalidICSImport
		}
		if start.AllDay {
			days := icsCalendarDayDifference(start.Time, end.Time)
			if days < 1 {
				return icsOccurrenceDuration{}, false, ErrInvalidICSImport
			}
			return icsOccurrenceDuration{CalendarDays: days}, true, nil
		}
		return icsOccurrenceDuration{Elapsed: end.Time.Sub(start.Time)}, true, nil
	}
	if durationProperty != nil {
		duration, err := parseICSDuration(durationProperty.Value)
		if err != nil || start.AllDay && duration.Elapsed != 0 {
			return icsOccurrenceDuration{}, false, ErrInvalidICSImport
		}
		return duration, true, nil
	}
	return icsOccurrenceDuration{}, false, nil
}

func parseICSDuration(value string) (icsOccurrenceDuration, error) {
	if strings.HasPrefix(value, "-") {
		return icsOccurrenceDuration{}, ErrInvalidICSImport
	}
	matches := icsDurationPattern.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return icsOccurrenceDuration{}, ErrInvalidICSImport
	}
	values := make([]int64, 5)
	hasValue := false
	for index := 1; index < len(matches); index++ {
		if matches[index] == "" {
			continue
		}
		parsed, err := strconv.ParseInt(matches[index], 10, 64)
		if err != nil {
			return icsOccurrenceDuration{}, ErrInvalidICSImport
		}
		values[index-1] = parsed
		hasValue = true
	}
	if !hasValue {
		return icsOccurrenceDuration{}, ErrInvalidICSImport
	}
	calendarDays, ok := safeICSCalendarDays(values[0], values[1])
	if !ok {
		return icsOccurrenceDuration{}, ErrInvalidICSImport
	}
	elapsed, ok := safeICSElapsed(values[2], values[3], values[4])
	if !ok || calendarDays == 0 && elapsed == 0 {
		return icsOccurrenceDuration{}, ErrInvalidICSImport
	}
	return icsOccurrenceDuration{CalendarDays: calendarDays, Elapsed: elapsed}, nil
}

func safeICSCalendarDays(weeks int64, days int64) (int, bool) {
	if weeks > (math.MaxInt64-days)/7 {
		return 0, false
	}
	total := weeks*7 + days
	if total > int64(math.MaxInt) {
		return 0, false
	}
	return int(total), true
}

func safeICSElapsed(hours int64, minutes int64, seconds int64) (time.Duration, bool) {
	if hours > math.MaxInt64/int64(time.Hour) {
		return 0, false
	}
	total := hours * int64(time.Hour)
	if minutes > (math.MaxInt64-total)/int64(time.Minute) {
		return 0, false
	}
	total += minutes * int64(time.Minute)
	if seconds > (math.MaxInt64-total)/int64(time.Second) {
		return 0, false
	}
	total += seconds * int64(time.Second)
	return time.Duration(total), true
}

func (duration icsOccurrenceDuration) EndAt(start time.Time) time.Time {
	return start.AddDate(0, 0, duration.CalendarDays).Add(duration.Elapsed)
}

func requiredICSEventMetadata(event *ics.VEvent) (EventMetadata, error) {
	metadata := EventMetadata{
		Title:       icsTextProperty(event, ics.ComponentPropertySummary),
		Description: icsTextProperty(event, ics.ComponentPropertyDescription),
		Location:    icsTextProperty(event, ics.ComponentPropertyLocation),
	}
	metadata = normalizeEventMetadata(metadata)
	if metadata.Title == "" {
		return EventMetadata{}, ErrInvalidICSImport
	}
	return metadata, nil
}

func icsEventMetadataPatch(event *ics.VEvent) icsMetadataPatch {
	return icsMetadataPatch{
		Title:       icsTextPropertyPatch(event, ics.ComponentPropertySummary),
		Description: icsTextPropertyPatch(event, ics.ComponentPropertyDescription),
		Location:    icsTextPropertyPatch(event, ics.ComponentPropertyLocation),
	}
}

func applyICSMetadataPatch(metadata EventMetadata, patch icsMetadataPatch) EventMetadata {
	if patch.Title.Set {
		metadata.Title = strings.TrimSpace(patch.Title.Value)
	}
	if patch.Description.Set {
		metadata.Description = strings.TrimSpace(patch.Description.Value)
	}
	if patch.Location.Set {
		metadata.Location = strings.TrimSpace(patch.Location.Value)
	}
	return metadata
}

func icsTextProperty(event *ics.VEvent, property ics.ComponentProperty) string {
	value := event.GetProperty(property)
	if value == nil {
		return ""
	}
	return value.Value
}

func icsTextPropertyPatch(event *ics.VEvent, property ics.ComponentProperty) icsStringPatch {
	value := event.GetProperty(property)
	if value == nil {
		return icsStringPatch{}
	}
	return icsStringPatch{Value: value.Value, Set: true}
}

func icsEventCancelled(event *ics.VEvent) bool {
	status := event.GetProperty(ics.ComponentPropertyStatus)
	return status != nil && strings.EqualFold(strings.TrimSpace(status.Value), string(ics.ObjectStatusCancelled))
}

func icsPropertyTimezone(property *ics.IANAProperty) string {
	value, err := icsSingleParameter(property, string(ics.ParameterTzid))
	if err != nil {
		return ""
	}
	return value
}

func icsSingleParameter(property *ics.IANAProperty, name string) (string, error) {
	if property == nil {
		return "", nil
	}
	values, exists := property.ICalParameters[name]
	if !exists || len(values) == 0 {
		return "", nil
	}
	if len(values) != 1 {
		return "", ErrInvalidICSImport
	}
	return strings.TrimSpace(values[0]), nil
}

func icsCalendarDayDifference(start time.Time, end time.Time) int {
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int(endDate.Sub(startDate) / (24 * time.Hour))
}

func icsOccurrenceKey(value time.Time) int64 {
	return value.Unix()
}

func wrapInvalidICSImport(err error) error {
	if err == nil || errors.Is(err, ErrInvalidICSImport) {
		return ErrInvalidICSImport
	}
	return fmt.Errorf("%w: %v", ErrInvalidICSImport, err)
}
