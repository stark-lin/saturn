# Platform API

## 1. Ownership and Authentication

Platform owns authentication, API key lifecycle, ObjectRef metadata queries, audit-log queries, and the shared SSE transport.

Saturn has one administrator (`USR-00000001`) and multiple API keys (`KEY-*`). Browser login returns a JWT backed by a Redis session. Programmatic clients use an API key directly. Both use:

```http
Authorization: Bearer <credential>
```

Business data is shared at instance scope. API keys require `data:read` or `data:write`; `data:write` implies reads. Administrator JWTs have all capabilities. Administrator account changes, API key management, and audit queries reject API-key principals with `403`.

## 2. Endpoint Inventory

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/auth/login` | Public | Create administrator browser session |
| `GET` / `HEAD` | `/api/auth/me` | Any authenticated principal | Return current principal |
| `PATCH` | `/api/auth/me` | Administrator | Update singleton profile |
| `PATCH` | `/api/auth/me/password` | Administrator | Change singleton password |
| `GET` / `HEAD` | `/api/auth/api-keys` | Administrator | List API key metadata |
| `POST` | `/api/auth/api-keys` | Administrator | Create and reveal an API key once |
| `POST` | `/api/auth/api-keys/{refcode}/revoke` | Administrator | Revoke an API key |
| `POST` | `/api/auth/logout` | Administrator JWT | Revoke current browser session |
| `GET` / `HEAD` | `/api/events` | `data:read` | Open shared SSE stream |
| `GET` / `HEAD` | `/api/platform/object-refs/{ref_code}` | `data:read` | Resolve exact metadata |
| `POST` | `/api/platform/object-refs/search` | `data:read` | Filter ObjectRef metadata |
| `GET` / `HEAD` | `/api/platform/recent-objects` | `data:read` | Recent ObjectRef metadata |
| `GET` / `HEAD` | `/api/platform/search` | `data:read` | Compatibility exact lookup |
| `GET` / `HEAD` | `/api/platform/audit-logs` | Administrator | Query append-only audits |

Removed multi-user paths such as `/api/auth/users` and `/api/auth/users/{id}/password` return `404`.

## 3. Administrator Session

### 3.1 Login

```http
POST /api/auth/login
Content-Type: application/json
```

```json
{
  "username": "admin",
  "password": "admin"
}
```

Successful response:

```json
{
  "token": "<jwt>",
  "expires_at": "2026-07-19T11:00:00Z",
  "user": {
    "refcode": "USR-00000001",
    "kind": "administrator",
    "username": "admin"
  }
}
```

The JWT subject is `USR-00000001`; it does not contain a role or database user ID. Invalid credentials return `401/invalid_credentials` without revealing whether the username exists.

### 3.2 Current Principal

`GET /api/auth/me` returns an envelope with `user`. For an API key it contains the key RefCode, name, kind, and scopes:

```json
{
  "user": {
    "refcode": "KEY-4F8A2C10",
    "kind": "api_key",
    "name": "saturn-mcp",
    "scopes": ["data:read", "data:write"]
  }
}
```

### 3.3 Profile and Password

`PATCH /api/auth/me` accepts optional `username` and `email`; at least one may be supplied. `PATCH /api/auth/me/password` accepts:

```json
{
  "current_password": "old-secret",
  "new_password": "new-secret"
}
```

Only the human administrator may call these endpoints. API keys receive `403/forbidden`.

### 3.4 Logout

`POST /api/auth/logout` deletes the current JWT token ID from Redis. API keys have no session and are ended only by revocation or expiry.

## 4. API Keys

### 4.1 Create

```http
POST /api/auth/api-keys
Content-Type: application/json
```

```json
{
  "name": "saturn-mcp",
  "scopes": ["data:read", "data:write"],
  "expires_at": null
}
```

`expires_at` is optional RFC3339 and must be in the future. Names must match `^[a-z0-9][a-z0-9._-]{0,63}$`. Names remain reserved after revocation.

Successful creation returns the complete secret exactly once:

```json
{
  "refcode": "KEY-4F8A2C10",
  "name": "saturn-mcp",
  "api_key": "sat_sk_xxxxxxxxxxxxxxxxxxxx",
  "key_prefix": "sat_sk_xxxxxxxx",
  "scopes": ["data:read", "data:write"],
  "expires_at": null
}
```

The web client presents this response in a one-time modal with a copy action. Closing the modal clears and removes the complete secret from the current page; only the key metadata remains, and the modal cannot be reopened.

The server generates 256 random secret bits, stores only the lowercase SHA-256 digest plus display prefix, and never provides a secret-recovery endpoint.

### 4.2 List

`GET /api/auth/api-keys` returns:

```json
{
  "api_keys": [
    {
      "refcode": "KEY-4F8A2C10",
      "name": "saturn-mcp",
      "key_prefix": "sat_sk_xxxxxxxx",
      "scopes": ["data:read"],
      "status": "ACTIVE",
      "created_at": "2026-07-19T10:00:00Z",
      "last_used_at": null,
      "expires_at": null,
      "revoked_at": null
    }
  ]
}
```

`key_hash` and the complete secret are never serialized. Status is `ACTIVE`, `REVOKED`, or `EXPIRED`.

### 4.3 Revoke

`POST /api/auth/api-keys/{refcode}/revoke` is idempotent for an existing key and sets `revoked_at` once. API keys are never physically deleted or renamed.

## 5. ObjectRef Metadata

All metadata routes require `data:read` and operate on the shared instance collection. Responses contain no database IDs or `owner_id`.

Exact response:

```json
{
  "ref_code": "NTE-00000001",
  "module": "notes",
  "object_type": "nte-obj",
  "title": "Weekly Review",
  "tags": ["weekly"],
  "status": "draft",
  "created_at": "2026-07-19T10:00:00Z",
  "updated_at": "2026-07-19T10:00:00Z"
}
```

`POST /api/platform/object-refs/search` supports `modules`, `object_types`, `statuses`, all-tags matching, created/updated RFC3339 ranges, sort by `created_at`/`updated_at`/`ref_code`, and `limit` up to 100. `/api/platform/recent-objects` defaults to 10 and allows 1–50.

## 6. Audit Logs

`GET /api/platform/audit-logs` is administrator-only. Supported filters:

| Parameter | Rule |
| --- | --- |
| `target_ref_code` | Valid business, API key, administrator, or system RefCode |
| `actor_ref_code` | `USR-00000001`, `KEY-*`, or `SYS-00000000` |
| `action` | `CREATE`, `READ`, `UPDATE`, `DELETE`, `EXPORT`, `LOGIN`, `LOGOUT` |
| `result` | `SUCCESS`, `FAILED`, `DENIED` |
| `limit` | 1–100, default 50 |
| `offset` | Non-negative integer |

Response rows use `actor_ref_code`; numeric actor IDs and actor-type columns no longer exist:

```json
{
  "audit_logs": [
    {
      "id": 42,
      "actor_ref_code": "KEY-4F8A2C10",
      "action": "CREATE",
      "target_ref_code": "NTE-00000001",
      "result": "SUCCESS",
      "source_ip": "127.0.0.1",
      "created_at": "2026-07-19T10:00:00Z"
    }
  ],
  "limit": 50,
  "offset": 0
}
```

Audit rows are append-only. Names and secrets are not copied into audit history.

## 7. SSE

`GET /api/events` uses streaming `fetch` with the same Bearer header. It returns `text/event-stream`, sends heartbeat comments, and terminates when the request context is canceled. Specific event names remain owned by their source modules.
