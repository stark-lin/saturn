# Object Ref Code

## 1. Positioning

Object Ref Code is a unified, readable reference code for important business objects within Saturn.

It is not a database primary key and does not replace internal relationships between business tables. It is used to provide users, frontends, search, cross-module associations, and LLM tool calls with a short, stable, copyable, and searchable readable reference identifier.

Examples:

```text
USR-00000001
KEY-00000002
NTE-00000003
FIL-00000004
ACC-00000005
CAL-00000006
LLM-00000007
```

Audit logs additionally retain a system-level target code:

```text
SYS-00000000
```

`SYS-00000000` is not registered in `object_refs`; it is only used for system-level operations such as `LOGIN` and `LOGOUT`. Auditing of business objects must use the real or reserved Ref Code of the module they belong to.

Where:

```text
USR stands for a human user identity
KEY stands for an API key identity
NTE stands for the Notes module
FIL stands for the Files module
ACC stands for the Accounting module
CAL stands for the Calendar module
LLM stands for the LLM module
```

The implementation format is fixed to `AAA-XXXXXXXX`: a three-letter uppercase module prefix, plus an eight-character uppercase Hex globally incrementing sequence number. The sequence number is uniformly allocated by the PostgreSQL `object_ref_code_sequence`; transaction rollbacks may leave sequence gaps, but allocated numbers are not reused. A prefix is a module namespace, not an object-type discriminator. When a module contains multiple types, only `object_refs.object_type` identifies the concrete type.

---

## 2. Design Intent

Object Ref Code primarily solves the following problems:

```text
Users can directly reference objects
LLMs can stably identify objects
Cross-module associations have a unified representation
Metadata search results are more readable
Avoid exposing internal database ids
```

For example, a user can say:

```text
View metadata for NTE-00000001
Link FIL-00000002 to ACC-00000003
Query CAL-00000004
```

The system does not need to show users UUIDs, auto-incremented database ids, or complex internal paths.

---

## 3. Basic Approach

Every important object still retains an internal `id`, while readable reference codes are authoritatively maintained by a global registry:

```text
source table id          internal primary key
object_refs.ref_code     external readable reference code
```

Business source tables do not store a duplicate `ref_code`. When a business module query needs to expose the reference code, it joins `object_refs`; search indexes may store a denormalized copy; cross-module generic relations use `object_refs.id`. Auth is an intentional compatibility exception: `users` and `api_keys` retain their claimed `ref_code` because JWTs, sessions, credential lookup, and append-only audit rows use actor RefCodes directly, while `object_refs` remains the global claim registry. `ref_code` is mainly used for display, metadata queries, search, LLM, cross-module referencing, and human communication.

Conceptually, there is a global registry recording the mapping between reference codes and real objects:

```text
object_refs
├── id / owner_id
├── ref_code
├── object_type / object_id
├── title / tags / status
└── created_at / updated_at
```

`owner_id = 0` is the reserved system owner (`SYS`) for `user` and `api_key` identity entries. Positive owner IDs continue to identify the administrator that owns normal business objects. The system owner is a registry sentinel, not a login-capable row in `users`.

This allows the system to resolve through a unified entry point:

```text
NTE-00000001 -> nte-obj
NTE-00000002 -> version-obj
FIL-00000003 -> file_collection
FIL-00000004 -> file
ACC-00000005 -> account
ACC-00000006 -> transaction
CAL-00000007 -> event_aggregate
CAL-00000008 -> event
```

`object_refs` is the authoritative source for `ref_code`, and cross-module display projections of `title`, `tags`, and `status`. The real content, the business source of title/tag, the meaning of status values, and status transition rules are still owned by the source business modules. `tags` are saved as `TEXT[]`; when written, they are trimmed, empty values are discarded, and duplicates are removed keeping the first occurrence, and responses preserve this order.

---

## 4. Applicable Objects

Currently registered object type matrix:

```text
user             USR  system-owned
api_key          KEY  system-owned
nte-obj          NTE
version-obj      NTE
file_collection  FIL
file             FIL
account          ACC
transaction      ACC
event_aggregate  CAL
event            CAL
llm_session      LLM
llm_request      LLM
```

The two auth identity types participate in global code claiming and internal resolution, but are excluded from the shared business ObjectRef metadata endpoints. API key names therefore remain visible only through administrator-only auth APIs.

Objects like RSS Items can extend the type matrix when their real CRUD workflows are implemented. There is no need to assign ref codes to internal relationship tables, configuration items, or index rows, such as:

```text
entity_links
sessions
search_index
```

---

## 5. Project Location

Object Ref Code is better placed in the platform layer rather than a specific business module:

```text
internal/platform/ref
```

The reason is that Accounting, Notes, Files, Calendar, LLM, Search, and frontend queries will all use it.

Conceptual responsibilities:

```text
Generate ref codes
Register object references
Resolve ref codes
Validate ref codes
Provide shared instance metadata queries
Support cross-module associations
Support LLM referencing
```

`internal/platform/ref` should not own any vertical business rules. Business modules are still responsible for their own creation, reading, permissions, auditing, and data relationships.

---

## 6. Runtime Contracts

Business objects are created by clients calling the server create endpoint of the owning module. Clients must not generate, reserve, or submit `ref_code` in the create payload, nor should they directly call a standalone number-generation endpoint.

Upon receiving a create request, the owning service claims the next unified `ref_code` from `platform/ref` and, within the same mutation transaction, creates the source record, registers the reference, and records any required audit event; only upon successful response is the claimed `ref_code` returned to the client. Administrator bootstrap follows the same rule for `USR-*`, and API key creation follows it for `KEY-*`; both registry rows use the `SYS` owner. If the transaction fails, no addressable new resource exists externally; the underlying global sequence allows gaps due to transaction rollbacks, and numbers must not be reused.

When updating an object's title, tags, status, or user-visible content that affects the display of the last updated time, the source business module synchronously updates the `object_refs` display projection in the same business operation. Hard-deleted objects remove their ObjectRefs. Notes deletion removes the `nte-obj` plus every immutable `version-obj` belonging to it in the same transaction.

Notes is the canonical example of a module namespace containing multiple types:

```text
NTE RefCode namespace
├── nte-obj: stable logical identity and mutable current-version pointer
└── version-obj: immutable complete content snapshot, independently resolvable
```

Creating a Note claims one NTE code for each object. Every update claims a new NTE code for a new `version-obj`; the `nte-obj` then advances its current pointer. The API does not restore old content, and hard deletion permanently removes the logical and version ObjectRefs.

Global metadata queries provide instance-wide exact reference code queries, JSON body condition queries, and recent updates lists. Exact queries use the RESTful ObjectRef metadata endpoint:

```http
GET /api/platform/object-refs/NTE-00000001
Authorization: Bearer <token>
```

```json
{
  "ref_code": "NTE-00000001",
  "module": "notes",
  "object_type": "nte-obj",
  "title": "Release notes",
  "tags": ["backend", "release"],
  "status": "draft",
  "created_at": "2026-05-25T00:00:00Z",
  "updated_at": "2026-05-25T00:00:00Z"
}
```

Compatibility with the old endpoint `GET /api/platform/search?ref_code=NTE-00000001` is temporarily retained; new clients should use `/api/platform/object-refs/{ref_code}`.

Clients can query the instance metadata collection using a JSON request body:

```http
POST /api/platform/object-refs/search
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "modules": ["notes"],
  "object_types": ["nte-obj", "version-obj"],
  "statuses": ["draft"],
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

The response is a metadata JSON list; when there are no results, it returns `[]`.

Clients can request the instance's recently updated metadata:

```http
GET /api/platform/recent-objects?limit=10
Authorization: Bearer <token>
```

```json
{
  "objects": [
    {
      "ref_code": "NTE-00000001",
      "module": "notes",
      "object_type": "nte-obj",
      "title": "Release notes",
      "tags": ["backend", "release"],
      "status": "draft",
      "created_at": "2026-05-25T00:00:00Z",
      "updated_at": "2026-05-25T00:00:00Z"
    }
  ],
  "limit": 10
}
```

These endpoints use the same metadata representation: all registered objects return `title` and `tags`; tagless objects return `"tags": []`, and tag names retain the first-occurrence order after server-side normalization. In business object responses, wherever `ref_code` is returned, `tags` must also be returned; `SYS-00000000` is only used for system-level audit targets, is not registered in `object_refs`, and does not require tags. The recently updated list is fixedly sorted by `object_refs.updated_at DESC, ref_code DESC`, with `limit` defaulting to `10` and restricted to `1..50`. JSON body conditional queries support module/object_type/status in, all-tags, created_at/updated_at range, created_at/updated_at/ref_code sort, and `limit`, defaulting to a maximum of `50` returned items with an upper limit of `100`. Responses do not return real business objects, singleton anchor IDs, internal object ids, or business detail URLs. Reads require `data:read`; object status does not grant additional access.

---

## 7. Boundaries

Object Ref Code is kept simple initially:

```text
Does not replace database ids
Does not participate in complex permission judgments
Does not maintain multiple sets of numbers separately per business module
Does not require all tables to have ref codes
Does not treat ref code strings as source business content or status encoding
Does not let platform/ref directly understand business module table structures
```

Metadata queries operate on shared instance data. Parsing a ref code only yields metadata positioning clues; reading the actual object still goes through the corresponding module's service / facade and scope checks.

---

## 8. One-Sentence Summary

Object Ref Code is a unified, readable module-level identity namespace for important Saturn objects; concrete type comes from ObjectRef metadata, while internal relationships still use database ids.
