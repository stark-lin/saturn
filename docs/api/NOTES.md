# Notes API

## 1. Ownership and Object Model

Authenticated API prefix: `/api/notes`. Module: `internal/notes`. Common rules: `../API.md`.

Notes owns two first-class Object Reference types in one module-level `NTE-*` namespace:

```text
nte-obj     stable, mutable logical Note identity
version-obj immutable full-content snapshot
```

The prefix identifies only the Notes module. It does not identify the object type. Platform/ObjectRef `object_type` is authoritative:

```text
NTE-00000001 -> nte-obj
NTE-00000002 -> version-obj
NTE-00000003 -> version-obj
```

Both types draw from the same global ObjectRef sequence and `object_refs.ref_code` uniqueness constraint. Internal relationships continue to use database IDs.

## 2. Current Status

`Implemented`. Logical Note CRUD, immutable version history, independent version reads, and transactional hard deletion are registered in `internal/app/routes.go`.

`migrations/000015_notes_version_objects.sql` upgrades existing Notes: each old current Markdown value becomes v1, existing Note ObjectRefs change from `note` to `nte-obj`, a `version-obj` ObjectRef is registered for each v1, and the logical Note points to it.

Collections, NoteLink, NoteTemplate, NoteSource/RSS ingestion, content restoration, and status workflows beyond `draft` remain outside this API. User-to-user sharing is not part of Saturn's single-instance model.

## 3. Endpoint Inventory

| Method | Path | Status | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/notes` | `Implemented` | List logical Notes and current-version summaries |
| `POST` | `/api/notes` | `Implemented` | Create an `nte-obj` and its initial `version-obj` |
| `GET` | `/api/notes/{ref_code}` | `Implemented` | Read an `nte-obj` with current content |
| `PATCH` | `/api/notes/{ref_code}` | `Implemented` | Create a new version and advance the current pointer |
| `DELETE` | `/api/notes/{ref_code}` | `Implemented` | Permanently delete the logical Note, all versions, and all ObjectRefs |
| `GET` | `/api/notes/{ref_code}/versions` | `Implemented` | List all versions of a logical Note, newest first |
| `GET` | `/api/notes/versions/by-ref/{ref_code}` | `Implemented` | Independently read one `version-obj` by RefCode |

All endpoints require an administrator JWT or API key. Reads require `data:read`; mutations require `data:write`. Path values must match `^NTE-[0-9A-F]{8}$`; because the prefix is a module namespace, the service resolves `object_type` and returns `404` when a valid NTE code names the wrong type for that endpoint.

## 4. Representations

### 4.1 Logical Note with Current Version

```json
{
  "note": {
    "ref_code": "NTE-00000001",
    "current_version_ref": "NTE-00000003",
    "version_number": 2,
    "operation": "update",
    "title": "PostgreSQL maintenance checklist",
    "markdown": "PostgreSQL maintenance checklist\npostgres, maintenance\n\nUpdated steps.",
    "content_type": "text/markdown",
    "tags": ["postgres", "maintenance"],
    "status": "draft",
    "created_at": "2026-05-25T00:00:00Z",
    "updated_at": "2026-05-26T00:00:00Z"
  }
}
```

`ref_code` is the stable `nte-obj` identity until the Note is deleted. `current_version_ref` is independently resolvable and changes whenever content is updated. `version_number` is a Note-local display/order value and never replaces a RefCode. The logical Note stores no content directly.

List items omit Markdown but include `ref_code`, `current_version_ref`, `version_number`, title, tags, status, and `updated_at`.

### 4.2 Immutable Version

```json
{
  "version": {
    "ref_code": "NTE-00000003",
    "nte_ref": "NTE-00000001",
    "parent_version_ref": "NTE-00000002",
    "version_number": 2,
    "title": "PostgreSQL maintenance checklist",
    "content": "PostgreSQL maintenance checklist\npostgres, maintenance\n\nUpdated steps.",
    "content_type": "text/markdown",
    "operation": "update",
    "tags": ["postgres", "maintenance"],
    "created_at": "2026-05-26T00:00:00Z"
  }
}
```

Version operations created by the current API are `create` or `update`. Version rows and their ObjectRef projections are immutable. An update points `parent_version_ref` at the former current version, while `version_number` preserves operation order.

The version list returns the same fields except `content`. Direct version reads are available only while the owning logical Note exists. Hard deletion removes every version, so deleted content cannot be read or restored.

## 5. Markdown Contract

Create/update content remains Markdown with this physical structure:

```text
Line 1: non-empty title
Line 2: comma-separated tags; may be empty
Line 3 onward: body
```

The server trims the title, normalizes tags by trimming/discarding empties/deduplicating in first-occurrence order, and saves the submitted source unchanged as `version-obj.content`. The only supported current content type is `text/markdown`.

## 6. Write Semantics

### Create

Request:

```json
{"markdown":"Title\ntag\n\nBody"}
```

The service claims two distinct `NTE-*` codes, creates the logical Note, registers `nte-obj`, creates v1 with operation `create`, registers `version-obj`, sets the current pointer, and writes CREATE audits in one business transaction. Success is `201` with `Location: /api/notes/{nte_ref}`.

### Update

`PATCH /api/notes/{nte_ref}` accepts only `markdown`. It locks the logical Note, creates a new immutable version with `version_number = current + 1`, advances the pointer, and refreshes only the logical Note's ObjectRef title/tags projection. Existing versions are never updated.

### Hard delete

`DELETE /api/notes/{nte_ref}` is permanent and cannot be undone. In one transaction, the service locks the logical Note, loads every version, records a DELETE audit for each `version-obj` and the `nte-obj`, removes every version ObjectRef and the logical ObjectRef, then physically deletes the Note; the database relationship cascades removal of all `note_versions` rows. No deleted-Note restore or old-content restore endpoint exists.

## 7. Query and Authorization Rules

`GET /api/notes` supports `text`, `tag`, `limit` (`1..100`, default `20`), and `offset` (`>=0`). Text search uses only the current version. Results sort by logical Note `updated_at` descending, then `nte-obj.ref_code` descending.

All Note and version operations use shared instance scope. Missing, deleted, and wrong-object-type resources return `404/not_found`; API keys without the required route scope receive `403/insufficient_scope` before the service call.

## 8. Audit and Errors

Creating or updating content records a CREATE audit for the new `version-obj` and a CREATE or UPDATE audit for the `nte-obj` in the same transaction. Hard deletion records DELETE for every version with reason `cascade_note_delete`, then DELETE for the logical Note. Audit rows store the administrator or API-key RefCode and never contain content, credentials, or Authorization headers.

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Malformed/unsupported JSON, query, or RefCode format |
| `400` | `invalid_markdown` | Markdown violates the fixed format |
| `401` | `unauthorized` | Authentication missing or invalid |
| `404` | `not_found` | Object missing, deleted, not owned, or wrong type |
| `500` | `notes_unavailable` | Repository or required lower-level dependency unavailable |

## 9. Implementation Constraints

```text
nte-obj and version-obj use the same NTE RefCode namespace.
Object type comes from object_refs.object_type, never from the prefix.
notes runtime behavior uses stable identity, current_version_id, and timestamps; the legacy deleted_at schema column is unused, and runtime deletion is physical.
note_versions stores immutable complete snapshots.
Every content update inserts a new note_versions row and ObjectRef.
version_number is display/order metadata only.
Deleting a Note atomically removes its source row, all version rows, and every associated ObjectRef; no content or Note restore endpoint exists.
Handlers bind transport; services enforce scope-sensitive business rules, version rules, projection updates, hard deletion, and audit.
```
