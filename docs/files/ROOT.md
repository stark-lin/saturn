# FILES Root Directory and Documentation

> Split from `docs/FILES.md`: Root directory structure, root file conventions, docs / migrations / web standards.

## 3. Root Directory Structure

This project adopts a Go modular monolith backend and a `web/src` frontend source directory. Directories are created only when entering the implementation scope; empty packages or empty pages are not committed for unimplemented capabilities.

```text
.
├── cmd
│   └── server
│       └── main.go
│
├── internal
│   ├── app
│   ├── accounting
│   │   ├── queries
│   │   └── sqlc
│   ├── notes
│   │   ├── queries
│   │   ├── rss
│   │   └── sqlc
│   ├── files
│   │   ├── queries
│   │   └── sqlc
│   ├── calendar
│   ├── llm
│   │   ├── queries
│   │   └── sqlc
│   └── platform
│       ├── audit
│       │   ├── queries
│       │   └── sqlc
│       ├── auth
│       │   ├── queries
│       │   └── sqlc
│       ├── config
│       ├── db
│       ├── httpx
│       ├── logger
│       ├── redis
│       ├── ref
│       │   ├── queries
│       │   └── sqlc
│       ├── search
│       └── storage
│           ├── queries
│           └── sqlc
│
├── web
│   └── src
│       ├── app
│       ├── pages
│       │   ├── accounting
│       │   ├── files
│       │   ├── llm
│       │   ├── notes
│       │   ├── calendar
│       │   └── settings
│       └── shared
│           ├── api
│           ├── components
│           ├── layout
│           └── utils
│
├── migrations
├── docs
│   ├── api
│   ├── decisions
│   ├── files
│   └── prd
├── scripts
├── docker
├── .env.example
├── config.example.json
├── AGENTS.md
├── Dockerfile
├── README.md
├── docker-compose.yml
├── go.mod
├── go.sum
└── sqlc.yaml
```

`calendar` is the new directory name for the original `planner` module. Code, API, migrations, and documentation use the standard English spelling `calendar`, not `calender`.

---

## 4. Root Directory File Conventions

### 4.1 `cmd/server`

```text
cmd/server/main.go
```

Responsibilities:

```text
Load configuration
Initialize application
Start HTTP server
Handle process exit signals
```

Prohibited:

```text
Implementing business logic
Directly accessing the database
Directly accessing Redis
Directly registering specific business handlers
```

---

### 4.2 `internal`

```text
internal
├── app
├── accounting
├── notes
│   └── rss
├── files
├── calendar
├── llm
└── platform
```

Responsibilities:

```text
internal/app is responsible for dependency assembly, route registration, and lifecycle management
accounting, notes, files, calendar, llm are top-level business modules
notes/rss carries Notes sub-capabilities like RSS source parsing, synchronization, and diagnostics
platform carries horizontal support capabilities that business services can depend on
```

Rules:

```text
Business rules must go into the corresponding business module service
handlers are only responsible for parameter binding, reading actors, and writing responses
repos only do data access
platform does not depend on business modules
Cross-business module calls can only go through exported services / facades or contributors
```

---

### 4.3 `web`

```text
web
└── src
    ├── app
    ├── pages
    │   ├── accounting
    │   ├── files
    │   ├── llm
    │   ├── notes
    │   ├── calendar
    │   └── settings
    └── shared
        ├── api
        ├── components
        ├── layout
        └── utils
```

Responsibilities:

```text
app: Frontend application entry point, routing, global providers, and startup assembly
pages: Page-level compositions, organized by routes and business entries
shared/api: API clients, request encapsulation, and SSE clients
shared/components: Cross-page reusable UI components; currently includes primitives and system clock
shared/layout: Application shell, Top Banner, Control Rack, Global Search, and layout components
shared/utils: Frontend utility functions with no business affiliation
```

Rules:

```text
pages can compose shared modules
shared does not depend on pages
The frontend interacts with the backend via REST API and SSE
The frontend does not directly access the database, Redis, local FS storage, or LLM providers
Cross-module UI must first confirm true reusability before entering shared
```

Current frontend baseline standards:

```text
The current frontend baseline uses native HTML pages, prioritizing the simplest pages that can run through business flows
Do not introduce frontend frameworks, bundlers, CSS frameworks, component libraries, or complex layout systems
Do not invest in implementation for typography, visual design, responsive polish, or theme systems
Prioritize using browser-default forms / inputs / buttons / links / lists / tables
Add minimal vanilla JavaScript only when REST APIs, SSE, or small local interactions require it
Directories under web/src are only created when real pages need them, not rolled out in advance as framework scaffolding
```

---

### 4.4 `docs`

```text
docs
├── PRD.md
├── MODULES.md
├── FILES.md
├── API.md
├── ER.md
├── PERMISSION.md
├── DEPLOY.md
├── DEPENDENCIES.md
├── OBJECT_REF_CODE.md
├── api
│   ├── ACCOUNTING.md
│   ├── CALENDAR.md
│   ├── FILES.md
│   ├── LLM.md
│   ├── NOTES.md
│   ├── PLATFORM.md
│   └── SETTINGS.md
├── decisions
│   └── README.md
├── files
└── prd
```

Responsibilities:

| File / Directory | Responsibility |
| --- | --- |
| `PRD.md` | PRD entry index and summary |
| `MODULES.md` | Module division and dependency relationship summary |
| `FILES.md` | File structure and directory standards entry index |
| `API.md` | Cross-module HTTP API conventions, status standards, and module doc indices |
| `ER.md` | Data models and table relationships |
| `PERMISSION.md` | Permission models |
| `DEPLOY.md` | Deployment methods |
| `DEPENDENCIES.md` | External dependencies and version constraints |
| `OBJECT_REF_CODE.md` | Object Ref Code conceptual summary |
| `api/` | HTTP endpoint contracts split by owner module |
| `decisions/` | Architectural decision records |
| `files/` | File structure and directory boundary detailed documents |
| `prd/` | Product scope, feature, and technical decision detailed documents |

---

### 4.5 `migrations`

```text
migrations
├── 000001_init.sql
├── 000002_auth.sql
├── 000003_files.sql
├── 000004_notes.sql
├── 000005_accounting.sql
├── 000006_calendar.sql
├── 000007_platform_search.sql
├── 000008_platform_import_export.sql
├── 000009_platform_storage.sql
├── 000010_llm.sql
└── 000011_settings.sql
```

Rules:

```text
Migration files increment numerically
One migration file corresponds to one explicit subject
Modifying migration files that have already been merged into the main branch and released is prohibited
All schema changes must be done through migrations
```

Naming format:

```text
NNNNNN_subject.sql
```

---

### 4.6 `sqlc.yaml` and repository queries

```text
sqlc.yaml
internal/{module}/queries/*.sql
internal/{module}/sqlc/*.go
internal/platform/{capability}/queries/*.sql
internal/platform/{capability}/sqlc/*.go
```

Responsibilities:

```text
sqlc.yaml defines PostgreSQL schema inputs, query inputs, and Go generation outputs
queries/*.sql stores named SQL queries owned by the module
sqlc/*.go stores database/sql call code generated by sqlc
repo.go calls the generated code and maps it to module domain models
```

Rules:

```text
migrations are the schema source for sqlc
Only create queries/ and sqlc/ for modules / platform capabilities that have real database behavior or defined repository contracts with schema ownership
Merely having generated query templates does not imply the corresponding route or repository adapter is assembled
Generated code is committed to the repository and not manually modified
Run go tool sqlc generate after every modification to queries or related migrations
Centralized business query generation packages under internal/platform/db are prohibited
Generation config must avoid outputting data models unused by one module to another
```

---

### 4.7 `scripts`

```text
scripts
```

Responsibilities:

```text
Stores development, build, migration, and operations utility scripts
```

Rules:

```text
Scripts must be runnable repeatedly or clearly state their side effects
Scripts must not bypass application services to directly modify business data, unless it is explicitly a data migration or repair script
Scripts involving dangerous operations must default to dry-run or require explicit confirmation parameters
```

---

### 4.8 `docker` and Root Deployment Files

```text
Dockerfile
docker-compose.yml
.env.example
config.example.json
docker
```

Responsibilities:

```text
Dockerfile defines the application image
docker-compose.yml defines the development default running topology
config.example.json provides the local JSON configuration template
.env.example provides optional environment variable examples for first-time config.json generation
docker/ stores Docker-related configuration, initialization scripts, and service configurations
```

Rules:

```text
Development defaults to starting the app, PostgreSQL, Redis, and local FS file storage volume via docker-compose
PostgreSQL is a required service
Redis is a required service
The local FS volume is the default file blob storage
Later stages may consider split deployment forms, but default dependency constraints are not altered
```
