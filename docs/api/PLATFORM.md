# Platform API

## 1. Ownership

Platform owns horizontal APIs such as authentication, object reference metadata lookup, storage diagnostics, audit logs, and shared SSE transport. It does not own the vertical business rules for Accounting, Notes, Files, Calendar, or LLM.

```text
Module: internal/platform
Common rules: ../API.md
```

---

## 2. Capability Paths and Statuses

| Capability | Path prefix | Current status |
| --- | --- | --- |
| Auth | `/api/auth` | `Implemented` |
| Events transport | `/api/events` | `Implemented` |
| Reference metadata | `/api/platform/object-refs`, `/api/platform/search`, `/api/platform/recent-objects` | `Implemented: owner-only metadata includes tags and JSON search filters` |
| Audit logs | `/api/platform/audit-logs` | `Implemented: superuser read-only queries` |
| ObjectRef | Internal registration and resolution service | `Implemented` |
| Storage | `/api/platform/storage` | `Planned` |

## 3. Endpoint Inventory

| Method | Path | Authentication | Status | Purpose |
| --- | --- | --- | --- | --- |
| `POST` | `/api/auth/login` | Public | `Implemented` | Create a JWT session using a username and password |
| `GET` | `/api/auth/me` | Bearer JWT | `Implemented` | Return the currently authenticated principal |
| `PATCH` | `/api/auth/me` | Bearer JWT | `Implemented` | Update the current user's editable account fields |
| `PATCH` | `/api/auth/me/password` | Bearer JWT | `Implemented` | Change the current user's password after verifying the current password |
| `POST` | `/api/auth/users` | Bearer JWT, superuser | `Implemented` | Create a user account |
| `PATCH` | `/api/auth/users/{id}/password` | Bearer JWT, superuser | `Implemented` | Reset a user account password |
| `POST` | `/api/auth/logout` | Bearer JWT | `Implemented` | Revoke the current JWT session |
| `GET` | `/api/events` | Bearer JWT | `Implemented` | Establish a shared SSE connection |
| `GET` | `/api/platform/object-refs/{ref_code}` | Bearer JWT | `Implemented` | Return owner-only metadata JSON containing `tags` by exact Object Ref Code |
| `POST` | `/api/platform/object-refs/search` | Bearer JWT | `Implemented` | Return current owner's object metadata JSON list by JSON query conditions |
| `GET` | `/api/platform/search?ref_code=<code>` | Bearer JWT | `Implemented` | Compatibility endpoint: Return owner-only metadata JSON containing `tags` by exact Object Ref Code |
| `GET` | `/api/platform/recent-objects?limit=<count>` | Bearer JWT | `Implemented` | Return the current owner's recently updated object metadata containing `tags` |
| `GET` | `/api/platform/audit-logs` | Bearer JWT, superuser | `Implemented` | Read-only query for append-only audit logs |

---

## 4. Auth

### 4.1 `POST /api/auth/login`

Request:

```json
{
  "username": "admin",
  "password": "admin"
}
```

Success: `HTTP 200`

```json
{
  "token": "<jwt>",
  "expires_at": "<RFC3339 expiration time>",
  "user": {
    "id": 1,
    "username": "admin",
    "role": "superuser"
  }
}
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Request JSON cannot be bound |
| `401` | `invalid_credentials` | Username or password invalid |
| `500` | `authentication_unavailable` | Authentication service unavailable |

A test account with username `admin`, password `admin`, and role `superuser` is idempotently created upon development schema startup. This default credential is only intended for local development and testing; credentials and the JWT secret must be replaced before deploying to a real environment.

### 4.2 `GET /api/auth/me`

Request header:

```http
Authorization: Bearer <token>
```

Success: `HTTP 200`

```json
{
  "user": {
    "id": 1,
    "username": "admin",
    "role": "superuser"
  }
}
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |

### 4.3 `PATCH /api/auth/me`

Request header:

```http
Authorization: Bearer <token>
```

Request:

```json
{
  "username": "admin2",
  "email": "admin@example.com"
}
```

Success: `HTTP 200`

```json
{
  "user": {
    "id": 1,
    "username": "admin2",
    "email": "admin@example.com",
    "role": "superuser"
  }
}
```

Rules:

```text
Only the authenticated user's own account can be updated through this endpoint.
username is optional, but when supplied it is trimmed, required to be non-empty, and must remain unique.
email is optional; an empty string clears the email to null, and a non-empty email must remain unique.
role and password cannot be changed through this endpoint.
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Request JSON or submitted account fields invalid |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `409` | `conflict` | username or email already exists |
| `500` | `authentication_unavailable` | Authentication service unavailable |

### 4.4 `PATCH /api/auth/me/password`

Request header:

```http
Authorization: Bearer <token>
```

Request:

```json
{
  "current_password": "admin",
  "new_password": "new-admin-password"
}
```

Success: `HTTP 200`

```json
{
  "password_updated": true
}
```

Rules:

```text
Any authenticated user can change their own password.
The current password must be verified before the new password hash is stored.
Passwords must be non-empty after trimming whitespace.
The request and response never expose password hashes.
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Request JSON invalid or new password empty |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `401` | `invalid_credentials` | current_password does not match the authenticated user |
| `500` | `authentication_unavailable` | Authentication service unavailable |

### 4.5 `POST /api/auth/users`

Request header:

```http
Authorization: Bearer <token>
```

Request:

```json
{
  "username": "alice",
  "email": "alice@example.com",
  "password": "initial-password",
  "role": "user"
}
```

Success: `HTTP 201`

```json
{
  "user": {
    "id": 2,
    "username": "alice",
    "email": "alice@example.com",
    "role": "user"
  }
}
```

Rules:

```text
Only a superuser can create ordinary user accounts.
username and password are required after trimming whitespace.
role is optional and defaults to user; the public API only accepts user.
email is optional; an empty string is stored as null, and a non-empty email must be unique.
The created password is stored only as a bcrypt hash.
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Request JSON or submitted account fields invalid, including role=superuser |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `403` | `forbidden` | authenticated actor is not a superuser |
| `409` | `conflict` | username or email already exists |
| `500` | `authentication_unavailable` | Authentication service unavailable |

### 4.6 `PATCH /api/auth/users/{id}/password`

Request header:

```http
Authorization: Bearer <token>
```

Request:

```json
{
  "password": "new-password"
}
```

Success: `HTTP 200`

```json
{
  "password_updated": true
}
```

Rules:

```text
Only a superuser can reset another account's password through this endpoint.
A superuser can reset their own password through this endpoint.
A superuser can reset user role accounts.
The public API does not create additional superusers; if legacy or manually modified data contains another superuser, resetting that account is rejected.
Passwords must be non-empty after trimming whitespace.
The request and response never expose password hashes.
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Request JSON invalid, path id invalid, or password empty |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `403` | `forbidden` | authenticated actor is not a superuser or targets another superuser from legacy/manually modified data |
| `404` | `not_found` | target user does not exist |
| `500` | `authentication_unavailable` | Authentication service unavailable |

### 4.7 `POST /api/auth/logout`

Request header:

```http
Authorization: Bearer <token>
```

Success: `HTTP 200`

```json
{
  "logged_out": true
}
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `500` | `authentication_unavailable` | Session revocation or audit write failed |

### 4.8 Auth Rules

```text
The JWT contains the subject user id, username, role, token id, and expiration.
Redis stores active JWT token ids, enabling logout to revoke the current token.
Middleware only authenticates and injects the Principal; resource-level authorization is still executed by each module's service.
Passwords, JWTs, and Authorization headers must not enter logs, audit metadata, or normal responses.
LOGIN and LOGOUT write to audit_logs with SYS-00000000 as the system target.
Account creation, account profile update, and password update write CREATE or UPDATE audit_logs with SYS-00000000 as the target and reason values that do not contain submitted credentials.
```

---

## 5. Events Transport

### 5.1 `GET /api/events`

Request header:

```http
Authorization: Bearer <token>
Accept: text/event-stream
```

Success headers:

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

After the connection is established, the server first writes an SSE comment:

```text
: connected
```

The wire format for named events; if the event name is unset, the `event:` line is omitted:

```text
event: <event-name>
data: <event-data>
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `500` | `sse_unsupported` | HTTP writer does not support streaming |
| `503` | `sse_closed` | Server event broker is closed |

This endpoint only defines the shared transport. Specific event names and JSON payloads generated by LLM or other implemented modules should be defined in the API documentation of the module that originates the behavior. Browser clients use a streaming `fetch` with a Bearer JWT header, instead of native `EventSource`.

---

## 6. Reference Metadata Queries

This section defines the unified ObjectRef metadata target contracts. Endpoints return basic fields such as `ref_code`, `title`, `tags`, and `status`; `tags` is a required response field.

### 6.1 `GET /api/platform/object-refs/{ref_code}`

This endpoint implements exact `ref_code` metadata resolution and does not return source business details, detail redirects, or index diagnostics. It is the RESTful single-resource read entry point for ObjectRef metadata.

Request header:

```http
Authorization: Bearer <token>
```

Success: `HTTP 200`

```json
{
  "ref_code": "NTE-00000001",
  "module": "notes",
  "object_type": "note",
  "title": "Release notes",
  "tags": ["backend", "release"],
  "status": "draft",
  "created_at": "2026-05-25T00:00:00Z",
  "updated_at": "2026-05-25T00:00:00Z"
}
```

Rules:

```text
Supports the three-letter module prefixes NTE / FIL / ACC / CAL / LLM and the eight-character uppercase Hex sequence number.
The path ref_code allows lowercase letters during resolution, but the response returns the normalized code.
The endpoint only returns metadata; it does not return source business records, internal object ids, or detail URLs.
title, tags, and status read the object_refs projection; tags are stored in object_refs.tags TEXT[].
All objects must output tags; returns [] when tagless, and retains the first-occurrence order after server-side normalization when tags exist.
Only the resource owner can obtain the metadata; status does not grant access permissions.
Non-owner superusers cannot read metadata through this entry point either.
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | `ref_code` missing or invalid format |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `404` | `not_found` | Reference does not exist, or the current actor is not the resource owner |
| `500` | `object_refs_unavailable` | Reference query service unavailable |

For compatibility, the old endpoint `GET /api/platform/search?ref_code=<code>` can still be used, with consistent response and permission semantics; the internal error code for the old endpoint remains `search_unavailable`. New clients should use `/api/platform/object-refs/{ref_code}`.

### 6.2 `POST /api/platform/object-refs/search`

This endpoint queries the currently authenticated owner's ObjectRef metadata collection using a JSON request body. It reads the `object_refs` metadata projection and does not read the source business body or details.

Request header:

```http
Authorization: Bearer <token>
Content-Type: application/json
```

Request:

```json
{
  "modules": ["notes", "files"],
  "object_types": ["note", "file"],
  "statuses": ["draft", "active"],
  "tags": ["backend", "release"],
  "created_at": {
    "from": "2026-05-01T00:00:00Z",
    "to": "2026-06-01T00:00:00Z"
  },
  "updated_at": {
    "from": "2026-05-01T00:00:00Z",
    "to": "2026-06-01T00:00:00Z"
  },
  "sort": {
    "field": "updated_at",
    "direction": "desc"
  },
  "limit": 50
}
```

Success: `HTTP 200`

```json
[
  {
    "ref_code": "NTE-00000001",
    "module": "notes",
    "object_type": "note",
    "title": "Release notes",
    "tags": ["backend", "release"],
    "status": "draft",
    "created_at": "2026-05-25T00:00:00Z",
    "updated_at": "2026-05-25T00:00:00Z"
  }
]
```

Query body fields:

| Field | Type | Default | Rule |
| --- | --- | --- | --- |
| `modules` | string array | empty | `notes/files/accounting/calendar/llm`; translates to `object_type in (...)` |
| `object_types` | string array | empty | `note/file_collection/file/event_aggregate/event/account/transaction/llm_session/llm_request` |
| `statuses` | string array | empty | exact `object_refs.status in (...)` |
| `tags` | string array | empty | object must contain all requested tags |
| `created_at.from` | RFC3339 string | empty | inclusive lower bound on `object_refs.created_at` |
| `created_at.to` | RFC3339 string | empty | exclusive upper bound on `object_refs.created_at` |
| `updated_at.from` | RFC3339 string | empty | inclusive lower bound on `object_refs.updated_at` |
| `updated_at.to` | RFC3339 string | empty | exclusive upper bound on `object_refs.updated_at` |
| `sort.field` | enum | `updated_at` | `created_at/updated_at/ref_code` |
| `sort.direction` | enum | `desc` | `asc/desc` |
| `limit` | integer | `50` | `1..100` |

Rules:

```text
Only returns metadata where owner_id = current actor.ID; status does not grant access permissions.
Non-owner superusers cannot read metadata through this entry point either.
When modules and object_types both exist, their intersection is taken; if the intersection is empty, it returns HTTP 200 and [].
Tag filtering uses all-tags semantics, meaning object_refs.tags must contain all tags in the request.
sort uses a fixed whitelist; both created_at and updated_at sorting use ref_code as the stable secondary sort.
An empty query body {} returns the current owner's recent metadata list (updated_at desc), up to 50 items.
The response is a JSON list without an extra envelope; returns [] when there are no results.
The response does not return source business content, owner_id, internal object ids, internal object-ref ids, or detail URLs.
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | JSON body cannot be bound, contains unknown fields, invalid enum values, invalid time formats, range from is later than to, or invalid limit |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `500` | `object_refs_unavailable` | metadata query service unavailable |

### 6.3 `GET /api/platform/recent-objects?limit=<count>`

This endpoint returns the recently updated object metadata for the currently authenticated owner and is available to cross-module clients. It reads the `object_refs` metadata projection and does not read the source business body or details.

Request header:

```http
Authorization: Bearer <token>
```

Query parameters:

| Parameter | Type | Default | Rule |
| --- | --- | --- | --- |
| `limit` | integer | `10` | `1..50` |

Success: `HTTP 200`

```json
{
  "objects": [
    {
      "ref_code": "NTE-00000012",
      "module": "notes",
      "object_type": "note",
      "title": "Go Project Layout",
      "tags": ["go", "reference"],
      "status": "draft",
      "created_at": "2026-05-23T01:00:00Z",
      "updated_at": "2026-05-25T07:30:00Z"
    }
  ],
  "limit": 10
}
```

Rules:

```text
Only returns metadata where owner_id = current actor.ID; status does not grant access permissions.
Non-owner superusers cannot read metadata through this entry point either.
Results are fixedly sorted by object_refs.updated_at DESC, ref_code DESC.
Each item uses the same metadata representation as the exact ref_code query, including title, tags, and status; outputs [] when tags are empty.
Returns HTTP 200, "objects": [] when there are no readable objects.
limit only controls the maximum number of records returned; the client cannot override the sort or permission scope.
The response does not return source business content, owner_id, internal object ids, internal object-ref ids, or detail URLs.
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | `limit` is invalid or contains unsupported query parameters |
| `401` | `unauthorized` | Bearer token missing, invalid, expired, or revoked |
| `500` | `recent_objects_unavailable` | metadata list query service unavailable |

---

## 7. Audit Log Queries

### 7.1 `GET /api/platform/audit-logs`

This endpoint is a read-only ops entry point, only allowing `superuser`s to read all audit records. Reading the audit log itself does not generate a `READ` record; the `READ` action is reserved only for LLM-originated business resource reads.

Request header:

```http
Authorization: Bearer <token>
```

Query parameters:

| Parameter | Type | Default | Rule |
| --- | --- | --- | --- |
| `limit` | integer | `50` | `1..100` |
| `offset` | integer | `0` | `>= 0` |
| `target_ref_code` | string | empty | Exact Ref Code |
| `actor_user_id` | integer | empty | `>= 1` |
| `action` | enum | empty | `CREATE/READ/UPDATE/DELETE/EXPORT/LOGIN/LOGOUT` |
| `result` | enum | empty | `SUCCESS/FAILED/DENIED` |

Success: `HTTP 200`

```json
{
  "audit_logs": [
    {
      "id": 1,
      "actor_type": "USER",
      "actor_user_id": 1,
      "action": "LOGIN",
      "target_ref_code": "SYS-00000000",
      "result": "SUCCESS",
      "source_ip": "127.0.0.1",
      "created_at": "2026-05-26T00:00:00Z"
    }
  ],
  "limit": 50,
  "offset": 0
}
```

Rules:

```text
results are ordered by created_at DESC, id DESC
SYS-00000000 is reserved for system-level targets such as LOGIN and LOGOUT
audit_logs supports INSERT and SELECT only; no update/delete endpoint exists and database triggers reject UPDATE, DELETE and TRUNCATE
successful business mutations write SUCCESS in the same PostgreSQL transaction as the mutation
FAILED or DENIED rows are inserted only after the outcome is known, using a new audit-only PostgreSQL transaction when the attempted transaction cannot commit
```

Errors:

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Query parameters invalid |
| `401` | `unauthorized` | token missing, invalid, expired, or revoked |
| `403` | `forbidden` | authenticated actor is not a superuser |
| `500` | `audit_logs_unavailable` | Query service unavailable |

---

## 8. Planned Platform API Constraints

```text
Future full-text Search must apply the actor's authorization scope before returning results.
The Storage API does not expose internal local FS paths or storage implementation details to business clients.
ObjectRef metadata resolution only returns owner-owned metadata; actual object reads cannot bypass the owner module's service permission checks.
```
