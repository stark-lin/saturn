# API.md

## 1. Goal and Boundaries

This document is the general specification and module document index for the Saturn HTTP API.

```text
docs/API.md              Cross-module general contracts, status rules, and document index
docs/api/<MODULE>.md     Endpoints, requests, responses, and permission contracts owned by the module
internal/app/routes.go   The single source of truth for currently registered HTTP routes
```

Specific endpoints are not repeatedly defined in this document. When adding or modifying endpoints, you must update the documentation of the module that owns the behavior; only modify this document when changing general responses, authentication, errors, or SSE transmission conventions.

---

## 2. Document Ownership Rules

Each API route has only one owner module. The owner is determined by the service that executes the business rules or orchestrates the behavior, not by the calling page or data source.

```text
Accounting behaviors         -> api/ACCOUNTING.md
Notes behaviors            -> api/NOTES.md
Files behaviors              -> api/FILES.md
Calendar behaviors           -> api/CALENDAR.md
LLM behaviors                -> api/LLM.md
Auth / ObjectRef metadata /
Storage /
Shared event streams and other platform behaviors -> api/PLATFORM.md
```

Cross-module orchestrations are still recorded in the orchestrating owner's API document and link to relevant module rules, without duplicating the same endpoint contract.

### 2.1 Module Index

| Module | Document | Reserved Path Prefix | Current Public Route Status |
| --- | --- | --- | --- |
| Accounting | [api/ACCOUNTING.md](api/ACCOUNTING.md) | `/api/accounting` | Ledger and immutable transaction APIs registered |
| Notes | [api/NOTES.md](api/NOTES.md) | `/api/notes` | Owner-only CRUD registered |
| Files | [api/FILES.md](api/FILES.md) | `/api/files` | Collection / File APIs registered |
| Calendar | [api/CALENDAR.md](api/CALENDAR.md) | `/api/calendar` | EventAggregate / Event APIs registered |
| LLM | [api/LLM.md](api/LLM.md) | `/api/llm` | Session / Request / Response APIs registered |
| Platform | [api/PLATFORM.md](api/PLATFORM.md) | `/api/auth`, `/api/events`, `/api/platform/*` | Auth, events, metadata, and superuser audit queries registered |

### 2.2 Non-module APIs

| Method | Path | Authentication | Status | Purpose |
| --- | --- | --- | --- | --- |
| `GET` | `/healthz` | Public | `Implemented` | Process health check |

`/healthz` is an application runtime status entry point, does not carry business module behavior, and is therefore kept in the general document.

---

## 3. Endpoint Status Specifications

Endpoint tables in module documents must be marked with one of the following statuses:

| Status | Meaning |
| --- | --- |
| `Implemented` | Registered in `internal/app/routes.go` and has an executable handler contract |
| `Planned` | Belongs to the confirmed scope but is not registered as a callable route |
| `Deprecated` | May still be callable but scheduled for removal; alternative entry points and removal conditions must be documented |

Rules:

```text
The presence of a package, handler type, migration, or frontend placeholder page does not mean the endpoint is Implemented.
Unregistered routes must not be described to clients as available interfaces.
New endpoints should first have their contracts determined in the corresponding module document, then have their status changed to Implemented upon implementation.
When removing or making incompatible changes to an endpoint, the documentation must be updated in the same change.
```

### 3.1 Minimum Content for Module Documents

Each `docs/api/<MODULE>.md` must contain at least:

```text
Module responsibilities and path ownership
Current implementation status
Endpoint inventory (method, path, auth, status, purpose)
Request / response / error contracts for implemented endpoints
Module-specific rules for permissions, audits, asynchronous events, or data exposure
```

Only modules in `Planned` status can document capability scopes and constraints first without making up unconfirmed request or response fields.

---

## 4. General HTTP Conventions

```text
REST API first
SSE transport is available at /api/events
No WebSocket
JSON for REST request / response bodies
Web UI consumes the same REST API and SSE endpoints
API business route prefix is /api
```

REST JSON responses use `Content-Type: application/json`. Routes requiring authentication use:

```http
Authorization: Bearer <token>
```

For the specific contract of authentication endpoints, see [api/PLATFORM.md](api/PLATFORM.md).

Successful responses should use explicit resource or result structures, rather than returning bare strings as the main response body.

---

## 5. Error and Permission Conventions

Error responses use a unified structure:

```json
{
  "error": {
    "code": "string",
    "message": "string"
  }
}
```

The handler is only responsible for the authentication entry point, parameter binding, and response writing; resource-level authorization is executed by the service. Ops APIs still call normal module services / facades and pass the corresponding `Principal`.

Error codes exposed externally must avoid resource enumeration. For interfaces that read or modify a single resource by resource ID, `ref_code`, or path:

```text
Resource does not exist: HTTP 404, code = "not_found"
Resource exists but the current actor has no access: HTTP 404, code = "not_found"
```

Do not return `access_denied`, `forbidden`, or HTTP 403 for such resource-level authorization failures. Authentication failures can still return HTTP 401, e.g., not logged in, expired session, or invalid credentials. Non-resource-level global entry restrictions can be defined by the module contract, but must not leak the existence of specific resources.

Plain text passwords and JWT / `Authorization` headers are sensitive information and must not be written to logs, audit metadata, or normal API responses.

---

## 6. SSE Conventions

`GET /api/events` is the unified authenticated SSE transmission entry point, its transmission contract is found in [api/PLATFORM.md](api/PLATFORM.md).

When an event is generated by the business behavior owner, its event name and payload should be defined in the corresponding module document, and linked from the Platform document. Because the native `EventSource` cannot send a Bearer header, browser clients after login use a streaming `fetch` with the `Authorization` header to connect to SSE.

---

## 7. Object Ref Code

Important resource responses may expose a `ref_code` as a readable reference identifier for users, frontends, search results, cross-module associations, and LLM tool calls.

Examples:

```text
NTE-00000001
FIL-00000002
ACC-00000003
CAL-00000004
LLM-00000005
```

API Conventions:

```text
ref_code does not replace internal ids
The authoritative source for ref_code is object_refs; the source business table does not store this code redundantly
Cross-module metadata projections of title, tags, and status are stored in object_refs; tags use object_refs.tags TEXT[]
Clients must not generate, reserve, or submit ref_code when creating business objects
The service of the object's owning module claims and registers the ref_code from Platform/ObjectRef during the server-side creation operation, and returns this code to the client only upon a successful response
No standalone ref_code claim endpoint separated from source resource creation is provided
Formal resource relationships still use internal ids
Reading the actual object via ref_code still goes to the corresponding module's service / facade
Permission checks are still executed at the service layer
Exact code metadata queries are returned as JSON by GET /api/platform/object-refs/{ref_code}; the old GET /api/platform/search?ref_code=<code> is retained only as a compatibility entry point
Metadata collection queries receive a JSON body via POST /api/platform/object-refs/search, and return a JSON list
Recently updated metadata queries are returned as a JSON list by GET /api/platform/recent-objects?limit=<count>; limit defaults to 10, with a range of 1..50
Metadata responses uniformly include title, tags, and status; tagless objects return an empty array, and tags retain their first-occurrence order after server-side normalization
Whenever ref_code is returned in a business object response, tags must also be returned simultaneously; SYS-00000000 is used only for system-level audit targets, is not registered as a business object, and does not require tags
Source business modules synchronously maintain the corresponding metadata projection when creating, updating, or deleting objects that affect title/tags/status
These metadata queries only allow access by the resource owner; status does not grant read permissions, nor does the superuser role relax this entry point
Metadata interfaces do not return business object contents, owner_ids, internal object ids, or business detail URLs; the actual reading endpoint is still defined by the module that owns the object
```

---

## 8. Maintenance Checklist

```text
When adding or modifying routes: Update the owning docs/api/<MODULE>.md and add handler contract tests
When modifying shared errors, authentication, SSE, or ref_code rules: Also update docs/API.md
When adjusting module owners or path prefixes: Update this index, docs/MODULES.md, and structure documents
Only routes registered in internal/app/routes.go can be marked as Implemented
```
