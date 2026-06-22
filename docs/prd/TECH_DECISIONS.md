# TECH_DECISIONS.md

> Split from `docs/PRD.md`: Original Chapters 12-14.3.

## 12. Technical Trade-offs

### 12.1 Backend

```text
Go
Modular Monolith
```

The system adopts a Go modular monolith architecture.

This architecture is geared towards single-instance, single-machine deployment, and personal self-hosting scenarios, not targeting microservices, multi-tenant SaaS, or distributed clusters as the default direction.

### 12.2 Database

```text
PostgreSQL
```

PostgreSQL is used to store:

```text
Users
Permissions
File metadata
Notes
RSS sources
Tasks and events
Reminders
Accounting records
Audit logs
Background task records
Search indices
LLM session records
```

### 12.3 Redis

Redis acts as an auxiliary layer, used only for:

```text
session
```

### 12.4 Local Filesystem Storage

File blobs are stored via the local filesystem.

Local FS storage is used for:

```text
Uploaded original files
Images
Videos
Thumbnails
```

PostgreSQL saves metadata, not large file bodies.

The default file storage directory is `storage.root`, with a development default of `./objects`, and the Docker development environment mounts the `/app/objects` volume. PostgreSQL saves metadata, not large file bodies.

### 12.5 Frontend

```text
web/src frontend application
REST API client
SSE client
```

The Web UI adopts the `web/src` frontend source directory, interacting with the backend via REST API and necessary SSE.
Currently, the static entry point returns `Cache-Control: no-store` for HTML, JavaScript, CSS, and vendor resources under `web/src` to ensure that the development client with no build steps reads the latest source code upon page revisit; if production resources with fingerprints are introduced later, a long-term caching strategy will be defined separately.

First version frontend standards:

```text
plain native HTML pages first
browser-default forms, inputs, buttons, links, lists, and tables
minimal vanilla JavaScript only when REST API, SSE, or small local interactions require it
no React / Vue / Svelte
no Vite / Webpack / Node frontend build step
no CSS framework, component library, or layout system
no visual design, complex layout, responsive polish, or theme work in the first version
```

Initial code inventory decision:

```text
plain native HTML first
minimal vanilla JavaScript fetch for REST calls when required
authenticated fetch streaming for SSE flows that require Bearer JWT
no React / Vite / Node build step in the initial pass
```

Notes body uniformly uses Markdown as the source format. Browser preview and display use fixed versions of `marked` and `DOMPurify` scripts delivered with `web/src` and provided homogeneously by the backend static entry point: it first parses the Markdown, then sanitizes the HTML before writing it to the DOM.

If specific frontend frameworks and build tools are needed later, they can be fixed through a new architecture decision.

### 12.6 Deployment

```text
Single Docker Host
Docker Compose
Optional split deployment layout
```

The system defaults to starting via the root directory `docker-compose.yml` during development.

The development default topology includes:

```text
Go app
PostgreSQL
Redis
local filesystem storage volume
```

Later stages may consider adjusting the deployment form of the app, PostgreSQL, Redis, and file storage volume. The deployment form only changes the running method and does not alter the product constraints of PostgreSQL, Redis, and local FS file storage as default components.

Deployment boundaries:

```text
Development defaults to Docker Compose running
Optional single-machine multi-container running later
Defaults to personal self-hosting
Does not require users to maintain multi-container orchestration during development
Does not target Kubernetes as a default deployment target
Does not target multi-node high availability clusters as a default deployment target
Does not target public SaaS multi-tenant operations as a default deployment target
```

### 12.7 Logging

The system uses structured logging.

For specific writing styles, field naming, and obfuscation rules, see [TESTING_LOGGING.md](../TESTING_LOGGING.md).

Focused log types:

```text
request log
job log
storage log
audit log
LLM tool call log
```

---

## 13. REST / SSE / WebSocket Decisions

### 13.1 REST API

REST API is the core interface.

Serving objects:

```text
Web UI
CLI
Mobile App
Automation scripts
LLM backend interaction
```

### 13.2 SSE

The current HTTP surface includes an authenticated `/api/events` SSE transport endpoint. Current business flows do not depend on event-streamed status or streaming payloads.

Future modules that produce business events must document event names and payloads in their owning API contracts before treating the stream as part of that feature.

### 13.3 WebSocket

The system does not do WebSocket.

Current runtime workflows are handled by REST requests and polling where needed.

Conclusion:

```text
REST submits requests
SSE is only a shared transport endpoint in the current baseline
Do not introduce WebSocket
```

---

## 14. System Capability Composition

This section describes the composition of system capabilities. Current baseline boundaries are summarized in `docs/PRD.md`.

### 14.1 Main Closed-Loop Capabilities

```text
Files
Notes
Accounting
Calendar
Platform Search
Ops UI
REST API
```

Main closed-loop capabilities are used to form complete personal file, note, bookkeeping, planning, search, and operations chains.

### 14.2 Extended Business Capabilities

```text
Accounting advanced reports
Calendar advanced recurrence
```

Extended business capabilities are used to supplement more complex statistical reports, subscription renewals, and repeating rule scenarios, connecting with the main closed loop via files, reminders, search, and LLM interfaces.

### 14.3 LLM Enhancement Capabilities

```text
LLM Sessions
LLM Messages
Context Builder
Draft Generation
LLM Audit Trail
```

LLM enhancement capabilities are based on existing data and APIs, providing backend-hosted personal data search, summarization, draft generation, and controlled assisted operations.
