# Notes API

## 1. Ownership

The Notes HTTP contract currently owns the single current Markdown source content of a Note, and the title and tag views derived from that content.

```text
Authenticated API prefix: /api/notes
Module: internal/notes
Common rules: ../API.md
```

In this document, `Note` refers to the first-class source resource of the Notes module; external references use `ref_code`, for example `NTE-00000001`, and do not expose the database `id`.

---

## 2. Current Status and Fixed Scope

`Implemented`. The owner-only `/api/notes` CRUD routes listed in this section have been registered in `internal/app/routes.go`.

`migrations/000004_notes.sql` has provided the persistent fields `markdown`, `created_at`, and `updated_at` currently required by the API.

The scope of the first version Note API fixed this time:

```text
Server-side Note create with server-claimed ref_code
Owner-only list / read / update / delete
Exactly one current Markdown source per Note
Title and tags derived from the current Markdown source
Read-only title/status metadata projected through Platform/ObjectRef
Read-only tag metadata projected to object_refs.tags for ObjectRef queries
```

Not in scope for this implementation:

```text
Revision history, version recovery, or any other version capability
Status changes
Collection
NoteLink
NoteTemplate
NoteSource / RSS ingestion
Asynchronous Notes events
```

When new endpoints are added for these capabilities, the request, response, permission, and status contracts must be supplemented before the routes are implemented.

---

## 3. Endpoint Inventory

| Method | Path | Authentication | Status | Purpose |
| --- | --- | --- | --- | --- |
| `GET` | `/api/notes` | Bearer JWT | `Implemented` | List Note summaries for the current owner |
| `POST` | `/api/notes` | Bearer JWT | `Implemented` | Create a Note on the server and claim its `ref_code` |
| `GET` | `/api/notes/{ref_code}` | Bearer JWT | `Implemented` | Read the current owner's Note source content |
| `PATCH` | `/api/notes/{ref_code}` | Bearer JWT | `Implemented` | Replace the current owner's Note source content |
| `DELETE` | `/api/notes/{ref_code}` | Bearer JWT | `Implemented` | Delete the current owner's Note |

Path parameter rule:

```text
ref_code: canonical NTE reference code, must match ^NTE-[0-9A-F]{8}$.
```

---

## 4. Resource Representations

### 4.1 Note Detail

`GET`, `POST`, and `PATCH` return the complete current Note. Only one current `markdown` exists per Note; updates replace this value and do not generate historical versions.

```json
{
  "note": {
    "ref_code": "NTE-00000001",
    "title": "PostgreSQL maintenance checklist",
    "markdown": "PostgreSQL maintenance checklist\npostgres, maintenance, reference\n\nCheck database health before the upgrade.",
    "tags": ["postgres", "maintenance", "reference"],
    "status": "draft",
    "created_at": "2026-05-25T00:00:00Z",
    "updated_at": "2026-05-25T00:00:00Z"
  }
}
```

Field rules:

| Field | Rule |
| --- | --- |
| `ref_code` | Claimed and returned by the server via Platform/ObjectRef during the creation operation; the client cannot generate, reserve, specify, or modify it |
| `title` | Derived by the server from the first line of the current `markdown` and synchronized to the `object_refs.title` metadata projection; the client cannot edit it separately |
| `markdown` | The Note's sole current authoritative source content; see Section 5 for the format |
| `tags` | Derived by the server from the second line of the current `markdown`, and projected to the same `object_refs.tags`; the client cannot edit them separately |
| `status` | Read-only projection of `object_refs.status`; Notes created by the current Notes API are fixed as `draft`, the client cannot specify or modify it |
| timestamps | RFC3339 UTC time strings |

`owner_id`, internal Note IDs, and internal object-ref IDs must not appear in Note API responses.

### 4.2 Note Summary List

The list does not return `markdown` to avoid loading large amounts of source body text.

```json
{
  "notes": [
    {
      "ref_code": "NTE-00000001",
      "title": "PostgreSQL maintenance checklist",
      "tags": ["postgres", "maintenance", "reference"],
      "status": "draft",
      "updated_at": "2026-05-25T00:00:00Z"
    }
  ],
  "pagination": {
    "limit": 20,
    "offset": 0,
    "has_more": false
  }
}
```

---

## 5. Markdown Source Format

Note bodies uniformly save and transmit Markdown source content; the API does not accept or persist browser-generated HTML as the body source.

`markdown` uses the following fixed physical line structure:

```text
Line 1: title, plain text; must be non-empty after trimming leading and trailing whitespace, do not write Markdown heading markers
Line 2: tags, separated by ASCII commas ","; allowed to be empty
Line 3 onwards: body, saved as-is as the Markdown body
```

A valid request must at least contain the first and second lines. A Note without tags should still pass an empty second line; the body text starts from the third line.

Example:

```md
PostgreSQL maintenance checklist
postgres, maintenance, reference

Check database health before the upgrade and verify the upgrade plan.
```

Derived field rules:

```text
title = trim(first line)
tags = split(second line, ","), trim each item, discard empty items, preserve first occurrence when identical tags repeat
body = all content beginning on physical line 3
```

The frontend can use fixed versions of `marked` and `DOMPurify` to provide a preview of the current content, but the preview result is not the authoritative source content stored by the server.

---

## 6. Authenticated Note Endpoints

All endpoints in this section require:

```http
Authorization: Bearer <token>
```

Requests with a JSON body additionally require `Content-Type: application/json`.

### 6.1 `GET /api/notes`

Query parameters:

| Parameter | Type | Default | Rule |
| --- | --- | --- | --- |
| `text` | string | omitted | Search within the current title and Markdown source content |
| `tag` | string | omitted | Only return Notes whose currently derived tags include this tag |
| `limit` | integer | `20` | `1..100` |
| `offset` | integer | `0` | `>= 0` |

Results only contain Notes where `owner_id = actor.ID`, and are fixed to sort by `updated_at` descending, then `ref_code` descending.

Success: `HTTP 200`, response uses the Note Summary List representation.

### 6.2 `POST /api/notes`

This endpoint is the sole Note creation entry point in the first version. The client must initiate a creation request to the server, and the server obtains the unified `ref_code`.

Request:

```json
{
  "markdown": "PostgreSQL maintenance checklist\npostgres, maintenance, reference\n\nCheck database health before the upgrade."
}
```

Rules:

```text
markdown is required and is validated under section 5.
Only markdown is accepted in the request body.
title, tags, status, ref_code and any version fields must not be supplied.
The created Note owner is always the authenticated actor.
The client must not generate, reserve or send ref_code in a create request.
```

Success: `HTTP 201`

```http
Location: /api/notes/NTE-00000001
```

The response uses the Note Detail representation. Creation is server-owned and executes as one business operation:

```text
1. Client sends POST /api/notes with current Markdown only.
2. Notes service validates Markdown and binds ownership to the authenticated actor.
3. Notes creates the source record and asks Platform/ObjectRef to claim the next unified ref_code and register the derived title plus status `draft` projection.
4. Notes writes the derived tags into the same object ref projection and records the create audit action.
5. Only a successful response returns the server-claimed ref_code and canonical Location.
```

No client-facing ID-reservation or `/claim` endpoint is defined.

### 6.3 `GET /api/notes/{ref_code}`

Success: `HTTP 200`, response uses the Note Detail representation.

Only the Note owner may read its Markdown source through this endpoint.

### 6.4 `PATCH /api/notes/{ref_code}`

Request:

```json
{
  "markdown": "PostgreSQL maintenance checklist\npostgres, maintenance\n\nUpdated steps."
}
```

Rules:

```text
markdown is required and is the only supported mutable field.
The request replaces the Note current Markdown source after validation.
The update refreshes the derived `object_refs.title` and `object_refs.tags` projection for the same object ref.
The current read-only status projection is preserved.
The API does not retain or expose previous Markdown values.
```

Success: `HTTP 200`, response uses the Note Detail representation.

### 6.5 `DELETE /api/notes/{ref_code}`

Deletes the current Note source, tag associations and object reference projection. Audit history remains according to the platform audit retention rule.

Success: `HTTP 204`, empty body.

---

## 7. Authorization And Audit

Notes permissions in this first contract are owner-only:

```text
An authenticated actor may create Notes only for itself.
An authenticated actor may list, read, modify and delete only Notes it owns.
The Notes API does not grant superuser access to another owner's Note.
The Notes API does not provide shared reading behavior.
The returned status is metadata and does not grant reading behavior.
Resource-not-found and resource-not-owned are folded to HTTP 404 / code = "not_found".
```

Audit requirements:

```text
Create, update and delete are service-level audited actions.
Create, update and delete SUCCESS rows contain the Note target_ref_code and commit in the same PostgreSQL transaction as the Note mutation.
Create reserves an NTE Ref Code before its auditable write transaction; if that transaction fails, the same code may remain only in its FAILED audit row.
After a write transaction has failed or a target is denied, FAILED or DENIED is inserted in a new audit-only PostgreSQL transaction.
Ordinary Note list and detail reads do not generate READ audit rows; READ is reserved for LLM-originated resource reads.
Audit rows must not contain Markdown source content, JWTs or Authorization headers.
```

---

## 8. Errors

Authenticated JSON endpoints use the common error envelope in `../API.md`:

```json
{
  "error": {
    "code": "invalid_markdown",
    "message": "markdown title line is required"
  }
}
```

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Malformed JSON, unsupported field, invalid query value or invalid pagination |
| `400` | `invalid_markdown` | Markdown does not satisfy the fixed source format |
| `401` | `unauthorized` | Bearer token is missing, invalid, expired or revoked |
| `404` | `not_found` | Note does not exist or is not owned by the actor |
| `500` | `notes_unavailable` | Notes service, repository or required lower-level dependency is unavailable |

---

## 9. Deferred Capabilities

The current API does not define or register endpoints for:

```text
Note revision history or version recovery
Status transitions or sharing management
Collection, NoteLink, NoteTemplate or NoteSource
```

If one of these capabilities is introduced later, this document must first define its representations, endpoint inventory, ownership rules, persistence requirements and tests.

---

## 10. Implementation Requirements

When routes implementing this contract are introduced:

```text
internal/app/routes.go must register each implemented path before its status changes to Implemented.
The API implementation maintains one current Markdown source per Note and exposes no version behavior.
The Notes schema stores the current markdown and create/update timestamps required by this contract.
Note creation and projection updates must maintain `object_refs.title/tags/status` in the same business operation.
ObjectRef projections registered by this owner-only API have status `draft`; status does not change owner-only access rules.
Note create handlers must reject client-supplied ref_code and return only the ref_code claimed by Platform/ObjectRef on the server.
Tags must be projected to ObjectRef rather than a Notes-private tag relationship, so Platform metadata can return tags beside the Note ref_code and title.
The service layer must enforce owner-only access and audit.
Handlers must only bind/validate transport input, obtain Principal, call services and write responses.
Contract tests must cover Markdown derivation, owner-only authorization-as-404 and current Markdown replacement without version endpoints.
```
