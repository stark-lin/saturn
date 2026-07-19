# Decision: Separate PostgreSQL Bootstrap, Ownership, and Runtime Roles

## Status

Accepted. This decision supersedes the application schema-bootstrap responsibility described in decision 0002; its sqlc query-ownership rules remain unchanged.

## Context

Saturn previously connected as the database owner and executed all migrations from the Go startup path. The same runtime identity could therefore create or drop schema objects, alter tables, and disable triggers. A trigger can only serve as a database enforcement boundary when the application role is neither superuser nor table owner and cannot assume an owning role.

The default deployment already creates PostgreSQL through Docker Compose, and the official PostgreSQL image has a first-run initialization boundary for an empty data directory.

## Decision

The PostgreSQL container, not the Saturn application process, owns first-run schema initialization.

```text
saturn_bootstrap  image initialization superuser only
saturn_owner      NOLOGIN database/schema/object owner
saturn            LOGIN application runtime role
```

`docker/postgres/init.sh` runs once for an empty data volume. In one transaction it creates the owner and runtime roles, assigns database/schema ownership, switches to `saturn_owner`, applies every numbered migration in lexical order, resets to the bootstrap identity, and applies `docker/postgres/runtime_grants.sql`.

The runtime grant file is derived from current repository SQL. It uses table-level `SELECT`, column-level `INSERT` and `UPDATE`, explicit root-object `DELETE`, and explicit sequence `USAGE`. Audit is `SELECT` plus column-level `INSERT`. Tables without assembled runtime behavior receive no privileges. `PUBLIC` privileges on Saturn schema objects are revoked.

The Go binary has no migration or drop-schema startup path and the application image does not contain migration files. Existing PostgreSQL volumes are never implicitly upgraded or rebuilt.

## Consequences

The application role cannot create schema objects, assume `saturn_owner`, disable triggers, set `session_replication_role`, bypass row security, mutate/delete/truncate audit rows, or access tables that are not part of current runtime behavior. Trigger functions and all schema objects remain owned by a non-login role.

New migrations affect newly initialized volumes. In-place upgrades of an existing valuable database require a separately reviewed privileged migration operation. Adding repository SQL may require a corresponding grant change; CI runs `scripts/test-postgres-bootstrap.sh` to verify the complete first-run schema and security boundary against PostgreSQL 17.

## Reconsideration Conditions

Revisit this decision when Saturn introduces a formal versioned migration runner, production upgrade/rollback orchestration, multiple database schemas, row-level security policies, or a secret-management system for database bootstrap credentials. Any replacement must preserve a distinct non-owner runtime identity and executable permission regression tests.
