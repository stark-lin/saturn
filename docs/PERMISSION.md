# Saturn Authentication and Permission Model

## 1. Instance Model

Saturn is a single-instance system with exactly one human administrator and any number of named API keys.

```text
administrator  USR-00000001
API key        KEY-xxxxxxxx
system         SYS-00000000
```

There is no registration, invitation, user list, user role, resource sharing, or user-to-user data isolation. All business data belongs to the Saturn instance. Existing `owner_id` columns are internal relational anchors to the singleton administrator row; they are not ownership or authorization boundaries and are never compared with a programmatic caller identity.

## 2. Administrator Authentication

The administrator signs in to the Web UI with the unique username and password. The development bootstrap creates `admin/admin` only when the singleton row does not exist. Production deployments must change this password and the JWT secret.

Successful login returns a short-lived JWT. The JWT subject is the fixed administrator RefCode, not a database ID or role. Redis stores the active browser-session token ID; logout removes it. Administrator responses expose:

```json
{
  "refcode": "USR-00000001",
  "kind": "administrator",
  "username": "admin"
}
```

The administrator may update only this account's username, optional email, and password. No endpoint can create another human account.

## 3. API Key Authentication

Programmatic clients use:

```http
Authorization: Bearer sat_sk_<secret>
```

The server generates the secret and returns it exactly once. PostgreSQL stores only its SHA-256 digest and a non-secret display prefix. A key is rejected when it is unknown, revoked, or expired. Successful authentication updates `last_used_at` and creates a principal whose data anchor is the singleton administrator ID but whose actor identity is the key's own `KEY-*` RefCode.

Supported scopes:

| Scope | Capability |
| --- | --- |
| `data:read` | Read instance business data, ObjectRef metadata, search, downloads, and SSE |
| `data:write` | Create, update, void, finish, or delete instance business data; also implies `data:read` |

API keys cannot modify the administrator account, manage API keys, query audit logs, or log out browser sessions. Those are administrator-only operations.

API key names use `^[a-z0-9][a-z0-9._-]{0,63}$`, are unique for the lifetime of the instance, and cannot be changed or reused after revocation. Keys are never physically deleted. PostgreSQL triggers enforce immutable key identity, irreversible revocation, monotonic last-use timestamps, and the no-delete rule even for direct SQL writes.

## 4. Business Resource Access

Every authenticated principal with the required scope operates on shared instance data. Service authorization checks action scope, not `owner_id`, role, sharing rows, or status. Repositories still accept fixed `auth.Scope` values so authorization does not generate SQL; production business reads receive `Scope{All: true}`.

Resource state remains a business rule. For example, `voided`, `finished`, and immutable objects retain their module-specific transition restrictions, but state never grants additional access.

## 5. Audit Identity

Audit rows and business source fields store a stable actor RefCode only:

```text
USR-00000001  administrator action
KEY-4F8A2C10   API key action
SYS-00000000   system or unidentified authentication action
```

`audit_logs.actor_ref_code` replaces actor type plus numeric user ID. `llm_requests.actor_ref_code` records the principal that submitted the request. Names are resolved for display from their domain objects and are not copied into audit history.

Audit logs are append-only and administrator-readable. Failed login attempts use `SYS-00000000` because no caller identity has been authenticated. API key secrets, bearer headers, passwords, and raw content must never enter logs or audit reasons.

## 6. Ops UI Boundary

The settings UI is an administrator aggregation layer. It reuses the normal Auth and Audit services to:

```text
inspect the singleton administrator
change its password
create/list/revoke API keys
query append-only audit logs
end the current browser session
```

It does not own privileged business paths or bypass module services.
