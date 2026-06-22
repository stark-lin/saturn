# TESTING_LOGGING.md

## 1. Goal

This document defines Saturn's engineering standards for writing tests and structured logging. The rules serve two goals:

```text
Tests protect business behaviors, permission rules, and interface contracts.
Logs help locate runtime issues, but do not replace audit records.
```

These standards primarily cover the Go backend. Once an independent testing framework is introduced for the frontend, frontend testing rules should be added to this document.

---

## 2. Testing Standards

### 2.1 Basic Principles

Tests should verify observable behaviors, not implementation details.

Priority:

```text
business rule
authorization rule
API contract
SQL status projection / constraint behavior
background job / import-export / search orchestration
pure helper behavior
```

By default, prioritize testing the service layer. Handler, repo, and platform tests only cover behaviors within their respective boundaries and do not repeat service business rules.

### 2.2 When Tests Must Be Added

| Change Type | Required Tests to Add |
| --- | --- |
| Fix bug | Add regression tests that can reproduce the issue |
| Add or modify service rules | Service unit tests or integration tests |
| Modify resource-level permissions | Permission matrix tests for owner/shared/superuser, etc.; confirm that status does not relax access; resource non-existence and unauthorized access must both externally manifest as `not_found` |
| Modify API requests or responses | Handler tests, and sync corresponding `docs/api/<MODULE>.md`; update `docs/API.md` simultaneously if shared conventions are involved |
| Modify repo SQL, sqlc queries, scope helpers, or migrations | Run `go tool sqlc generate`; add repo tests or integration tests with a real database |
| Modify background jobs or search | Service / orchestration tests, using fake repos |
| Modify logger, request id, error response format | Boundary tests; assert logs only if the log field is a contract |

Tests are not required for simple getters, setters, or assignments to pure data structure fields.

### 2.3 Layered Testing Boundaries

Service tests:

```text
Verify business rules
Verify input validation
Verify resource-level authorization
Verify audit calls
Verify collaboration results of repo / platform dependencies
```

Service tests can use fake repos, fake authorizers, fake clocks, fake storage, and fake queues. Fakes can only be placed in `_test.go` or test helpers, and must not become selectable production backends.

Handler tests:

```text
Verify request binding
Verify actor / session reading
Verify status codes
Verify response JSON shapes
Verify response headers
Verify error code mappings
```

Handler tests do not re-prove business rules. Complex business branches should be handled by fake services returning explicit results.

Repo tests:

```text
Verify critical SQL
Verify permission scope helpers
Verify unique constraints / foreign keys / soft delete filtering
Verify transactional behavior
```

As long as the behavior depends on PostgreSQL semantics, prioritize using real database integration tests instead of using memory fakes to guess SQL behavior. Tests requiring external infrastructure should have clear environment switches and skip information.

Platform tests:

```text
Use unit tests for pure functions
Use boundary tests for HTTP / SSE / queue / storage / scheduler
Use fake servers or mock transports for external service adapters
```

### 2.4 Files and Packages

Default test file:

```text
service_test.go
```

Add as needed:

```text
handler_test.go
repo_test.go
middleware_test.go
import_export_contributor_test.go
search_contributor_test.go
```

Tests should default to using the same package as the code under test to easily cover business boundaries within modules. Only use external test packages like `xxx_test` when testing public API contracts.

### 2.5 Naming and Structure

Single-scenario tests use full behavior naming:

```go
func TestNewModuleBuildsNotesDependencies(t *testing.T)
func TestServiceCreateNoteRejectsMarkdownWithEmptyTitle(t *testing.T)
func TestHandlerCreateNoteReturnsBadRequestForInvalidJSON(t *testing.T)
```

Multi-scenario tests use table-driven tests, letting the case name describe the business scenario:

```go
func TestServiceCanReadNote(t *testing.T) {
    tests := []struct {
        name string
        actorID int64
        ownerID int64
        actorRole string
        wantAllowed bool
    }{
        {name: "owner can read own note", actorID: 1, ownerID: 1, actorRole: "user", wantAllowed: true},
        {name: "other user cannot read note", actorID: 2, ownerID: 1, actorRole: "user", wantAllowed: false},
        {name: "superuser cannot read another owner note", actorID: 9, ownerID: 1, actorRole: "superuser", wantAllowed: false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test body
        })
    }
}
```

Assertions prioritize using the Go standard library:

```go
if got != want {
    t.Fatalf("got %q, want %q", got, want)
}
```

Only introduce additional assertion dependencies when they significantly improve readability for complex comparisons, and synchronize `docs/DEPENDENCIES.md`.

### 2.6 Test Data and Determinism

Tests should satisfy:

```text
Do not depend on pre-existing data on the local machine
Do not depend on test execution order
Do not depend on the passage of real time
Do not depend on true random results
Do not access real external network services
Do not use log output as ordinary test assertions
```

Recommended practices:

```text
Use fixed IDs, fixed times, fixed ref_codes
Use t.TempDir to manage temporary files
Use t.Setenv to manage environment variables
Use t.Cleanup to release resources
Test helpers must call t.Helper()
Isolate shared states before using t.Parallel()
```

Tests involving context cancellation, deadlines, retries, or schedulers should use fake clocks or controllable channels, avoiding bare `time.Sleep` to wait for results.

### 2.7 Error and Log Testing

Error assertions should verify stable contracts:

```text
Error type
Error code
HTTP status
User-visible error message
Whether it is retriable
```

Do not assert full error strings unless the string itself is the API contract.

Ordinary unit tests should discard logs:

```go
log := logger.New(io.Discard, "error")
```

Only capture and assert structured fields using a buffer when the log field itself is a runtime contract, such as `request_id`, `job_id`, or `error`.

### 2.8 Execution Requirements

After changing Go code, run by default:

```sh
go test ./...
```

After modifying `queries/*.sql`, `sqlc.yaml`, or migrations that generated queries depend on, you must update and compile generated code first:

```sh
go tool sqlc generate
```

Changes involving package dependency directions, module boundaries, or new imports must also run architectural dependency checks:

```sh
go run github.com/arch-go/arch-go/v2 --color no
```

`arch-go` uses rules in the repository root `arch-go.yml` for dependency checks. This check should verify that the dependency graph remains a directed acyclic graph, lower layers do not depend on upper layers, and cross-business module whitelists do not form circular dependencies.

You can run a narrower scope during development:

```sh
go test ./internal/notes -run TestServiceCreateNote
```

If tests depend on PostgreSQL, Redis, a local FS storage directory, or other infrastructure and are unavailable locally, explain the reason for skipping. Do not silently turn infrastructure-dependent failures into passes.

`internal/platform/db` PostgreSQL integration tests must be explicitly enabled using a test database connection string:

```sh
SATURN_TEST_DATABASE_URL='postgres://saturn:saturn@localhost:5432/saturn_test?sslmode=disable' go test ./internal/platform/db
```

This test will perform development-phase schema bootstrapping and may drop and recreate known tables in the test database. Do not point it to the local development database or any database with valuable data. When `SATURN_TEST_DATABASE_URL` is not set, PostgreSQL integration tests should `t.Skip` and explain the reason.

`internal/platform/auth` generated query integration tests also use `SATURN_TEST_DATABASE_URL`, but create temporary `users` tables within the current connection without wiping the persistent test schema. This allows proving repository SQL behavior separately from schema bootstrap tests.

### 2.9 Prohibitions

Prohibited:

```text
Only testing that there are no panics
Copying production implementation logic to calculate expected values
Exposing production-only setters or memory backends for testing
Mocking all business behaviors in service tests
Covering massive business matrices in handler tests
Using memory fakes instead of SQL semantics in repo tests
Depending on real networks, real current time, or local private files
Asserting non-contractual log messages in ordinary tests
```

---

## 3. Logging Standards

### 3.1 Basic Principles

Saturn uses Go `log/slog` for structured logging. The application entry point creates JSON loggers via `internal/platform/logger.New`.

Logs are used for runtime diagnostics:

```text
Service startup / shutdown
HTTP requests
Background jobs
Storage operations
Search indexing
LLM provider calls / tool call metadata
Unexpected operational failures
```

Audit logs are used for business auditing:

```text
who did what to which resource and when
only LLM-originated resource reads are audited as READ
SYS-00000000 identifies system-level targets such as LOGIN and LOGOUT
audit_logs is append-only; application code may only INSERT or SELECT, and runtime mutation/truncation is rejected
SUCCESS for a business mutation is inserted in the same PostgreSQL transaction as that mutation
FAILED or DENIED is inserted only after the outcome is known, in a new audit-only PostgreSQL transaction when a prior transaction failed
```

Do not substitute ordinary loggers for audits, and do not log sensitive payloads for the sake of auditing.

### 3.2 Passing Loggers

Rules:

```text
Create the logger at the app layer
Components that need logging explicitly receive the logger via their constructor
Consumer side defines the minimal Logger interface
Do not create global loggers in business packages
Do not use slog.Default() in normal business paths
Default to using io.Discard logger in tests
```

Example:

```go
type Logger interface {
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}
```

If a component only needs `Error`, the interface only defines `Error`. Interfaces are narrowed based on consumer needs.

### 3.3 Log Levels

| Level | Use Cases |
| --- | --- |
| DEBUG | Local diagnostics, high-frequency details, toggleable troubleshooting info |
| INFO | Service lifecycle, request completion, job completion, important state changes |
| WARN | Recoverable exceptions, degradations, success after retries, discouraged-but-functional configurations |
| ERROR | Request failures, job failures, external dependency failures, data consistency risks |

Normal business validation failures do not necessarily require ERROR. Client input errors are typically returned by handlers as 4xx, and INFO or WARN is logged only when operational troubleshooting is needed.

### 3.4 Logging Locations

Recommend logging at boundary layers:

```text
HTTP middleware logs request completion
job runner logs job start / completion / failure
external adapter logs provider / storage / network failure
LLM service logs provider metadata and tool call metadata
```

The service layer logs only under the following circumstances:

```text
A business action triggers important asynchronous flows
Needs to aggregate multiple downstream error contexts
Needs to record non-audit runtime states
```

The repo layer does not log by default; it only returns errors with context. If slow query logging is introduced later, it should be uniformly handled at the DB/platform boundary.

### 3.5 Field Naming

Log messages use English, are brief, stable, and have no periods:

```text
request completed
storage upload failed
llm tool call rejected
```

Fields use lower snake case. Common fields:

| Field | Meaning |
| --- | --- |
| `request_id` | HTTP request id |
| `actor_id` | Current logged-in subject ID |
| `module` | Business module name |
| `operation` | Current operation name |
| `resource_type` | Resource type, e.g., `note`, `file` |
| `resource_id` | Database internal ID |
| `ref_code` | User-visible object reference code |
| `job_id` | Background task ID |
| `provider` | External provider name |
| `status` | HTTP status or job status |
| `duration_ms` | Time spent in milliseconds (number) |
| `count` | Number of items processed |
| `error` | Raw error value |

Multi-word fields should not use camelCase, spaces, or dots.

### 3.6 Sensitive Information

Prohibited from being logged:

```text
password
token
api key
cookie
authorization header
session id
raw note body
raw file content
raw LLM prompt / response
private document text
full request / response body
```

When troubleshooting is needed, prioritize logging:

```text
resource_id
ref_code
content length
file size
mime type
hash prefix
provider request id
count
status
error class
```

### 3.7 Error Logs

Error handling rules:

```text
Bottom layers return errors with context
Boundary layers uniformly log errors
Do not log-and-return at every layer
The error field name is fixed as "error"
Expected context.Canceled / client disconnects are not logged as ERROR
```

Example:

```go
log.Error(
    "storage upload failed",
    "request_id", requestID,
    "actor_id", actor.ID,
    "resource_type", "file",
    "resource_id", fileID,
    "error", err,
)
```

### 3.8 Good and Bad Examples

Good example:

```go
log.Info(
    "job completed",
    "request_id", requestID,
    "actor_id", actor.ID,
    "job_id", jobID,
    "duration_ms", elapsed.Milliseconds(),
    "count", objectCount,
)
```

Bad examples:

```go
log.Info("job done")
log.Error("failed", "data", requestBody)
log.Info("user login", "password", password)
```

Problems with bad examples:

```text
Missing searchable fields
Message is too vague
Logged sensitive payload
```

### 3.9 Relationship Between Logs and Tests

Tests default to not caring about logs. Only assert logs in the following circumstances:

```text
request middleware must include request_id
job runner must include job_id
security events must be degraded or desensitized
a certain field is relied upon by external log systems
```

When asserting logs, only assert structured fields and values, not the full JSON string order.
