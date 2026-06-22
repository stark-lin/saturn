# Saturn PRD

> Official Project Name: Saturn.
>
> This document is the PRD entry index. Detailed product scope, feature modules, technical trade-offs, and current baseline boundaries have been split by topic to avoid a single overly long PRD.

## Reading Entry

| Document | Content |
| --- | --- |
| [prd/PRODUCT.md](prd/PRODUCT.md) | Project overview, user and permission models, top-level Domains |
| [prd/FEATURES.md](prd/FEATURES.md) | Accounting, Notes, Files, Calendar, LLM, Platform and other feature modules |
| [prd/TECH_DECISIONS.md](prd/TECH_DECISIONS.md) | Technical trade-offs, REST / SSE / WebSocket decisions, system capability composition |
| [OBJECT_REF_CODE.md](OBJECT_REF_CODE.md) | Object Ref Code unified readable object reference code concept summary |

## Splitting Principles

```text
PRD.md keeps the entry and summary
Product scope goes to prd/PRODUCT.md
Feature modules go to prd/FEATURES.md
Technical decisions go to prd/TECH_DECISIONS.md
Current baseline scope stays in PRD.md
```

## Current Baseline Scope

The core product loop is now treated as closed: a runnable personal self-hosted data service with identity, files, notes, accounting, calendar, ObjectRef metadata search, REST, local filesystem storage, PostgreSQL, Redis-backed sessions, and Docker Compose development deployment.

Current included capabilities:

```text
Identity / Auth / Audit
Files
Notes
Accounting
Calendar
Platform Search
Ops UI
Read-only queued request/result LLM
REST API
Docker Compose development deployment
PostgreSQL
Redis
Local filesystem file storage
```

Future extensions:

```text
full LLM write-agent execution
plugin system
mobile app
multi-tenant workspace / organization
embedding / vector search
external bank sync
complex theme marketplace
real-time collaboration
More complete platform/search diagnostics
```

Current LLM boundary:

```text
LLM sessions
LLM messages
authorized reference context assembly
queued provider request processing
result polling
LLM audit trail
```

Outside the current LLM boundary:

```text
LLM directly executing formal write operations
LLM bypassing target module services
LLM agent autonomously batch modifying data
plugin runtime
embedding / vector search
```

## Final Summary

*Saturn is a local-first, single-instance-friendly, self-hosted personal data service.*

Its core value is to unify personal data that is usually scattered across different services—integrating files, notes, tasks, reminders, manual accounting, and platform capabilities into a service under your own control, and connecting to LLMs through stable backend interfaces.

The core system pipeline is:

```text
Scattered Personal Services
→ Self-hosted Unified Personal Data Service
→ API
→ LLM-ready Backend
```

The top-level domains are:

```text
Accounting
Notes
Files
Calendar
LLM
Platform
```

Key design decisions:

```text
The main body of the system is a self-hosted unified personal data service.
Personal use is the first priority.
Development defaults to deploying the development environment via the root `docker-compose.yml`.
Default components include the Go app, PostgreSQL, Redis, and a local FS file storage directory.
Splitting default components into multi-container deployments can be considered later.
No multi-tenancy.
Enterprise collaboration, organization management, or public SaaS are not the main goals.
High-sensitivity data such as emails, passwords, keys, investments, medical, legal, and identity documents are not handled.
The core goal is to unify scattered personal services and connect them to LLMs.
The REST API is the core interface.
No WebSocket.
LLM is integrated via backend interfaces and controlled reference context assembly.
LLM does not directly access the database, Redis, or local FS storage.
LLM is not designed as an agent that can read or operate on high-sensitivity data.
Resource-level authorization is executed at the service layer.
Access does not generate business SQL; the repo only applies fixed scopes.
Platform/Search is currently an ObjectRef metadata capability for exact reference lookup, filtered metadata search, and recent-object metadata.
Platform/ObjectRef can provide unified readable reference codes for important objects, and return title/tags/status projections as a unified owner-only metadata result for user reference, LLM calls, search, and cross-module association.
Calendar is reduced to EventAggregate / Event: Aggregate owns immutable metadata and tags, and specific events own start time, duration, immutable metadata, tags, and scheduled/finished/voided status.
Life Records is reduced to Accounting / Bookkeeping.
Accounting only records and aggregates; it does not provide investment advice or automatic bank synchronization.
```
