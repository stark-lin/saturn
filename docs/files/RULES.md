# FILES Rules and Constraints

> Split from `docs/FILES.md`: Original Chapters 18-21 and Chapter 23. Current baseline scope is summarized in `docs/PRD.md`; file structure scope is summarized in `docs/FILES.md`.

## 18. Dependency Direction

### 18.1 Allowed Dependency Directions

```text
cmd/server
    ↓
internal/app
    ↓
business modules
    ↓
internal/platform
    ↓
external infrastructure
```

The overall dependency graph must remain a Directed Acyclic Graph (DAG). Dependencies can only flow from the upper layer to the lower layer as shown above; a lower layer package must not directly or indirectly import an upper layer package; no whitelist exceptions may form circular dependencies.

Basic business call pattern:

```text
handler
    ↓
service
    ↓
repo / platform service
    ↓
db / Redis-backed platform service / local filesystem storage / LLM provider adapter
```

### 18.2 Allowance Rules

```text
app can depend on all modules
business modules can depend on platform
business modules in principle do not directly depend on other business modules, exceptions must enter the whitelist
platform must not depend on business modules
The dependency graph must remain directed and acyclic
Lower layers must not depend on upper layers
handlers do not access repos directly
handlers do not access Redis directly
handlers do not access DB drivers directly
```

### 18.3 Business Module Dependency Whitelist

Cross-business module dependencies are only allowed via the other's exported service / facade or explicitly registered contributors; depending on the other's repo, handler, or internal SQL is not allowed.

| Caller | Allowed Dependencies | Rules |
| --- | --- | --- |
| `accounting` | No default business dependencies | tags and refs in the minimal ledger only reuse platform capabilities; FIL / NTE text in notes does not establish cross-module dependencies |
| `notes` | `files` | Associates Note attachments only through the service / facade; does not read the files repo or local FS storage implementations |
| `calendar` | No default business dependencies | The current core model only has EventAggregate / Event; if associating external resources in the future, only save ObjectRef-level references, and orchestrate via app-layer facades when displaying external resources |
| `llm` | `accounting`, `notes`, `files`, `calendar` | Orchestrates read-only LLM request capabilities only via business services / facades; does not access repos directly, does not bypass auth / audit; provider calls are executed asynchronously via PostgreSQL-backed LLM workers |
| `platform/search` | `platform/ref` | Exposes ObjectRef metadata HTTP handlers only; does not index source records or call source module repos |

Un-whitelisted business module dependencies are prohibited by default.

### 18.4 auth scope and SQL Rules

`platform/auth` does not concatenate business SQL, nor does it understand business table structures.

Authorization is divided into two categories:

```text
Single resource write operations / detail reading:
  service reads resource facts first
  service calls auth authorizer
  service then executes business action

Lists / searches:
  service calls auth scope
  repo applies scope using this module's fixed SQL helper

ObjectRef metadata search:
  platform/search handler calls platform/ref metadata services
  platform/ref applies owner-only metadata visibility
```

SQL bloat control:

```text
Each module retains at most one repo_access.go carrying visibility filtering helpers
All list/search/recent queries reuse the same auth scope helper
Permission helpers only handle visible set filtering, not complete business policies
Write operations do not use massive WHERE conditions to express complete authorization rules
SQL uses parameterized queries, not concatenating user input
Concatenating fixed SQL fragments within the code is allowed; concatenating user input, field names, table names, or permission expressions is not allowed
```

sqlc generation rules:

```text
Persisted query templates for existing real database behavior, or existing repository contracts supported by schema, are written to the owning module's queries/*.sql
queries use sqlc named queries, and generated output goes into the same module's sqlc/ subpackage
Generated code is committed to the repository and must not be edited manually
repo.go adapts generated parameters/results to domain models; the service does not call the generated package directly
Template generated packages do not constitute an available route, service wiring, or SQLRepository implementation
sqlc generated subpackages comply with the owning module's dependency direction and arch-go coverage rules
Finite query variants of scopes are chosen as fixed named queries by the repo; user-controlled SQL is not dynamically generated
go tool sqlc generate must be run after schema or query changes
Centralized business query generation packages must not be created under platform/db
Empty queries/ or sqlc/ directories must not be created ahead of time for data access lacking an existing repository contract
Generated packages must not contain other business module models unused by this module's queries
```

Recommended read scopes are kept simple:

```text
superuser: all
owner: owner_id = actor_id
shared: exists share row
```

Object `status` is only used for status projection, not for relaxing read scopes. If owner/shared is insufficient later, a unified authorization table (e.g., `resource_grants`) can be considered. The current baseline does not introduce general ACL expression systems.

### 18.5 Prohibited Dependencies

```text
platform → business module
lower layer → upper layer
any circular dependency
business module → app
handler → redis client
handler → db driver
handler → local filesystem storage implementation
files → llm
notes → llm
accounting → llm
calendar → llm
llm → business module repo
ops view copying other modules' business logic
ops view → Redis client
ops view → DB driver
```

---

## 19. Naming Conventions

### 19.1 File Naming

Use lowercase words with underscores:

```text
service.go
service_test.go
current_user.go
request_id.go
```

Prohibited:

```text
Service.go
serviceImpl.go
currentUser.go
```

### 19.2 Go Type Naming

Structs, interfaces, and functions use Go standard naming styles:

```go
type Service struct {}
type Repository struct {}
type Handler struct {}
type CreateNoteRequest struct {}
type CreateNoteResponse struct {}
```

Go initialisms use standard uppercase forms:

```text
ID
HTTP
URL
SQL
API
SSE
JSON
```

Examples:

```go
type RequestID string
func ParseURL(raw string) (*url.URL, error)
func WriteJSON(w http.ResponseWriter, v any) error
```

Prohibited:

```text
RequestId
HttpClient
UrlValue
SqlStore
```

Package naming rules:

```text
Package names are short, all lowercase, no underscores
Do not use common, shared, utils as catch-all packages
Interfaces are preferentially defined on the consumer side
Define interfaces only when needing to replace implementations, isolate tests, or express stable boundaries
```

### 19.3 Module Initialization Naming

Each module uses `Module` to represent the module assembly structure:

```go
type Module struct {
    Handler *Handler
    Service *Service
}
```

---

## 20. Test File Standards

For detailed test standards, see [TESTING_LOGGING.md](../TESTING_LOGGING.md).

Default test files:

```text
service_test.go
```

Complex modules can add:

```text
repo_test.go
handler_test.go
```

Rules:

```text
Prioritize testing the service layer
repo tests only cover critical SQL behaviors
Run go tool sqlc generate first when modifying query SQL or related schemas
handler tests only cover request binding, authentication, and response formats
Do not write tests for simple getters/setters
Production code does not provide memory/db replacement backends
Test code allows fakes / mocks / test containers
Fakes are only placed in _test.go or test helpers
go test ./... runs by default after Go code changes
```

---

## 21. Logging Standards

For detailed logging standards, see [TESTING_LOGGING.md](../TESTING_LOGGING.md).

Rules:

```text
Use log/slog structured logging
The app layer creates loggers
Components needing logs explicitly receive loggers via constructors
Consumer side defines minimal Logger interfaces
Code identifiers in log messages, field names, and values must use English
Fields use lower snake case
The error field name is fixed as "error"
Ordinary loggers do not replace audit logs
Do not log passwords, tokens, cookies, session ids, raw content, raw LLM prompts / responses
Do not log-and-return at every layer; prioritize centrally logging errors at boundary layers
```

Common fields:

```text
request_id
actor_id
module
operation
resource_type
resource_id
ref_code
job_id
provider
status
duration_ms
count
error
```

---

## 22. Prohibited Items

The following structures are prohibited from being introduced into the project:

```text
internal/domain
internal/usecase
internal/infrastructure
internal/delivery
internal/adapter
internal/common
internal/shared
pkg/common
pkg/utils
```

Unless there is already a clear, stable, irreplaceable responsibility boundary.

The following implementation methods are prohibited:

```text
Redis optional switch
memory/db/redis multi-backends
Business modules directly operating Redis
Business modules directly operating external SDKs
ops view rewriting business logic
LLM bypassing business services
Cross-module circular dependencies
Splitting directories early for future requirements
Committing empty packages / empty routes / empty migrations for unimplemented modules
```

---

---

## 23. Final Structure Constraints

Project structure must satisfy:

```text
PostgreSQL is required
Redis is required
The local FS file storage directory is the default blob storage
Development defaults to running app + PostgreSQL + Redis + local FS volume via root docker-compose.yml
Later stages may consider adjusting deployment forms
Business modules do not directly touch Redis
ops views reuse normal service paths, not accessing Redis or business repos directly
platform/search is an ObjectRef metadata HTTP capability, not a source-module indexing service
Accounting, Notes, Files, Calendar, Platform, and read-only queued request/result LLM must keep real business closed loops
No multi-backends
No redis-less mode
No microservice splitting
No overly deep directory layering
No early abstraction of future capabilities
No committing empty module placeholders with no business behavior
```

This file is the guideline for project file organization. Update this document first when adding directories or changing module boundaries.
