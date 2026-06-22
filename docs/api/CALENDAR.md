# Calendar API

## 1. Ownership

Calendar owns the HTTP contracts for event aggregates, specific schedule events, the main calendar view, and event completion/voiding.

```text
Path prefix: /api/calendar
Module: internal/calendar
Common rules: ../API.md
```

---

## 2. Current Status

`Implemented`. The `/api/calendar` routes are registered in `internal/app/routes.go`; the current implementation includes EventAggregate, Event, the main CalendarView, Event finish / void, and the aggregate deletion closed loop.

## 3. Endpoint Inventory

| Method | Path | Authentication | Status | Purpose |
| --- | --- | --- | --- | --- |
| `GET` | `/api/calendar/view` | Authenticated | `Implemented` | Read the main calendar view, returning only scheduled events |
| `GET` | `/api/calendar/aggregates` | Authenticated | `Implemented` | List event aggregates |
| `POST` | `/api/calendar/aggregates` | Authenticated | `Implemented` | Create a nullable event aggregate |
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

Event only uses a start time and a duration:

```text
starts_at          RFC3339 timestamp
duration_minutes  Positive integer minutes
```

Event does not design an `ends_at`; the client can use `starts_at + duration_minutes` to calculate the display end time.

## 5. Request Contract

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
  "duration_minutes": 60,
  "recurrence": {
    "kind": "weekly",
    "weekdays": ["mon", "wed"],
    "week_count": 2
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
| `duration_minutes` | Yes | Positive integer minutes |
| `recurrence.kind` | No | `single` or `weekly`; defaults to `single` |
| `recurrence.weekdays` | Weekly only | `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun`, allows multiple selections, duplicates will be removed |
| `recurrence.week_count` | Weekly only | Positive integer weeks; the current service layer limits the max to 520 |

Recurrence rules:

```text
single: Only generates 1 Event with the start time being starts_at.
weekly: Using Monday of the week containing starts_at as the starting point, expands according to weekdays over week_count weeks.
The first week of weekly recurrence will not generate events earlier than starts_at.
Duplicate events are allowed; there is no uniqueness constraint on the same owner, same start time, and same title.
```

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
      "duration_minutes": 60,
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

When creating an empty aggregate, `events` returns an empty array. `POST /api/calendar/aggregates/{ref_code}/events`
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
    "duration_minutes": 60,
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
EventAggregate and Event tags each hang onto their own object_ref_id.
Event does not provide a delete endpoint; status transitions only allow scheduled -> finished, scheduled -> voided, finished -> voided.
finished / voided Events do not enter the main CalendarView.
finished / voided Events are still displayed in the EventAggregate's child event list.
Deleting an EventAggregate is an aggregate-level deletion, allowing the cascaded deletion of its Events.
Creating an EventAggregate, registering the ObjectRef, writing tags, and the SUCCESS audit must all be committed in the same transaction.
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
           When creating an EventAggregate, the new aggregate always belongs to the creating actor;
           When creating an Event under an existing aggregate, the new Event belongs to the owner of that aggregate.
```

Inaccessible resources and non-existent resources both manifest externally as:

```http
HTTP 404
{"error":{"code":"not_found","message":"Calendar resource not found"}}
```

| Status | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Invalid creation request JSON/unknown fields, query, `ref_code`, dates, duration, weekday, or enumerated values |
| `401` | `unauthorized` | Unauthenticated or missing authenticated Principal |
| `404` | `not_found` | EventAggregate / Event does not exist, or the current actor has no access rights |
| `409` | `conflict` | Finishing an Event that is already `finished` / `voided`, or voiding an Event that is already `voided` |
| `500` | `calendar_unavailable` | Calendar dependencies or internal operations failed |
