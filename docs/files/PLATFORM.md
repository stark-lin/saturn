# FILES platform Support Layer

> Split from `docs/FILES.md`: `internal/platform` lower-level support package standards.

## 6. `internal/platform`

`internal/platform` is a collection of lower-level support packages for the business service layer, including infrastructure adaptation, cross-cutting capabilities, and cross-business general application capabilities.

Core definitions:

```text
Business services can depend on platform
platform must not depend on any business service, repo, handler, or model
platform does not own specific business module rules
platform does not reverse-customize logic for a specific business module
```

Allowed:

```text
Being depended on by business services
Being used by handlers / middleware for common capabilities like authentication, response, logging, rate limiting
Depending on external infrastructure, standard libraries, and third-party libraries
Depending on other packages within the platform
```

Prohibited:

```text
Depending on internal/app
Depending on vertical business modules like internal/accounting, internal/notes, internal/files, internal/calendar, internal/llm
Containing specific business rules of accounting/notes/files/calendar/llm
Generating or concatenating business module SQL
Directly understanding business module table structures
```

Target directory:

```text
internal/platform
├── audit
├── auth
├── config
├── db
├── httpx
├── logger
├── redis
├── ref
├── search
└── storage
```

`auth`, `storage`, `search`, and `ref` must not directly import vertical business modules or access vertical business repos.

Redis is a required infrastructure but is only allowed to be used by `platform/auth`'s session store. The LLM asynchronous request queue is handled by `llm_requests` and PostgreSQL `FOR UPDATE SKIP LOCKED`.

---

### 6.1 `platform/audit`

```text
internal/platform/audit
├── model.go
├── repo.go
├── queries
│   └── events.sql
├── sqlc
│   └── generated Go query code
└── service.go
```

Responsibilities:

```text
append-only audit event recording
superuser-only audit event querying
critical operation audit format conventions
```

Rules:

```text
Business modules record audit events via the audit service
Business modules must not write directly to the audit table
audit does not own business module rules
audit_logs only supports INSERT and SELECT, not UPDATE, DELETE, or TRUNCATE
Ordinary resource reads are not audited; only LLM-initiated reads record READ
Successful business changes and SUCCESS audits are in the same PostgreSQL transaction
After failure or denial conclusions are determined, insert FAILED or DENIED in a new audit-only transaction
```

---

### 6.2 `platform/auth`

```text
internal/platform/auth
├── queries
│   └── users.sql
├── sqlc
│   └── generated Go query code
├── principal.go
├── context.go
├── user.go
├── credential.go
├── token.go
├── session.go
├── middleware.go
├── handler.go
├── action.go
├── resource.go
├── scope.go
├── authorizer.go
├── repo.go
└── service.go
```

Responsibilities:

```text
User model
Login and logout
Password hashing and verification
JWT issuance and verification
session creation, reading, and revocation
Current Principal injection and reading
Route-level authentication middleware
Role, action, resource facts, and authorization judgments
List / search scope generation
```

Rules:

```text
auth can depend on db, clock, config, and Redis-backed session capabilities
auth does not depend on any business modules
auth does not generate business SQL
auth does not know the table structures of accounting, notes, files, calendar
auth's own user authentication persistence queries can be implemented via this package's queries/ and sqlc/
auth's generation config must not output models unused by other business modules
Resource-level authorization is completed by business services calling the auth authorizer
List / search authorization is generated as scopes by auth, and business repos are responsible for implementing it into this module's SQL
```

Authorization division:

```text
middleware:
  Responsible for route-level entry control like RequireLogin, RequireRole

handler:
  Responsible for parameter binding, reading Principal, calling service
  Can use route-level guards
  Does not implement resource-level permission checks

service:
  Must receive actor / Principal
  Responsible for action-level and resource-level authorization
  Responsible for critical write operation audits

repo:
  Only responsible for data access
  List / search queries can receive auth.Scope
  Does not redefine permission policies
```

`auth.Scope` only expresses authorization semantics, not SQL:

```go
type Scope struct {
    All           bool
    OwnerID       int64
    IncludeShared bool
}
```

### 6.4 `platform/config`

```text
internal/platform/config
├── config.go
└── load.go
```

Responsibilities:

```text
Loading configuration
Defining configuration structure
Reading JSON configuration files
Generating config.json when local config is missing
Providing required infrastructure configuration
```

Rules:

```text
config.json is the default runtime configuration file
config.example.json is the template committed to the repository
SATURN_* environment variables are only used when generating config.json for the first time
Fails fast when parsing an existing config file fails
Redis configuration is a required item
PostgreSQL configuration is a required item
Local FS storage configuration is a required item
Does not provide redis_enabled=false configuration
```

Configuration validation stays local to `platform/config` until a real shared validation package is needed.

---

### 6.5 `platform/db`

```text
internal/platform/db
├── db.go
├── migrate.go
└── tx.go
```

Responsibilities:

```text
Database connection
Migration execution
Transaction encapsulation
```

Prohibited:

```text
Storing business SQL
Storing business repositories
Storing centralized sqlc business query generation packages
```

---

### 6.6 `platform/httpx`

```text
internal/platform/httpx
├── bind.go
├── error.go
├── middleware.go
├── response.go
└── sse.go
```

Responsibilities:

```text
Request binding
Unified error responses
Unified success responses
HTTP middleware basic encapsulation
SSE response encapsulation
```

Rules:

```text
httpx does not depend on business modules
httpx does not contain business error text
SSE transient state does not use Redis; if persistence is needed later, it is defined by the event-producing module's own PostgreSQL state table
```

### 6.8 `platform/logger`

```text
internal/platform/logger
└── logger.go
```

Responsibilities:

```text
structured logging encapsulation
request / storage / LLM audit related log field conventions
```

---

### 6.9 `platform/ref`

```text
internal/platform/ref
├── code.go
├── model.go
├── repo.go
├── queries
│   └── object_refs.sql
├── sqlc
│   └── generated Go query code
├── resolver.go
└── service.go
```

Responsibilities:

```text
Generate unified readable ref codes
Register references for important business objects
Resolve ref codes to object types and internal ids
Validate ref code formats
Maintain title/status metadata projections and provide owner-only metadata querying
Support search results, cross-module associations, and LLM references
```

Rules:

```text
ref codes do not replace database ids
Business table relations still use internal ids
platform/ref does not own business module rules
platform/ref does not directly read or modify business module repos
Reading real objects still goes through the corresponding module's service / facade
Permission judgments are still executed by business services and platform/auth
object_refs is the authoritative source for ref_code and title/tags/status metadata projections; source business tables do not duplicate saving ref_code
Module prefixes are fixed to NTE / FIL / ACC / CAL / LLM; objects of the same module are differentiated by object_type
Code suffixes are generated as 8-character uppercase Hex by a global PostgreSQL sequence; numbers are not reused
GET /api/platform/search?ref_code=<code> only returns metadata JSON containing title/tags/status to the owner
```

For detailed concepts, see [../OBJECT_REF_CODE.md](../OBJECT_REF_CODE.md).

---

### 6.11 ObjectRef Tags

```text
internal/platform/ref
└── object_refs.tags TEXT[]
```

Responsibilities:

```text
Maintain object tag projections based on object_refs
Provide business modules with the ability to register or update current object tag sets
Provide tags sets for owner-only ObjectRef metadata responses
```

Rules:

```text
platform/ref does not own derived tag rules for Notes, Files, or other modules
Business modules are responsible for deriving tag names from their inputs, and writing them to tags upon ObjectRef registration or projection updates
tags are only saved in object_refs, not directly written to business tables
Metadata objects without tags return an empty tags array
```

### 6.13 `platform/search`

```text
internal/platform/search
└── handler.go
```

Responsibilities:

```text
Owner-only ObjectRef metadata endpoint
ObjectRef JSON search endpoint
Recent objects endpoint
Compatibility ref_code metadata endpoint
```

Rules:

```text
Does not own source business records
Does not modify source business records
Delegates metadata reads to platform/ref
```

---

### 6.14 `platform/storage`

```text
internal/platform/storage
├── client.go
├── object.go
├── repo.go
├── queries
│   └── objects.sql
├── sqlc
│   └── generated Go query code
├── service.go
└── usage.go
```

Responsibilities:

```text
Local FS storage encapsulation
Object read/write
Object staging and promotion
Object deletion
Object metadata
Object references
Storage usage and diagnostics
```

Rules:

```text
Default file blob storage is local FS
Business modules do not write directly to the local FS
Business modules access storage capabilities via platform/storage by default
platform/storage does not own Files business rules
```
