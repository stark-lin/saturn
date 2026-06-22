# Decision: Generate Typed Repository Queries With sqlc

## Status

Accepted for repository SQL implementation work.

## Context

Saturn uses PostgreSQL through Go `database/sql` and `github.com/jackc/pgx/v5/stdlib`. The initial repository bootstrap placed the first executable SQL statements in `internal/platform/auth/repo.go`, while most business module repositories still describe boundaries without implemented database behavior.

The project rules assign business SQL to the module that owns the records. `internal/platform/db` owns database connections, transactions, and schema bootstrap; it must not become a shared home for vertical module queries.

## Decision

Saturn uses `sqlc` to generate type-safe Go code from implemented repository queries and from query templates for repository contracts that already exist in source code and have an owned schema table.

Rules:

```text
migrations/ remains the PostgreSQL schema source consumed by sqlc
each owning module keeps hand-written named queries in its own queries/ directory
each owning module keeps generated Go query code in its own sqlc/ subpackage
generated sqlc code is committed and is not edited by hand
repo.go remains the domain-facing adapter and maps generated rows to module models
services depend on repository interfaces, not generated sqlc packages
internal/platform/db remains limited to connections, transactions, and migrations
modules add query and generated-code directories only for implemented persistence or an existing repository contract backed by migrations
generated templates do not make an unfinished service, route, or repository adapter available at runtime
```

The initial adoption migrates the already implemented authentication user queries:

```text
internal/platform/auth/queries/users.sql
internal/platform/auth/sqlc/
```

Current generated query packages are limited to implemented repository contracts retained in source code:

```text
internal/accounting, internal/notes, internal/files, internal/llm
internal/platform/audit, internal/platform/auth, internal/platform/ref, internal/platform/storage
```

Saturn keeps the existing runtime database stack:

```text
Go database/sql
github.com/jackc/pgx/v5/stdlib
sqlc Go generation with sql_package: "database/sql"
sqlc Go generation with omit_unused_structs: true
```

`sqlc` is a fixed development tool dependency and is run through the Go module tool mechanism.

## SQL And Authorization Constraints

Generated code does not move authorization decisions out of services or `platform/auth`. For list and search behavior, a module repo selects fixed parameterized named queries consistent with `auth.Scope`; it does not assemble user-controlled SQL fragments.

The following remain prohibited:

```text
centralized business query packages under internal/platform/db
generated module packages containing unused models owned by other modules
handlers or services calling generated query packages directly
generated query packages importing module services
SQL assembled from user-provided table names, column names, ordering, or permission expressions
empty sqlc query packages or queries for behavior with no existing repository contract
```

## Consequences

Repository SQL becomes explicit, checked during generation, and compiled through stable module-level adapters. Schema changes and SQL changes must regenerate committed output and run relevant PostgreSQL repository tests.

The generated packages add source files to the repository and expose database-oriented row types internally. Domain types and service contracts remain owned by their normal module packages.

## Reconsideration Conditions

Revisit this decision only if the runtime database access stack changes materially, sqlc cannot express required PostgreSQL queries without excessive workarounds, or query ownership rules require a new module boundary.
