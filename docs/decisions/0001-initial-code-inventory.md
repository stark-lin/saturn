# Decision: Initial Code Inventory Scaffold

## Status

Accepted for the initial repository bootstrap.

## Context

The product docs target a real MVP and normally prohibit empty packages, empty routes, and empty migrations. The repository, however, started with documentation only, and the initial implementation request was to create the documented code-file inventory with a first-line English responsibility comment in every created source/config/script/migration artifact.

## Decision

The first code pass is an explicit scaffold/inventory phase. It creates compilable Go packages, migration skeletons with real table definitions, Docker/dev files, scripts, and a minimal static web client, but it does not register unfinished feature routes or claim complete MVP business behavior.

The initial stack is:

```text
Go module: github.com/stark-lin/saturn
Backend: Go net/http and standard library first
Frontend: plain native HTML first, with minimal vanilla JavaScript only when required
Build: no React, Vite, Node, or frontend package manager in this pass
```

## Consequences

Future feature work should replace scaffold boundaries with real service, repository, handler, test, and migration behavior module by module. The long-term rule still stands: do not add new empty future packages after this bootstrap inventory.
