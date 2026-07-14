# Decision: Use Gin at the HTTP Routing Boundary

## Status

Accepted for HTTP route registration and middleware composition.

## Context

Saturn initially used Go `http.ServeMux` and repeated authentication wrapping for every protected route. As the implemented API grew across Platform, Accounting, Calendar, Files, LLM, and Notes, route prefixes and shared route middleware became difficult to scan while the business handlers themselves remained valid standard `net/http` handlers.

The router change must not move business behavior into `internal/app`, force Gin into business modules, or replace the existing structured logging, request ID, recovery, authentication, static file, and SSE contracts.

## Decision

Saturn uses `github.com/gin-gonic/gin` for HTTP route matching, module path groups, and route-level middleware composition in `internal/app`.

Rules:

```text
Gin remains an internal/app dependency
business and platform handlers retain net/http contracts
Gin path parameters are copied to http.Request.PathValue before handler execution
all authenticated API groups reuse platform/auth bearer authentication
GET routes also register HEAD to retain net/http ServeMux behavior
Gin default logging and recovery middleware are not installed
the existing app middleware chain remains authoritative for logging, request IDs, request source capture, and panic recovery
the existing web and SSE handlers remain authoritative
```

## Consequences

Route registration is grouped by API ownership and authentication is composed once for the protected API tree. Existing handlers and their tests remain reusable, while router-level tests cover parameter adaptation, HEAD registration, authentication boundaries, unknown routes, and static web fallback.

Gin and its transitive modules become runtime dependencies. The router boundary must keep adapting `net/http` handlers until a separate decision explicitly changes handler contracts.

## Reconsideration Conditions

Revisit this decision if Gin can no longer preserve required `net/http` or SSE behavior, introduces unacceptable operational or dependency costs, or the application adopts a different HTTP transport architecture.
