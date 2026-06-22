# FILES app Assembly Layer

> Split from `docs/FILES.md`: Original Chapter 5.

## 5. `internal/app`

```text
internal/app
├── app.go
├── deps.go
├── routes.go
├── middleware.go
└── lifecycle.go
```

### 5.1 File Responsibilities

| File | Responsibility |
| --- | --- |
| `app.go` | Defines the `App` struct, HTTP router holder, and startup entry |
| `deps.go` | Initializes config, DB, Redis, local FS storage, platform services, and business module dependencies |
| `routes.go` | Centrally registers APIs, frontend resource entries, platform capabilities, SSE, and health check routes |
| `middleware.go` | Assembles common middleware such as authentication, logging, recover, request id, etc. |
| `lifecycle.go` | Manages server start/stop, LLM worker / scheduler start/stop, and graceful shutdown |

### 5.2 Rules

Allowed:

```text
Depend on all business modules
Depend on internal/platform
Responsible for dependency injection
Responsible for route registration
```

Prohibited:

```text
Implementing business rules
Writing SQL directly
Manipulating Redis keys directly
Handling local FS storage details directly
```

---
