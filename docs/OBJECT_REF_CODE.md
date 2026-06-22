# Object Ref Code

## 1. Positioning

Object Ref Code is a unified, readable reference code for important business objects within Saturn.

It is not a database primary key and does not replace internal relationships between business tables. It is used to provide users, frontends, search, cross-module associations, and LLM tool calls with a short, stable, copyable, and searchable readable reference identifier.

Examples:

```text
NTE-00000001
FIL-00000002
ACC-00000003
CAL-00000004
LLM-00000005
```

Audit logs additionally retain a system-level target code:

```text
SYS-00000000
```

`SYS-00000000` is not registered in `object_refs`; it is only used for system-level operations such as `LOGIN` and `LOGOUT`. Auditing of business objects must use the real or reserved Ref Code of the module they belong to.

Where:

```text
NTE stands for the Notes module
FIL stands for the Files module
ACC stands for the Accounting module
CAL stands for the Calendar module
LLM stands for the LLM module
```

The implementation format is fixed to `AAA-XXXXXXXX`: a three-letter uppercase module prefix, plus an eight-character uppercase Hex globally incrementing sequence number. The sequence number is uniformly allocated by the PostgreSQL `object_ref_code_sequence`; transaction rollbacks may leave sequence gaps, but allocated numbers are not reused. When the same module contains multiple object types, they are distinguished by `object_type` in the metadata.

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

The source business table does not store a duplicate `ref_code`. When a module query needs to expose the reference code, it joins `object_refs`; search indexes may store a denormalized copy; cross-module generic relations use `object_refs.id`. `ref_code` is mainly used for display, metadata queries, search, LLM, cross-module referencing, and human communication.

Conceptually, there is a global registry recording the mapping between reference codes and real objects:

```text
object_refs
├── id / owner_id
├── ref_code
├── object_type / object_id
├── title / tags / status
└── created_at / updated_at
```

This allows the system to resolve through a unified entry point:

```text
NTE-00000001 -> note
FIL-00000002 -> file_collection
FIL-00000003 -> file
ACC-00000004 -> account
ACC-00000005 -> transaction
CAL-00000006 -> event_aggregate
CAL-00000007 -> event
```

`object_refs` is the authoritative source for `ref_code`, and cross-module display projections of `title`, `tags`, and `status`. The real content, the business source of title/tag, the meaning of status values, and status transition rules are still owned by the source business modules. `tags` are saved as `TEXT[]`; when written, they are trimmed, empty values are discarded, and duplicates are removed keeping the first occurrence, and responses preserve this order.

---

## 4. Applicable Objects

Currently registered object type matrix:

```text
note             NTE
file_collection  FIL
file             FIL
account          ACC
transaction      ACC
event_aggregate  CAL
event            CAL
llm_session      LLM
llm_request      LLM
```

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
Provide owner-only metadata queries
Support cross-module associations
Support LLM referencing
```

`internal/platform/ref` should not own any vertical business rules. Business modules are still responsible for their own creation, reading, permissions, auditing, and data relationships.

---

## 6. Runtime Contracts

Business objects are created by clients calling the server create endpoint of the owning module. Clients must not generate, reserve, or submit `ref_code` in the create payload, nor should they directly call a standalone number-generation endpoint.

Upon receiving a create request, the business module service should, within the same creation operation, create the source record, claim the next unified `ref_code` from `platform/ref`, register the reference, and record the audit event; only upon successful response is the claimed `ref_code` returned to the client. If the transaction fails, no addressable new resource exists externally; the underlying global sequence allows gaps due to transaction rollbacks, and numbers must not be reused.

When updating an object's title, tags, status, or user-visible content that affects the display of the last updated time, the source business module synchronously updates the `object_refs` display projection in the same business operation; when deleting the source object, the reference row is synchronously deleted.

Global metadata queries provide owner-only exact reference code queries, JSON body condition queries, and recent updates lists. Exact queries use the RESTful ObjectRef metadata endpoint:

```http
GET /api/platform/object-refs/NTE-00000001
Authorization: Bearer <token>
```

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

Compatibility with the old endpoint `GET /api/platform/search?ref_code=NTE-00000001` is temporarily retained; new clients should use `/api/platform/object-refs/{ref_code}`.

Clients can query the current owner's metadata collection using a JSON request body:

```http
POST /api/platform/object-refs/search
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "modules": ["notes"],
  "object_types": ["note"],
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

Clients can request the current owner's recently updated metadata:

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
      "object_type": "note",
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

These endpoints use the same metadata representation: all registered objects return `title` and `tags`; tagless objects return `"tags": []`, and tag names retain the first-occurrence order after server-side normalization. In business object responses, wherever `ref_code` is returned, `tags` must also be returned; `SYS-00000000` is only used for system-level audit targets, is not registered in `object_refs`, and does not require tags. The recently updated list is fixedly sorted by `object_refs.updated_at DESC, ref_code DESC`, with `limit` defaulting to `10` and restricted to `1..50`. JSON body conditional queries support module/object_type/status in, all-tags, created_at/updated_at range, created_at/updated_at/ref_code sort, and `limit`, defaulting to a maximum of `50` returned items with an upper limit of `100`. Responses do not return real business objects, owner IDs, internal object ids, or business detail URLs. `status` is not involved in authorization; regardless of status or actor role, only objects whose `owner_id` matches the current actor ID can be read from these endpoints; other exact queries uniformly appear as non-existent, and list queries do not contain unreadable objects.

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

Metadata queries perform owner-only isolation. Parsing a ref code only yields metadata positioning clues; reading the actual object still goes through the corresponding module's service / facade.

---

## 8. One-Sentence Summary

Object Ref Code is a unified, readable reference code for all important internal objects in Saturn, used for user referencing, LLM calls, global metadata queries, and cross-module associations; internal data relationships still use database ids.
