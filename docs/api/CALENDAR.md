# Calendar API

## 1. Ownership

Calendar owns the HTTP contracts for event aggregates, synchronous ICS aggregate import, specific schedule events, the main calendar view, and event completion/voiding.

```text
Path prefix: /api/calendar
Module: internal/calendar
Common rules: ../API.md
```

---

## 2. Current Status

`Implemented`. The `/api/calendar` routes are registered in `internal/app/routes.go`; the current implementation includes EventAggregate, synchronous ICS import, Event, the main CalendarView, Event finish / void, and the aggregate deletion closed loop.

## 3. Endpoint Inventory

| Method | Path | Authentication | Status | Purpose |
| --- | --- | --- | --- | --- |
| `GET` | `/api/calendar/view` | Authenticated | `Implemented` | Read the main calendar view, returning only scheduled events |
| `GET` | `/api/calendar/aggregates` | Authenticated | `Implemented` | List event aggregates |
| `POST` | `/api/calendar/aggregates` | Authenticated | `Implemented` | Create a nullable event aggregate |
| `POST` | `/api/calendar/aggregates/import-ics` | Authenticated | `Implemented` | Create an event aggregate and its concrete events from an uploaded ICS file |
| `GET` | `/api/calendar/aggregates/{ref_code}` | Authenticated | `Implemented` | Read aggregate details and all child events |
| `DELETE` | `/api/calendar/aggregates/{ref_code}` | Authenticated | `Implemented` | Delete an entire event aggregate and its child events |
| `POST` | `/api/calendar/aggregates/{ref_code}/events` | Authenticated | `Implemented` | Create specific events under a designated aggregate |
| `GET` | `/api/calendar/events/{ref_code}` | Authenticated | `Implemented` | Read a single event |
| `POST` | `/api/calendar/events/{ref_code}/finish` | Authenticated | `Implemented` | Mark a scheduled event as finished |
| `POST` | `/api/calendar/events/{ref_code}/void` | Authenticated | `Implemented` | Void a single event |

---

## 4. Models and Reference Codes

```text
EventAggregate is a collection of events, similar to an Accounting Account.
Event          is a specific small event that must belong to an EventAggregate, similar to an Accounting Transaction.
CalendarView   is the main calendar view query model.
```

Common ID and metadata rules:

```text
EventAggregate is registered with object_type=event_aggregate, ref_code=CAL-*.
Event is registered with object_type=event, ref_code=CAL-*.
Both EventAggregate and Event must save object_ref_id, and expose ref_code via the corresponding ObjectRef.
Both EventAggregate and Event support tags; tags are written to their respective `object_refs.tags`.
EventAggregate.metadata is immutable after creation.
Event.metadata is immutable after creation.
EventAggregate can be created empty; an Event must be created within an EventAggregate scope.
Events are not allowed to be deleted, only scheduled -> finished, scheduled -> voided, finished -> voided are allowed.
EventAggregates are allowed to be deleted; deleting an aggregate cascades the deletion of its child events and corresponding object refs/tags.
```

Statuses:

```text
EventAggregate.status: active, only stored in the ObjectRef projection
Event.status: scheduled | finished | voided
```

Event uses explicit start and end timestamps:

```text
starts_at  RFC3339 timestamp
ends_at    RFC3339 timestamp, later than starts_at
```

The server stores both timestamps. Clients do not calculate an event end from a duration.

## 5. Request Contract

### 5.1 JSON Creation

Create an event aggregate:

```json
{
  "metadata": {
    "title": "Training",
    "description": "Weekly training block",
    "location": "Gym",
    "timezone": "Australia/Sydney"
  },
  "tags": ["health"]
}
```

Create an event under a specific aggregate:

```http
POST /api/calendar/aggregates/CAL-00000001/events
```

```json
{
  "metadata": {
    "title": "Training session",
    "description": "Strength",
    "location": "Gym"
  },
  "tags": ["workout"],
  "starts_at": "2026-06-01T09:00:00Z",
  "ends_at": "2026-06-01T10:00:00Z",
  "recurrence": {
    "kind": "week",
    "count": 2
  }
}
```

Field rules:

| Field | Required | Rule |
| --- | --- | --- |
| `metadata.title` | Yes | Must not be empty after trimming; acts as the ObjectRef title projection for the EventAggregate |
| `metadata.description` | No | Saved after trimming; immutable after creation |
| `metadata.location` | No | Saved after trimming; immutable after creation |
| `metadata.timezone` | No | Saved after trimming; immutable after creation |
| `tags` | No | Trimmed, empty values removed, and deduplicated before associating with the EventAggregate |

Event creation field rules:

| Field | Required | Rule |
| --- | --- | --- |
| `metadata.title` | Yes | Must not be empty after trimming; acts as the ObjectRef title projection for the Event |
| `metadata.description` | No | Saved after trimming; immutable after creation |
| `metadata.location` | No | Saved after trimming; immutable after creation |
| `tags` | No | Trimmed, empty values removed, and deduplicated before associating with each generated Event |
| `starts_at` | Yes | RFC3339 timestamp; the handler also accepts `YYYY-MM-DD` as midnight time |
| `ends_at` | Yes | RFC3339 timestamp; the handler also accepts `YYYY-MM-DD` as midnight time; must be later than `starts_at` |
| `recurrence.kind` | No | `none`, `week`, `month`, or `year`; defaults to `none` |
| `recurrence.count` | Repeating kinds only | Total number of generated Events including the first Event; integer range `1..520`; for `none`, omit it or use `1` |

Recurrence rules:

```text
none: Generates exactly 1 Event starting at starts_at.
week: Generates recurrence.count Events on the same weekday and clock time as starts_at, with each Event starting 7 days after the previous one. Count includes the first Event.
month: Generates recurrence.count Events on the same calendar day and clock time in successive months. If the template day does not exist in a target month, that instance uses the target month's last day; each occurrence is calculated from the original template, so January 31 produces January 31, February 28/29, and March 31.
year: Generates recurrence.count Events on the same calendar date and clock time in successive years. February 29 uses February 28 in non-leap years and returns to February 29 in later leap years.
Each repeating instance copies the submitted end clock and calendar-day offset onto its own start date. For example, a same-day 09:00-10:00 template produces 09:00-10:00 on every generated date; an overnight 23:00-01:00 template ends at 01:00 on the following date for every instance.
Duplicate events are allowed; there is no uniqueness constraint on the same owner, same start time, and same title.
```

### 5.2 ICS Aggregate Import

```http
POST /api/calendar/aggregates/import-ics
Content-Type: multipart/form-data
```

Multipart fields:

| Field | Required | Rule |
| --- | --- | --- |
| `title` | Yes | Non-empty after trimming; becomes the immutable EventAggregate title and ObjectRef title projection |
| `file` | Yes | One textual iCalendar file, at most 1 MiB |

No other multipart value or file fields are accepted. The uploaded source file is parsed synchronously and is not retained after import.

VEVENT mapping:

```text
SUMMARY     -> Event.metadata.title; required for every effective occurrence
DESCRIPTION -> Event.metadata.description
LOCATION    -> Event.metadata.location
DTSTART     -> Event.starts_at
DTEND or DURATION -> Event.ends_at
```

Unsupported VCALENDAR properties and non-VEVENT components are preserved in `EventAggregate.metadata.description` under an `Unsupported iCalendar content:` block. Structural `VERSION` / `PRODID` and the supported calendar timezone property are not copied. This preserves content such as `CALSCALE`, `METHOD`, `X-WR-*`, and an unused custom `VTIMEZONE` without treating it as Calendar business data.

VEVENT properties that are not mapped to Event fields or recurrence semantics are preserved in `Event.metadata.description` under the same heading. The block contains normalized ICS content lines in source order; unsupported parameters on otherwise supported properties and nested components such as `VALARM` are also preserved. Structural `UID` and `DTSTAMP` properties are not copied. Master content is inherited by expanded occurrences, while override content is added only to the occurrences affected by that override.

An all-day `DTSTART;VALUE=DATE` without `DTEND` or `DURATION` is treated as one calendar day. A date-time event must normally provide `DTEND` or a positive `DURATION`. When `DTEND` exactly equals `DTSTART`, import represents the point event with a one-second duration, or one calendar day for an all-day event, and preserves the original `DTEND` in the Event description. Earlier end times remain invalid. `DTEND` remains exclusive for all-day events. Imported Events have empty tags and `scheduled` status.

Time and timezone rules:

```text
UTC values ending in Z are preserved as absolute timestamps. If a value redundantly combines TZID with a trailing Z, Z takes precedence and the original property is preserved in the Event description.
TZID values must resolve through the server's IANA timezone database.
X-WR-TIMEZONE or TIMEZONE-ID supplies the default location for floating values and is projected to aggregate metadata.timezone.
If no calendar default or TZID is supplied, floating values are interpreted as UTC.
Custom VTIMEZONE identifiers that cannot be resolved as IANA timezone names are rejected.
```

Recurrence import rules:

```text
VEVENTs are grouped by UID. Each UID has one master VEVENT and may have RECURRENCE-ID overrides.
The master DTSTART, one optional RRULE, RDATE values, and EXDATE values form the recurrence set.
RRULE supports RFC frequencies and modifiers understood by the fixed recurrence dependency, including INTERVAL, COUNT, UNTIL, BYDAY, BYMONTHDAY, BYMONTH, BYYEARDAY, BYWEEKNO, BYHOUR, BYMINUTE, BYSECOND, BYSETPOS, and WKST.
An RRULE must be finite: it must have exactly one of COUNT or UNTIL.
RDATE can add date/date-time occurrences or PERIOD values with an occurrence-specific end/duration.
EXDATE removes matching occurrences, including DTSTART.
RECURRENCE-ID replaces one occurrence; STATUS:CANCELLED removes it.
RECURRENCE-ID;RANGE=THISANDFUTURE applies its time shift, duration, metadata changes, or cancellation to that and later occurrences.
Every override must match an occurrence from the recurrence set; ambiguous masters or duplicate overrides are rejected.
```

The importer expands the complete recurrence set into concrete, non-recurring Events before writing. The final upload must produce `1..512` Events; a 513th Event rejects the entire import and no truncated result is saved. This RFC recurrence expansion is independent from the JSON creation endpoint's `week` / `month` / `year` expansion semantics.

List queries:

```text
GET /api/calendar/view:       from, to, limit, offset
GET /api/calendar/aggregates: limit, offset
```

| Parameter | Rule |
| --- | --- |
| `from`, `to` | Required; RFC3339 timestamp or `YYYY-MM-DD`; `from <= to` |
| `limit` | Defaults to `25`, range `1..100` |
| `offset` | Defaults to `0`, must be a non-negative integer |

List endpoints reject undefined query parameters. `/view` only returns `scheduled` events; `finished` and `voided` events are only displayed in the child event list of aggregate details and single event details.

## 6. Response Contract

Creating an EventAggregate returns `HTTP 201`, and sets:

```http
Location: /api/calendar/aggregates/CAL-00000001
```

Create and read aggregate responses:

```json
{
  "aggregate": {
    "ref_code": "CAL-00000001",
    "metadata": {
      "title": "Training",
      "description": "Weekly training block",
      "location": "Gym",
      "timezone": "Australia/Sydney"
    },
    "tags": ["health"],
    "created_at": "2026-06-01T00:00:00Z"
  },
  "events": [
    {
      "ref_code": "CAL-00000002",
      "aggregate_ref_code": "CAL-00000001",
      "starts_at": "2026-06-01T09:00:00Z",
      "ends_at": "2026-06-01T10:00:00Z",
      "metadata": {
        "title": "Training session",
        "description": "Strength",
        "location": "Gym"
      },
      "status": "scheduled",
      "tags": ["workout"],
      "created_at": "2026-06-01T00:00:00Z",
      "updated_at": "2026-06-01T00:00:00Z"
    }
  ]
}
```

When creating an empty aggregate, `events` returns an empty array. ICS import returns the created aggregate and all imported concrete events in the same structure. `POST /api/calendar/aggregates/{ref_code}/events`
returns the same response structure, where `aggregate` is the parent aggregate, and `events` is the list of events generated by this creation; if the client needs the full child event list, they can subsequently read the aggregate details.

Main view response:

```json
{
  "from": "2026-06-01T00:00:00Z",
  "to": "2026-06-30T23:59:59Z",
  "events": [],
  "pagination": {
    "limit": 25,
    "offset": 0,
    "has_more": false
  }
}
```

Read, finish, and void responses for a single Event:

```json
{
  "event": {
    "ref_code": "CAL-00000002",
    "aggregate_ref_code": "CAL-00000001",
    "starts_at": "2026-06-01T09:00:00Z",
    "ends_at": "2026-06-01T10:00:00Z",
    "metadata": {
      "title": "Training session",
      "description": "Strength",
      "location": "Gym"
    },
    "status": "voided",
    "tags": ["workout"],
    "created_at": "2026-06-01T00:00:00Z",
    "updated_at": "2026-06-01T00:00:00Z"
  }
}
```

Deleting an EventAggregate successfully returns `HTTP 204` without a JSON body.

## 7. Invariants and Transactions

```text
EventAggregates can be created empty, similar to Accounting Accounts.
Every Event must belong to an EventAggregate.
Event creation must specify the parent aggregate via `/api/calendar/aggregates/{ref_code}/events`, similar to Accounting Transactions which must specify an Account.
EventAggregate metadata is immutable after creation.
Event metadata is immutable after creation.
Event starts_at and ends_at are immutable after creation, and ends_at must be later than starts_at.
EventAggregate and Event tags each hang onto their own object_ref_id.
Event does not provide a delete endpoint; status transitions only allow scheduled -> finished, scheduled -> voided, finished -> voided.
finished / voided Events do not enter the main CalendarView.
finished / voided Events are still displayed in the EventAggregate's child event list.
Deleting an EventAggregate is an aggregate-level deletion, allowing the cascaded deletion of its Events.
Creating an EventAggregate, registering the ObjectRef, writing tags, and the SUCCESS audit must all be committed in the same transaction.
Importing ICS parses, expands, and validates the complete upload before opening the business transaction.
ICS aggregate import creates the EventAggregate, every concrete Event, all ObjectRefs/status projections, and all SUCCESS audits in one transaction.
Creating Events under an EventAggregate, registering Event ObjectRefs, writing Event tags, and the SUCCESS audit for each Event must all be committed in the same transaction.
finish / void Event, ObjectRef status projection updates, and the SUCCESS audit must be committed in the same transaction.
Deleting an EventAggregate, deleting child Event/ObjectRef/tag associations, the DELETE audit for each child Event, and the aggregate DELETE audit must all be committed in the same transaction.
If a write operation fails or is rejected within the transaction, no SUCCESS audit is retained; a FAILED or DENIED audit is recorded after the outcome is finalized.
```

## 8. Permissions and Errors

All endpoints require a Bearer JWT. Resource authorization is executed in the Calendar service, and the repo only applies fixed scope queries:

```text
user:      Can only list, read, and write EventAggregates / Events they own.
superuser: Can list, read, create Events for, finish, void, or delete existing EventAggregates / Events of any owner;
           When creating or importing an EventAggregate, the new aggregate always belongs to the creating actor;
           When creating an Event under an existing aggregate, the new Event belongs to the owner of that aggregate.
```

Inaccessible resources and non-existent resources both manifest externally as:

```http
HTTP 404
{"error":{"code":"not_found","message":"Calendar resource not found"}}
```

| Status | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Invalid creation JSON/multipart/ICS, unknown fields, file size, query, `ref_code`, timestamps or timestamp order, timezone, recurrence rules/overrides, recurrence kind/count, or imported Event count |
| `401` | `unauthorized` | Unauthenticated or missing authenticated Principal |
| `404` | `not_found` | EventAggregate / Event does not exist, or the current actor has no access rights |
| `409` | `conflict` | Finishing an Event that is already `finished` / `voided`, or voiding an Event that is already `voided` |
| `500` | `calendar_unavailable` | Calendar dependencies or internal operations failed |
