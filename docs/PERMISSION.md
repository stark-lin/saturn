# PERMISSION.md

## 1. Goal

This document records Saturn's permission model. For detailed execution conventions, refer to `docs/FILES.md`.

---

## 2. Roles

```text
superuser
user
```

The `superuser` is the instance owner. A `user` is an ordinary user and only possesses a proper subset of the capabilities of a superuser.

The system does not implement tenants, workspaces, or organizations.

The authentication entry point uses a unique `username` and a password; `email` can be null and is not involved in login. An `admin/admin` `superuser` account is created when the development environment is started.

Account management rules:

```text
superuser can create user accounts
the public account creation API does not provide superuser creation
user can update only their own username and email
superuser can update only their own username and email through the current-user profile endpoint
user can change their own password after verifying the current password
superuser can reset their own password
superuser can reset user role account passwords
superuser cannot reset another superuser's password if legacy or manually modified data contains more than one superuser
user cannot create accounts, change roles, or reset another account's password
```

---

## 3. Object Status And Access

```text
object_refs.status stores the current status projection for registered objects
object_refs.title, object_refs.tags and object_refs.status are metadata projections associated with the registered ref_code
the owning business module defines status values and transition rules
title, tags and status are metadata and do not grant access
```

Default scopes:

```text
superuser: all
owner: owner_id = actor_id
shared: exists explicit share row where the owning module defines sharing
```

Notes owner-only exception:

```text
The current Notes API only allows the actor to create their own Notes, and read, modify, or delete Notes they own.
The Notes API does not apply shared scopes and does not allow superusers to access Notes of other owners via the Notes API.
The current Notes API returns `status = "draft"`, but does not provide capabilities for status modification, sharing, or version reading.
When a resource does not exist or does not belong to the current owner, it uniformly returns HTTP 404 / code "not_found".
```

---

## 4. Execution Locations

```text
middleware: authentication and route-level guard
handler: bind input, read Principal, call service, write response
service: business rules, resource-level authorization, audit
repo: data access only
```

`platform/auth` returns authorization decisions or scopes; it does not generate business SQL.

Module repos translate scopes into module-specific fixed, parameterized SQL.

External response rules:

```text
resource not found -> HTTP 404 / code "not_found"
resource access denied -> HTTP 404 / code "not_found"
```

Resource-level access denial does not return 403 or `access_denied` externally, avoiding the leakage of resource existence. Internal services can retain explicit authorization decisions, which handlers fold into "not found" when writing responses according to API contracts.

---

## 5. Ops UI

The Ops UI is an aggregation entry point for superusers; it does not possess independent privileged business paths.

When operating on business resources, the Ops UI must:

```text
create or read superuser Principal
call the same exported service / facade as normal flows
reuse service-level authorization and audit
avoid direct repo / Redis / DB driver access
```

---

## 7. Platform Search

Platform search must apply auth scope before returning results.

Search contributor / facade must not return resources that the current actor cannot read.

Owner-only reference metadata responses may return ref_code, title, tags and status only after the object scope has been authorized.

---

## 8. Audit Logs

```text
only superuser can list or filter all audit_logs rows
ordinary users have no audit log query endpoint access
the audit query itself does not create a READ audit row
only LLM-originated resource reads use audit action READ
audit_logs is append-only: application behavior may INSERT or SELECT, never UPDATE, DELETE or TRUNCATE
```

Audit write transaction rules:

```text
successful business mutations insert SUCCESS audit rows inside the same PostgreSQL transaction
if that transaction fails or rolls back, no SUCCESS row is retained
after a failed or denied outcome is known, FAILED or DENIED is inserted in a new audit-only PostgreSQL transaction
LOGIN and LOGOUT target SYS-00000000 because they are system-level operations
```
