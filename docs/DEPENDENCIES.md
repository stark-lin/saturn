# DEPENDENCIES.md

## 1. Goal

This document records Saturn's external dependency boundaries. Specific versions are fixed when the code and lockfiles are introduced.

---

## 2. Required Infrastructure

```text
Go
PostgreSQL 17
Redis
Local filesystem storage under configured storage.root
```

Redis is strictly required for:

```text
administrator browser sessions
```

LLM requests, once submitted, are stored in PostgreSQL `llm_requests`, claimed by a fixed number of in-app workers using `FOR UPDATE SKIP LOCKED`, and the `success/error` results are written back to PostgreSQL; the current baseline does not configure retries or dead-letters. If a single provider call exceeds `llm.timeout_seconds`, an `error` is written with `error_code = llm_request_timeout`.

---

## 3. Web

```text
web/src plain native HTML pages
REST API access through browser forms or minimal vanilla JavaScript
SSE client only when required by a real flow
shared frontend files only after real reuse exists; current reuse is native app shell, layout, and UI primitives
```

The current frontend baseline does not introduce frameworks, component libraries, CSS frameworks, bundlers, or Node frontend build steps. If frontend frameworks or build tools are genuinely needed later, they must be established via a new architecture decision.

Markdown browser rendering for Notes uses fixed-version resources served identically from the Go static Web entry, and are not loaded from CDNs at runtime:

```text
marked 15.0.12      -> web/src/vendor/marked.min.js  -> /vendor/marked.min.js
DOMPurify 3.4.5     -> web/src/vendor/purify.min.js  -> /vendor/purify.min.js
```

`marked` is only responsible for parsing Markdown source content into HTML; it must be sanitized by `DOMPurify` before being inserted into the DOM. Both are browser runtime resources without build steps and do not introduce frontend frameworks or component systems.

---

## 4. Initial stack lock

The initial code inventory fixes the first implementation stack as:

```text
Go module: github.com/stark-lin/saturn
Backend: Go net/http handlers with Gin HTTP routing
Frontend: plain native HTML first
Frontend HTTP: browser forms or minimal vanilla JavaScript fetch when required
Frontend streaming: authenticated SSE uses minimal vanilla JavaScript fetch streaming
Frontend build: no React / Vite / Node build step in the initial pass
```

This stack can be revisited by a later architecture decision if the frontend or backend requirements outgrow the current HTTP/static setup.

HTTP route matching, path grouping, and route-level middleware composition use:

```text
github.com/gin-gonic/gin v1.12.0
```

Gin is limited to the `internal/app` routing boundary. Business and platform handlers retain the standard `net/http` contract; the app adapter maps Gin path parameters into `http.Request.PathValue`, and the existing structured request logging, request ID, recovery, authentication service, static file serving, and SSE handlers remain authoritative.

Backend PostgreSQL access uses Go `database/sql` with:

```text
github.com/jackc/pgx/v5/stdlib
```

Business modules must not import the PostgreSQL driver directly; runtime database connections and transactions are encapsulated by `internal/platform/db`. Schema bootstrap belongs to the PostgreSQL container initialization boundary, not the Go process.

Implemented repository SQL and persisted query templates for existing repository contracts use the following fixed-version development tool to generate type-safe calling code:

```text
github.com/sqlc-dev/sqlc/cmd/sqlc v1.31.1
```

Rules:

```text
sqlc continues to generate database/sql invocation code without changing the runtime PostgreSQL driver
Query SQL belongs to the module owning the data, placed in the module's own queries/ directory
Generated output is placed in the same module's sqlc/ sub-package and committed to the repository; do not edit manually
internal/platform/db does not carry business queries or act as a unified generated package
Query templates do not imply that the corresponding service, route, or SQLRepository has completed runtime wiring
```

Generation command:

```sh
go tool sqlc generate
```

Local password hashing uses:

```text
golang.org/x/crypto/bcrypt
```

Calendar's synchronous textual ICS import uses fixed-version, pure Go parsing and recurrence libraries:

```text
github.com/arran4/golang-ical v0.3.5
github.com/teambition/rrule-go v1.8.2
```

`golang-ical` parses iCalendar components and properties. `rrule-go` expands bounded RFC recurrence rules and combines RRULE, RDATE, and EXDATE values. Calendar owns UID grouping, RECURRENCE-ID override application, the 512-Event limit, validation, authorization, ObjectRef synchronization, and auditing; neither dependency performs network calls or data persistence.

---

## 5. Runtime Configuration

Application runtime configuration defaults to using a JSON file:

```text
config.json
```

The repository commits `config.example.json` as a template, but the real `config.json` is not committed. When `config.json` is missing, the application generates a local configuration using built-in defaults and current `SATURN_*` environment variables; if an existing config file fails to parse, it must fail fast.

Database configuration is located in the `database` section:

```json
{
  "database": {
    "url": "postgres://saturn:saturn@localhost:5432/saturn?sslmode=disable"
  }
}
```

The configured URL must use the least-privilege `saturn` runtime role. Runtime configuration has no migration URL and no drop/rebuild switch; the Go process cannot create, alter, or drop the schema.

Startup dependency readiness is configured in the `startup` section:

```json
{
  "startup": {
    "readiness_timeout_seconds": 30
  }
}
```

At process startup, PostgreSQL and Redis readiness checks run concurrently. The main startup flow blocks until both are ready, then wires services using the already initialized schema and starts the HTTP server and workers. If either dependency remains unavailable past `startup.readiness_timeout_seconds`, startup fails fast without entering a degraded mode.

Authentication configuration includes the administrator JWT signature secret and token validity in minutes. Saturn access API keys are generated at runtime and persisted only as SHA-256 digests in PostgreSQL; they have no configuration-file secret. The repository template and Docker development config still use a development JWT secret, but when `config.json` does not exist the startup path generates a random `auth.jwt_secret` and writes it to disk. Existing config files must provide `auth.jwt_secret` and `auth.token_ttl_minutes` explicitly.

LLM configuration is located in the `llm` section:

```json
{
  "llm": {
    "enabled": false,
    "api_key": "",
    "endpoint": "https://api.openai.com/v1/chat/completions",
    "model": "gpt-4o-mini",
    "rate_limit_per_minute": 60,
    "max_tokens": 1024,
    "worker_count": 1,
    "timeout_seconds": 60
  }
}
```

When `llm.enabled = true`, you must configure `llm.api_key`, `llm.endpoint`, `llm.model`, `llm.rate_limit_per_minute`, `llm.max_tokens`, `llm.worker_count`, and `llm.timeout_seconds`. `llm.worker_count` controls the fixed number of workers executing provider calls concurrently. The API key is a secret and is not written to logs, audits, or normal API responses.

Environment variables are only used for the initial generation of the config file and do not serve as a long-term runtime override mechanism.

You can use the following when generating the config file for the first time:

```text
SATURN_STARTUP_READINESS_TIMEOUT_SECONDS
SATURN_LLM_ENABLED
SATURN_LLM_API_KEY
SATURN_LLM_ENDPOINT
SATURN_LLM_MODEL
SATURN_LLM_RATE_LIMIT_PER_MINUTE
SATURN_LLM_MAX_TOKENS
SATURN_LLM_WORKER_COUNT
SATURN_LLM_TIMEOUT_SECONDS
```

For local development using Homebrew PostgreSQL 17, start the service first:

```sh
brew services start postgresql@17
```

The default development configuration uses:

```text
postgres://saturn:saturn@localhost:5432/saturn?sslmode=disable
```

When using local PostgreSQL instead of Compose, provision the complete schema and the `saturn_owner` / `saturn` split with a privileged migration process before starting Go. Do not make `saturn` the database or table owner. The Compose initialization under `docker/postgres/` is the authoritative executable example.

If the local PostgreSQL uses passwordless login for the current macOS user, you can set `SATURN_DATABASE_URL` before generating `config.json` for the first time, or directly modify the uncommitted local `config.json`.

When running the Go process directly for local development, Redis must also be started first. The default development config uses explicit IPv4 loopback to prevent `localhost` on macOS from resolving to an unlistening IPv6 `::1`:

```text
127.0.0.1:6379
```

You can use Docker Compose to start only Redis:

```sh
docker compose up -d redis
```

---

## 6. Dependency Rules

Files blob storage uses the local file system, with the default directory being `./objects`, which can be specified via `storage.root` or `SATURN_STORAGE_ROOT` when the config is first generated.

Business modules must not directly depend on the Redis client, LLM SDK, or specific external service clients; Files must access local FS storage capabilities via `internal/platform/storage`. The Redis client is only used by the administrator auth session store; API-key authentication uses PostgreSQL.

External service clients can only be encapsulated in the corresponding packages within `internal/platform`.

Overall code dependencies must remain a directed acyclic graph (DAG). Lower-layer packages cannot directly or indirectly depend on upper-layer packages; cross-module dependency whitelists can only permit directions that conform to the DAG.

---

## 7. Development and Architecture Check Tools

```text
arch-go
sqlc
```

`arch-go` is used for dependency checks, ensuring Go package imports comply with layered dependency rules. Rules should be maintained in `arch-go.yml` at the repository root, and must at least cover:

```text
cmd/server -> internal/app
internal/app -> business modules / internal/platform
business modules -> internal/platform
business module generated sqlc subpackages follow their owning module dependency boundary
internal/platform does not depend on business modules or internal/app
Business module whitelists must not form circular dependencies
```

`arch-go` is a development and CI check tool, not a runtime dependency. The tool version is fixed via `tools.go` and `go.mod` in the root directory. The project check command uses:

```sh
go run github.com/arch-go/arch-go/v2 --color no
```

`sqlc` is a development and CI code generation tool, not a runtime dependency. It is managed via `tool` directives in `go.mod` and fixed module versions. After modifying query SQL or related schemas, you must rerun:

```sh
go tool sqlc generate
```

When introducing tools or updating versions, the version must be fixed and this document synchronized.
