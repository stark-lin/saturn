# FILES.md

## 1. Goal

This document is the entry index for the project directory structure, file responsibilities, module boundaries, and dependency rules. Details are split by topic into the following sub-documents.

The project adopts a Go modular monolith structure. The system takes personal self-hosting as its main use case. During development, it defaults to running the app, PostgreSQL, Redis, and a local FS file storage volume via the root `docker-compose.yml`. Redis is used only for Session state management and remains a required infrastructure component for authentication; a Redis-less operation mode is not provided. Adjustments to deployment forms can be considered later, but the default infrastructure constraints will not be changed.

---

## 2. General Principles

### 2.1 File Organization Principles

1. The directory structure should directly reflect system module boundaries.
2. Keep an independent directory for each business module.
3. `internal/app` is only responsible for wiring; it does not implement business rules.
4. `internal/platform` is a collection of underlying support packages for the business service layer.
5. Business modules must not directly depend on Redis, local FS storage implementations, LLM SDKs, or specific external service clients; the overall dependency graph must remain a directed acyclic graph (DAG), and lower layers must not depend on upper layers.
6. Do not pre-establish complex directories for future extensions.
7. Do not maintain multiple sets of backends for the same capability.
8. Do not use overly deep Clean Architecture layered directories.
9. Do not use microservice-style directory splitting.
10. Operations entry points only serve as aggregation displays; they must not re-implement the business logic of other modules.
11. Handlers do not carry resource-level permission rules.
12. Services must serve as the primary boundaries for business rules, resource-level authorization, and audit logging.
13. The current baseline only commits code that has actual business behavior; do not create empty packages, empty services, empty repos, or empty handlers for unimplemented modules.
14. Modules can be planned in documentation, but corresponding directories and routes are created only when they enter the current implementation scope.
15. File splitting is based on business behaviors, not aimed at accumulating file counts.

Initial bootstrap exception:

```text
The initial code inventory may create documented scaffold files to establish the repository shape.
Each created source/config/script/migration file must start with a one-line English responsibility comment.
Scaffold code must compile and must not register unfinished API behavior as available functionality.
After bootstrap, new modules follow the normal no-empty-placeholder rule.
```

Current baseline file scope:

```text
cmd/server/main.go
Dockerfile
docker-compose.yml
.env.example
config.example.json
docker
scripts
internal/app
internal/platform
internal/accounting
internal/notes
internal/notes/rss
internal/files
internal/calendar
internal/llm
migrations
web/src
```

### 2.2 Redis Principles

Redis is a required system component, used only for:

```text
session
```

The system does not provide the following alternative implementations:

```text
database-backed session store
optional redis mode
```

Business modules must not use Redis for cache, rate limit, job queue, temporary state, or SSE transient state. LLM asynchronous tasks use PostgreSQL `FOR UPDATE SKIP LOCKED` to claim queued requests.

Production runtime only provides a Redis-backed session store. Test code can use fakes, mocks, or test containers in `_test.go`, but must not expose a memory/database session backend as a selectable runtime implementation.

---

## 3. Split Documents

| Document | Content |
| --- | --- |
| [MODULE_ER.md](MODULE_ER.md) | Module ER diagram, module-owned objects, platform collaboration boundaries, and dependency directions |
| [ER.md](ER.md) | Data ER diagram, table groupings, modeling rules, and Object Ref Code conceptual model |
| [API.md](API.md) | Cross-module API conventions, endpoint state rules, and module API document index |
| [api/PLATFORM.md](api/PLATFORM.md) | Currently implemented Platform/Auth/SSE endpoint contracts |
| [files/ROOT.md](files/ROOT.md) | Root directory structure, root directory file conventions, docs / migrations / web conventions |
| [files/APP.md](files/APP.md) | `internal/app` wiring layer file responsibilities |
| [files/PLATFORM.md](files/PLATFORM.md) | `internal/platform` lower-layer support package responsibilities |
| [files/MODULES_CORE.md](files/MODULES_CORE.md) | Business module common structure, Accounting, Notes, Files, Calendar, LLM, Platform |
| [files/MODULES_EXTENDED.md](files/MODULES_EXTENDED.md) | Subsequent extension rules for top-level business modules |
| [files/RULES.md](files/RULES.md) | Dependency directions, naming conventions, testing conventions, prohibitions, final structural constraints |
| [TESTING_LOGGING.md](TESTING_LOGGING.md) | Engineering standards for writing tests and structured logging |
| [OBJECT_REF_CODE.md](OBJECT_REF_CODE.md) | Object Ref Code unified readable object reference code concept summary |

## 4. Maintenance Rules

```text
FILES.md only keeps the entry, global principles, and split index.
When adding directories or changing module boundaries, update corresponding documents under docs/files.
When changing the current product baseline, update PRD.md and the affected split documents.
When changing dependency directions or naming rules, update files/RULES.md.
When changing testing or logging rules, update both TESTING_LOGGING.md and files/RULES.md simultaneously.
When adding or modifying HTTP endpoints, update the corresponding api/<MODULE>.md covered by API.md conventions.
```
