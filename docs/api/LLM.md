# LLM API

## 1. Ownership

LLM owns the HTTP contracts for sessions, requests, authorized reference context assembly, OpenAI-style provider calls, asynchronous task dispatch, and LLM-originated read audits.

```text
Path prefix: /api/llm
Module: internal/llm
Common rules: ../API.md
```

---

## 2. Current Status

`Implemented`. The current `/api/llm` routes are registered in `internal/app/routes.go`.

## 3. Endpoint Inventory

| Method | Path | Authentication | Status | Purpose |
| --- | --- | --- | --- | --- |
| `GET` | `/api/llm/sessions` | Authenticated | `Implemented` | List LLM sessions accessible to the current actor |
| `POST` | `/api/llm/sessions` | Authenticated | `Implemented` | Create an LLM session and register the `LLM-*` session ref |
| `GET` | `/api/llm/sessions/{ref_code}` | Authenticated | `Implemented` | Read a session along with its requests in the same view |
| `DELETE` | `/api/llm/sessions/{ref_code}` | Authenticated | `Implemented` | Delete the entire session, and recursively delete its requests and ObjectRef projections |
| `POST` | `/api/llm/sessions/{ref_code}/requests` | Authenticated | `Implemented` | Create an immutable request, write a PostgreSQL queued request, and return the queued request |
| `GET` | `/api/llm/requests/{ref_code}` | Authenticated | `Implemented` | Query current status and result of a request by request ref |

---

## 4. Data and Status

```text
llm_sessions
llm_requests
llm_request_references
```

Object references:

```text
Session  -> object_type = llm_session,  ref_code = LLM-*
Request  -> object_type = llm_request,  ref_code = LLM-*
```

Both Session and Request write tags to their respective `object_refs.tags`. In business object responses, wherever `ref_code` is returned, `tags` must also be returned; when there are no tags, it returns `[]`.

Status rules:

```text
session.status = active
request input fields are immutable, modification is not allowed after submission
request does not provide a separate delete API
requests can only be deleted recursively along with session deletion
request.response_status initial value = queued
request.response_status can only transition queued -> running -> success/error
request.response_status cannot be rewritten once it is success or error
```

The `request` saves both request inputs and response results in the same row:

```text
prompt
model
max_tokens
context_json
request_json
response_status
content
error_code
error_message
response_json
completed_at
```

`context_json` is the authorized context assembled by the backend based on references; `request_json` is the final JSON sent to the OpenAI-style API. Both are only recorded in the database and are not returned as complete fields in normal HTTP responses.

---

## 5. Asynchronous Execution

```text
1. Frontend POSTs to create a request.
2. The service persists the queued request and the request ObjectRef.
3. HTTP returns 202 and the request ref.
4. A fixed number of workers claim queued requests using PostgreSQL `FOR UPDATE SKIP LOCKED`, and update the request to running.
5. The worker calls the provider, and writes the success/error result back to the same llm_requests row.
6. If a single provider call exceeds `llm.timeout_seconds`, an error is written with error_code = llm_request_timeout.
7. The frontend polls GET /api/llm/requests/{ref_code} every 1 second until success/error.
```

The current baseline does not implement retries or dead-lettering. Provider call failures or timeouts write `error`; worker infrastructure errors only record logs and do not re-enqueue.

---

## 6. Create Session

```http
POST /api/llm/sessions
Authorization: Bearer <token>
Content-Type: application/json
```

Request:

```json
{
  "title": "Monthly review",
  "tags": ["monthly", "review"]
}
```

Response `201`:

```json
{
  "session": {
    "ref_code": "LLM-00000001",
    "title": "Monthly review",
    "status": "active",
    "tags": ["monthly", "review"],
    "created_at": "2026-06-05T00:00:00Z",
    "updated_at": "2026-06-05T00:00:00Z"
  }
}
```

`Location`:

```text
/api/llm/sessions/LLM-00000001
```

---

## 7. Read Session View

```http
GET /api/llm/sessions/LLM-00000001?limit=25&offset=0
Authorization: Bearer <token>
```

Response `200`:

```json
{
  "session": {
    "ref_code": "LLM-00000001",
    "title": "Monthly review",
    "status": "active",
    "tags": ["monthly", "review"],
    "created_at": "2026-06-05T00:00:00Z",
    "updated_at": "2026-06-05T00:00:00Z"
  },
  "requests": [
    {
      "ref_code": "LLM-00000002",
      "prompt": "Summarize this account",
      "model": "gpt-4o-mini",
      "max_tokens": 1024,
      "tags": ["summary"],
      "references": [
        {
          "ref_code": "ACC-00000001",
          "module": "accounting",
          "object_type": "account",
          "title": "Cash",
          "status": "active",
          "tags": ["cash"]
        }
      ],
      "response_status": "running",
      "content": "",
      "created_at": "2026-06-05T00:00:01Z",
      "updated_at": "2026-06-05T00:00:02Z"
    }
  ]
}
```

---

## 8. Create Request

```http
POST /api/llm/sessions/LLM-00000001/requests
Authorization: Bearer <token>
Content-Type: application/json
```

Request:

```json
{
  "prompt": "Summarize this account",
  "references": ["ACC-00000001"],
  "model": "",
  "max_tokens": 0,
  "tags": ["summary"]
}
```

Rules:

```text
When model is empty, use llm.model
When max_tokens is 0, use llm.max_tokens
max_tokens cannot be greater than llm.max_tokens
references can be empty; when non-empty, must be valid ref_codes
Duplicate references will be deduplicated
tags can be empty; when non-empty, trimmed, empty values removed, and deduplicated before associating with the Request
```

Response `202` returns queued request view:

```json
{
  "request": {
    "ref_code": "LLM-00000002",
    "prompt": "Summarize this account",
    "model": "gpt-4o-mini",
    "max_tokens": 1024,
    "tags": ["summary"],
    "references": [
      {
        "ref_code": "ACC-00000001",
        "module": "accounting",
        "object_type": "account",
        "title": "Cash",
        "status": "active",
        "tags": ["cash"]
      }
    ],
    "response_status": "queued",
    "content": "",
    "created_at": "2026-06-05T00:00:01Z",
    "updated_at": "2026-06-05T00:00:01Z"
  }
}
```

`Location`:

```text
/api/llm/requests/LLM-00000002
```

---

## 9. Read Request

```http
GET /api/llm/requests/LLM-00000002
Authorization: Bearer <token>
```

Response `200`:

```json
{
  "request": {
    "ref_code": "LLM-00000002",
    "prompt": "Summarize this account",
    "model": "gpt-4o-mini",
    "max_tokens": 1024,
    "references": [],
    "tags": ["summary"],
    "response_status": "success",
    "content": "The account balance is ...",
    "created_at": "2026-06-05T00:00:01Z",
    "updated_at": "2026-06-05T00:00:02Z",
    "completed_at": "2026-06-05T00:00:02Z"
  }
}
```

If the provider is not configured or the call fails, the worker writes `error`:

```json
{
  "request": {
    "ref_code": "LLM-00000002",
    "prompt": "Summarize this account",
    "model": "gpt-4o-mini",
    "max_tokens": 1024,
    "references": [],
    "tags": ["summary"],
    "response_status": "error",
    "content": "",
    "error_code": "llm_request_failed",
    "error_message": "provider unavailable",
    "created_at": "2026-06-05T00:00:01Z",
    "updated_at": "2026-06-05T00:00:02Z",
    "completed_at": "2026-06-05T00:00:02Z"
  }
}
```

---

## 10. Delete Session

```http
DELETE /api/llm/sessions/LLM-00000001
Authorization: Bearer <token>
```

Response `204` with no body.

Deletion rules:

```text
Only the entire session can be deleted
Deleting a session will recursively delete all llm_requests and llm_request_references under that session
Delete request and session ObjectRef projections
ObjectRef deletion will delete the tags projection on the same row
Write a DELETE audit for each request, reason = cascade_llm_session
Write a DELETE audit for the session
Terminal requests (success/error) can also only be deleted recursively through the session
```

---

## 11. Permissions, References, and Auditing

```text
The handler only performs authentication, parameter binding, and response writing
The service executes session permissions, reference permissions, auditing, and provider call orchestration
The repo only executes fixed SQL
As an upper-layer module, LLM can depend on business module services / facades
LLM does not directly access other business module repos or business tables
```

Reference resolution flow:

```text
1. The backend parses the ObjectRef by ref_code to identify the module/object_type.
2. The backend calls the read method of the corresponding business module service and passes in the original actor.
3. The business module service executes its existing resource-level permission rules.
4. LLM saves a snapshot of the reference to llm_request_references.
5. LLM stitches the authorized payload into context_json.
6. LLM writes an audit READ for each successfully read reference, with actor_type = LLM.
```

If a single resource does not exist or access is denied, it returns:

```text
HTTP 404
code = not_found
```

The LLM provider API key is a secret, is not returned to the client, and is not written to request/response JSON, audits, logs, or normal API responses.

---

## 12. Not in Current Scope

```text
No expression evaluation
No formal business write operations executed
No streaming endpoint
No retry queue
No dead-letter queue
```
