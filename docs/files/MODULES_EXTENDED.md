# FILES Business Module Extensions

> Split from `docs/FILES.md`: Subsequent extension rules for top-level business modules.

## 14. Extension Principles

Top-level business modules remain stable:

```text
Accounting
Notes
Files
Calendar
LLM
Platform
```

New capabilities are preferentially placed back into their owning top-level domain; peer modules are not added for similar capabilities.

Rules:

```text
Ledger accounts and immutable flows belong to Accounting
RSS and templates belong to Notes
Attachments, external links, object references belong to Files
Tasks, events, reminders, recurrence rules belong to Calendar
Tools, contexts, prompts, summaries, audits belong to LLM
Authentication, config, search, object ref codes, storage, audit, and operations views belong to Platform
```

Prohibited:

```text
Creating an independent business module for Admin
Retaining compatibility directories for old module names
```

---

## 15. Accounting Extensions

Allowed extensions:

```text
recurring transaction preview
account reconciliation
```

Boundaries:

```text
Accounting only records and tallies
Account is the ledger; Transaction can only be added or voided
Classification semantics only use ObjectRef tags projection
FIL / NTE references are only kept as Transaction note text
Does not provide financial advice
Does not connect to bank sync
Does not save bank credentials
Does not establish receipt attachments or reminder object links
```

---

## 16. Notes Extensions

Allowed extensions:

```text
note revision diff
note template variables
RSS source diagnostics
collection navigation
backlinks
```

Boundaries:

```text
Note attachments are handled through the Files facade
Collection is not upgraded to a global system; Tag uses ObjectRef tags projection
```

---

## 17. Files Extensions

Allowed extensions:

```text
file versioning
preview generation
thumbnail generation
trash retention
share link expiration
attachment ownership checks
object reference diagnostics
```

Boundaries:

```text
Files currently owns immutable Collection and File metadata; Tag uses ObjectRef tags projection
Files does not write directly to the local FS
Files uses local FS storage capabilities via platform/storage
Files does not understand Accounting / Notes / Calendar business rules
```

---

## 18. Calendar Extensions

Allowed extensions:

```text
additional calendar views
conflict diagnostics
external object links through ObjectRef
future reminder dispatch
future recurrence expansion jobs
```

Boundaries:

```text
The current core model is EventAggregate + Event; extensions must not bypass the aggregate boundary
Calendar does not directly read Accounting / Notes / Files repos
Reminders or background expansions should go through the owning module's own PostgreSQL-backed worker until a real scheduler is implemented
Time judgment stays local to the owning service until a real shared clock abstraction is needed
```

---

## 19. LLM Extensions

Allowed extensions:

```text
read-only tools
draft-only tools
context packs
prompt versioning
summary cache
tool call audit
safety review
```

First version prohibited:

```text
Formal write operations automatically executing
Bypassing business services
Bypassing permission checks
Bypassing preview / safety checks
Direct access to DB / Redis / local FS storage implementations
```

---

## 20. Platform Extensions

Platform carries horizontal product capabilities:

```text
Auth
Config
ObjectRef
Reference Metadata Search
Storage
```

Allowed extensions:

```text
object ref code diagnostics
storage usage report
ops dashboard
```

Boundaries:

```text
Platform/Search does not own source business records
Platform/ObjectRef does not replace internal ids, nor bypass business services
Platform/Storage does not own Files business rules
Ops pages do not copy business module rules
Platform/Auth does not generate business SQL
Future platform capabilities should not be added as empty packages before real behavior exists
```
