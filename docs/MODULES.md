# MODULES.md

## 1. Goal

This document records Saturn's business module splits, module boundaries, and dependency directions. For detailed directory conventions, please refer to `docs/FILES.md`.

For module relationship diagrams, see [MODULE_ER.md](MODULE_ER.md).

---

## 2. Top-level Business Modules

Business capabilities are organized into the implemented top-level domains below.

```text
Accounting
├── Account
└── Transaction
    └── Tags projected to ObjectRef

Notes
├── Note
├── Collection
├── NoteLink
├── NoteTemplate
└── NoteSource
    └── RSS

Files
├── FileCollection
├── File
├── Upload
└── Download

Calendar
├── EventAggregate
└── Event
    └── Tags projected to ObjectRef

LLM
├── Session
├── Request
├── Request Result
├── Reference Context
└── Provider Audit

Platform
├── Auth
├── Audit
├── Config
├── ObjectRef
├── Reference Metadata Search
├── Storage
└── SSE
```

Go directories use lowercase package names:

```text
internal/accounting
internal/notes
internal/files
internal/calendar
internal/llm
internal/platform
```

`Platform` is the product platform capability domain; `internal/platform` carries both underlying support packages and platform capability sub-packages. Platform capabilities must not own vertical business rules of Accounting, Notes, Files, Calendar, or LLM.

---

## 3. First Batch Modules

The first batch implements the main closed loop:

```text
Platform / Auth / Audit / Config / Storage / ObjectRef metadata / SSE
Files
Notes
Accounting
Calendar
LLM read-only queued request/result
Background jobs
```

The first batch does not implement:

```text
full LLM write-agent execution
plugin system
external bank sync
embedding / vector search
real-time collaboration
global search indexing / reindex orchestration
scheduler package
```

---

## 4. Core Dependency Directions

```text
cmd/server
  -> internal/app
  -> business modules
  -> internal/platform
  -> external infrastructure
```

`internal/app` is responsible for wiring all modules and routes.

`internal/calendar` uses `EventAggregate` as the aggregate root, and `Event` is a specific schedule instance that must belong to an aggregate. Aggregates can be created empty like Accounting Accounts; Events must be created through the parent aggregate scope like Transactions. Both aggregates and specific events are registered as `CAL-*` ObjectRefs and can have tags attached; metadata is immutable after creation. Specific events cannot be deleted, only transitioned via `scheduled -> finished`, `scheduled -> voided`, or `finished -> voided`; after becoming finished/voided, they do not enter the main calendar view but remain in the aggregate's sub-event list. Deletion only happens at the EventAggregate level, and the Calendar service cascades the cleanup of child events' references and tags, while writing a DELETE audit for each cascaded deleted event.

The core dependency graph must remain a directed acyclic graph (DAG). Dependencies can only point from upper layers to lower layers, and lower modules cannot directly or indirectly depend on upper modules; cross-business module whitelists also cannot introduce circular dependencies.

Business modules do not directly depend on the Redis client, local FS storage implementations, LLM SDKs, or DB drivers.

---

## 5. Platform

`Platform` is a horizontal product capability domain, not a privileged entry point for any specific vertical business module.

Rules:

```text
Auth owns identity, session, password, and authorization entry points
Audit owns append-only audit record insertion and superuser-only audit queries
Config owns runtime configuration loading and validation
Search currently owns ObjectRef metadata HTTP handlers and compatibility metadata lookup
ObjectRef owns readable reference code generation and shared title/status metadata projections
ObjectRef owns unified ref_code, title, tags and status metadata projections
Storage owns local filesystem storage abstraction, usage and diagnostics
```

Global search indexing and scheduling are not retained as code in the current baseline. Add their packages only when their runtime behavior is implemented.

`Platform/Search` is limited to ObjectRef metadata endpoints:

```text
platform search does not own source business records
platform search does not mutate source records
platform search delegates metadata reads to platform/ref
```

`Platform/ObjectRef` is a unified object reference code capability at the conceptual layer. It provides short, stable, readable `ref_code` for first-class business objects per business module, and maintains `title` and `status` projections used for owner-only metadata queries. These are used for user referencing, LLM calls, search results, and cross-module associations. It does not replace internal database `id`s, does not directly own business rules, and does not bypass business module services / facades. See [OBJECT_REF_CODE.md](OBJECT_REF_CODE.md) for a detailed summary.

`ObjectRef` maintains unified `ref_code`, `title`, `tags`, and `status` metadata projections. Platform metadata returns `object_refs.tags` as an array of `tags` together with the object's `ref_code` and `title`; tagless objects return an empty array. Notes, Files, Accounting, Calendar, and LLM can use tags, but do not each own independent tag relationship tables.

---

## 6. LLM

`LLM` is a controlled auxiliary module and does not own the business rules of other modules.

Rules:

```text
LLM can orchestrate business module facades
LLM cannot access business repos directly
LLM cannot bypass auth or audit
LLM cannot bypass preview / safety checks
current baseline LLM is read-only queued request/result
```

LLM tool calls must go through exported service / facade paths and record LLM audit events.

Current baseline LLM persists `llm_sessions`, and merges request/response into `llm_requests` and `llm_request_references`. Session and Request are registered as `LLM-*` ObjectRefs; the HTTP session detail displays request inputs and response results in a request view. Requests are immutable once submitted, and are claimed and asynchronously advanced (`queued -> running -> success/error`) by a fixed number of workers using PostgreSQL `FOR UPDATE SKIP LOCKED`; no retry / dead-letter is implemented; provider timeouts are written to `llm_request_timeout`; you can only delete an entire session which recursively deletes its requests.
