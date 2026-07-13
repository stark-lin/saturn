# PRD Features Modules

> Split from `docs/PRD.md`: Product feature module scope.

## 4. Accounting

### 4.1 Positioning

Accounting is a minimalist manual bookkeeping module. An Account is a ledger, and Transactions are its immutable flow.

Principles:

```text
Only record, do not advise.
Only summarize, do not manage wealth.
Only manual, do not connect to banks.
```

### 4.2 Feature Scope

```text
Accounting
├── Account
│   ├── Opening Balance Cents
│   ├── Cached Balance Cents
│   ├── Currency
│   └── Tags (via ObjectRef)
└── Transaction
    ├── Income / Expense / Adjustment
    ├── Occurred Date
    ├── BIGINT Amount Cents
    ├── Title / Note
    ├── Tags (via ObjectRef)
    └── Posted / Voided Status
```

Constraints:

```text
Account and Transaction both use ACC-* reference codes provided by Platform/ObjectRef.
Transactions cannot be modified after creation; they can only transition from posted to voided.
A single Transaction does not provide a delete behavior; deleting an Account will cascade physically delete its Transactions.
Categories, merchants, and scenarios are uniformly expressed via tags; FIL-* / NTE-* weak references are written in the note.
Account balance only aggregates posted transactions and is recalculated within the same transaction of creation or voiding.
Balance-related writes and ledger deletions must lock the Account row first.
The first version does not provide Category, Budget, Subscription, Report, or object attachment relations.
```

---

## 5. Notes

### 5.1 Positioning

Notes is the main content area of the system, carrying personal notes, technical articles, changelogs, snippets, debug logs, reference notes, and RSS sources.

### 5.2 Feature Scope

```text
Notes
├── nte-obj (stable logical Note)
│   ├── Private Note
│   ├── Markdown Note
│   ├── Technical Note
│   ├── Changelog Note
│   ├── Snippet
│   ├── Debug Log
│   ├── Reference Note
│   └── Soft delete / restore
├── version-obj
│   ├── Immutable full snapshot
│   ├── Independent NTE RefCode
│   └── Version history / restore by copy
│
├── Tag Projection (via ObjectRef)
├── Collection
├── NoteLink
├── NoteTemplate
└── NoteSource
    └── RSS
```

## 6. Files

### 6.1 Positioning

Files is the file material foundation of the system, used to uniformly manage regular files, media files, documents, compressed packages, receipt files, manuals, downloaded packages, and other materials.

### 6.2 Feature Scope

```text
Files
├── Collection
├── File
│   ├── Upload
│   ├── Metadata
│   ├── SHA-256 Hash
│   ├── BLAKE3 Hash
│   ├── Direct Delete
│   └── Download
│
└── Collection Delete Cascade
    └── Expiring Link
```

### 6.3 Typical Uses

```text
Note attachments
Transaction receipts
Manuals
General material archiving
release packages
```

---

## 7. Calendar

### 7.1 Positioning

The first version of Calendar is narrowed down to event aggregates and specific events.

Design mindset:

```text
EventAggregate is the aggregate root.
EventAggregate can be created empty.
Event is a specific schedule instance and must be created under an EventAggregate.
Event only saves the start and end timestamps.
Recurrence rules are only expanded into specific Events upon Event creation.
finished / voided Events do not enter the main Calendar view but are retained in the aggregate details.
```

### 7.2 Feature Scope

```text
Calendar
├── EventAggregate
│   ├── immutable metadata
│   ├── tags
│   └── aggregate delete
│
├── Event
│   ├── starts_at
│   ├── ends_at
│   ├── immutable metadata
│   ├── tags
│   └── scheduled -> finished -> voided
│
└── CalendarView
    └── scheduled events only
```

---

## 8. LLM

### 8.1 Positioning

LLM is a system enhancement module. It adopts a backend-hosted model: the frontend does not call the model directly, and the LLM does not access the database, Redis, or local FS storage directly.

Interaction flow:

```text
Frontend submits natural language request
→ Backend creates LLM session
→ Backend performs permission check
→ Backend constructs context
→ Backend calls LLM provider
→ LLM requests tool call
→ Backend executes controlled tool
→ Result returned to frontend via HTTP polling
```

### 8.2 Feature Scope

```text
LLM
├── Tool
├── Context
├── Prompt
├── Summary
└── Audit
```

First version capabilities:

```text
Ask My Data
Search files / notes / accounting records / planned items
Summarize files
Summarize Notes
Generate Note drafts
Suggest tags
Suggest relations
Record LLM reference read audit
```

The first version LLM only does read-only / draft-only and does not execute formal write operations.

---

## 9. Platform

### 9.1 Positioning

Platform is a horizontal product capability domain responsible for authentication, configuration, search, object reference codes, storage, audit, and operations views. Platform does not own the vertical business rules of Accounting, Notes, Files, Calendar, or LLM.

### 9.2 Feature Scope

```text
Platform
├── Auth
│   ├── Login
│   ├── Session
│   ├── Password
│   ├── Identity
│   └── Access Entry Points
│
├── Config
│   ├── Runtime Config
│   ├── Config Validation
│   └── Config Diagnostics
│
├── Search
│   ├── Exact ObjectRef Metadata Lookup
│   ├── Filtered ObjectRef Metadata Search
│   ├── Recent Object Metadata
│   └── Owner Scope Filtering
│
├── ObjectRef
│   ├── Readable Ref Code
│   ├── Object Registration
│   ├── Ref Resolution
│   ├── Owner-only Metadata Lookup
│   ├── Cross-module Reference
│   └── LLM Reference
│
├── Tag
│   ├── Tag Metadata
│   └── ObjectRef Tag Projection
│
├── Storage
│   ├── Local File Storage
│   ├── Object References
│   ├── Storage Usage
│   └── Storage Diagnostics
│
└── Validate
    ├── Input Validation Primitives
    ├── Config Validation Helpers
    └── Error Normalization
```

### 9.3 Contributor Mechanism

Platform/Search integrates business modules via contributors when source modules expose searchable metadata:

```text
Accounting contributor
Notes contributor
Files contributor
Calendar contributor
LLM contributor when needed
```

Rules:

```text
Platform/Search does not directly own source business records.
Platform/Search does not directly modify source business modules.
Business modules decide which fields are indexable.
internal/app is responsible for registering each module's contributor to the Platform.
Contributor adapters can wrap business services via internal/app to avoid source modules reverse-depending on Platform orchestration packages.
```

### 9.4 Object Ref Code

Object Ref Code is a unified, readable reference code for important business objects. Its goal is not to replace database primary keys, but to serve as a readable reference identifier used by users, frontends, search, cross-module associations, and LLM calls.

Examples:

```text
NTE-00000001
FIL-00000002
ACC-00000003
CAL-00000004
LLM-00000005
```

Core intent:

```text
Users can directly reference objects
LLMs can stably identify objects
Cross-module associations have a unified representation
Metadata search results are more readable
Avoid exposing internal database ids
```

Basic boundaries:

```text
Internal data relationships still use database ids
ref_code is mainly used for display, owner-only metadata query, search, LLM, and manual references
object_refs authoritatively maintains ref_code and title/tags/status metadata projections
Exact ref_code and recent object metadata responses both return title, tags, status; tags is an empty array when there are no tags
ref_code uses NTE / FIL / ACC / CAL / LLM module prefixes; object types are additionally expressed by object_type
The first version Search metadata query only allows resource owners access
Prioritize allocating to first-class business objects like Note, File, EventAggregate, Event, Account, Transaction, Collection
Does not require relationship tables, configuration items, sessions, or search index rows to have ref codes
Do not treat ref code as the sole source of business logic
Do not bypass service layer permission checks via ref code
```

Conceptual location:

```text
internal/platform/ref
```

For a detailed summary, see [../OBJECT_REF_CODE.md](../OBJECT_REF_CODE.md).
