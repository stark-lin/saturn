# Accounting API

## 1. Ownership

Accounting owns the HTTP contracts for manual ledgers and immutable transactions.

```text
Path prefix: /api/accounting
Module: internal/accounting
Common rules: ../API.md
```

## 2. Current Status

`Implemented`. The current Accounting API implements the Account and Transaction main closed loop; tag projections reuse `ObjectRef`.

## 3. Endpoint Inventory

| Method | Path | Authentication | Status | Purpose |
| --- | --- | --- | --- | --- |
| `GET` | `/api/accounting/accounts` | Authenticated | `Implemented` | List readable ledgers |
| `POST` | `/api/accounting/accounts` | Authenticated | `Implemented` | Create a ledger |
| `GET` | `/api/accounting/accounts/{ref_code}` | Authenticated | `Implemented` | Read a ledger |
| `DELETE` | `/api/accounting/accounts/{ref_code}` | Authenticated | `Implemented` | Delete a ledger and all its transactions |
| `GET` | `/api/accounting/transactions` | Authenticated | `Implemented` | List transactions |
| `POST` | `/api/accounting/transactions` | Authenticated | `Implemented` | Create a posted transaction |
| `GET` | `/api/accounting/transactions/{ref_code}` | Authenticated | `Implemented` | Read a transaction |
| `POST` | `/api/accounting/transactions/{ref_code}/void` | Authenticated | `Implemented` | Void a posted transaction |

## 4. Models and Reference Codes

```text
Account     is a ledger; registered in Platform/ObjectRef with object_type=account, ref_code=ACC-*.
Transaction is an entry directly belonging to an Account; registered with object_type=transaction, ref_code=ACC-*.
Title       Account name and Transaction display title project to corresponding object_refs.title.
Tags        Written to the corresponding object_refs.tags for the Account or Transaction.
Note        Can contain FIL-* or NTE-* text references, but Accounting does not create object links.
```

Amounts are uniformly `BIGINT` cents:

```text
12.50 AUD  -> 1250
-8.99 AUD  -> -899
```

Model enumerations and defaults:

```text
Account.type:       default | cash | bank | credit_card | digital_wallet | stored_value | other
Account.currency:   Three-letter uppercase code; defaults to AUD if not provided at creation
Transaction.kind:   income | expense | adjustment
Transaction.status: posted | voided; owned by the server, clients must not submit
```

`Account.type` is retained as a compatibility field and defaults to `default` if not submitted; the current frontend uses `tags` to classify ledgers and does not display a type control. `Account.status=active` is only stored in the ObjectRef projection; current Account HTTP responses do not output this field. Platform metadata queries return the Account's `title=name` and associated tags. Transaction metadata queries return the corresponding display title and associated tags; for an empty `title`, the display projection is generated as `<Kind> YYYY-MM-DD`.

## 5. Request Contract

The request body for creation endpoints uses JSON; undefined fields will be rejected with `HTTP 400 / invalid_request`. `POST /transactions/{ref_code}/void` does not require a request body, and the current implementation does not read the body.

Create an Account:

```json
{
  "name": "Daily account",
  "currency": "AUD",
  "opening_balance_cents": 5000,
  "tags": ["bank", "daily"]
}
```

Field rules:

| Field | Required | Rule |
| --- | --- | --- |
| `name` | Yes | Must not be empty after trimming |
| `type` | No | Enumerated value; `default` if not provided |
| `currency` | No | Uppercase after trimming, must be three `A-Z` characters; `AUD` if not provided |
| `opening_balance_cents` | No | Integer cents; `0` if not provided, allows negative values |
| `tags` | No | Trimmed, empty values removed, and deduplicated before associating with the Account; defaults to an empty array |

Create a Transaction:

```json
{
  "account_ref_code": "ACC-00000001",
  "occurred_on": "2026-05-26",
  "kind": "expense",
  "amount_cents": -1250,
  "title": "Lunch",
  "note": "Receipt FIL-00000008",
  "tags": ["food", "work"]
}
```

Field rules:

| Field | Required | Rule |
| --- | --- | --- |
| `account_ref_code` | Yes | `ACC-*` reference code of the Account; input is normalized to uppercase |
| `occurred_on` | Yes | `YYYY-MM-DD` date |
| `kind` | Yes | `income`, `expense`, or `adjustment` |
| `amount_cents` | Yes | Non-zero integer cents; `income > 0`, `expense < 0`, `adjustment` can be positive or negative |
| `title` | No | Saved after trimming; defaults to an empty string |
| `note` | No | Saved as-is as text; defaults to an empty string |
| `tags` | No | Trimmed, empty values removed, and deduplicated before associating with the Transaction; defaults to an empty array |

List queries:

```text
accounts:     limit, offset
transactions: account_ref_code, status, tag, from, to, limit, offset
```

| Parameter | Rule |
| --- | --- |
| `account_ref_code` | Optional Account `ACC-*` reference code filter |
| `status` | Optional; `posted` or `voided` |
| `tag` | Optional; exact filter by associated tag name after trimming |
| `from`, `to` | Optional `YYYY-MM-DD`; both boundaries are inclusive, and `from <= to` |
| `limit` | Defaults to `25`, range `1..100` |
| `offset` | Defaults to `0`, must be a non-negative integer |

List endpoints reject undefined query parameters. Accounts are sorted by `updated_at DESC, ref_code DESC`; Transactions are sorted by `occurred_on DESC, internal id DESC`.

## 6. Response Contract

Creating an Account returns `HTTP 201`, and sets:

```http
Location: /api/accounting/accounts/ACC-00000001
```

Creation and read responses for a single Account:

```json
{
  "account": {
    "ref_code": "ACC-00000001",
    "name": "Daily account",
    "type": "default",
    "currency": "AUD",
    "opening_balance_cents": 5000,
    "balance_cents": 3750,
    "tags": ["bank", "daily"],
    "created_at": "2026-05-26T00:00:00Z",
    "updated_at": "2026-05-26T00:00:00Z"
  }
}
```

Account list response:

```json
{
  "accounts": [],
  "pagination": {
    "limit": 25,
    "offset": 0,
    "has_more": false
  }
}
```

Deleting an Account successfully returns `HTTP 204` without a JSON body.

Creating a Transaction returns `HTTP 201`, and sets:

```http
Location: /api/accounting/transactions/ACC-00000002
```

Creation, read, and void responses for a single Transaction:

```json
{
  "transaction": {
    "ref_code": "ACC-00000002",
    "account_ref_code": "ACC-00000001",
    "occurred_on": "2026-05-26",
    "kind": "expense",
    "amount_cents": -1250,
    "title": "Lunch",
    "note": "Receipt FIL-00000008",
    "status": "posted",
    "tags": ["food", "work"],
    "created_at": "2026-05-26T00:00:00Z",
    "updated_at": "2026-05-26T00:00:00Z"
  }
}
```

Transaction list response:

```json
{
  "transactions": [],
  "pagination": {
    "limit": 25,
    "offset": 0,
    "has_more": false
  }
}
```

## 7. Invariants and Transactions

```text
Once created, a Transaction cannot modify its account, date, kind, amount, title, note, or tags.
Account tags are written at creation; the current Account contract provides no endpoint for subsequent updates.
Transactions do not provide a separate deletion endpoint; the only status transition is posted -> voided.
Deleting an Account is a ledger-level deletion that physically cascades the deletion of its Transactions and their corresponding references and tag associations.
Before deleting an Account, its Transactions must be enumerated, and a DELETE SUCCESS audit with the reason `cascade_account_delete` must be written for each cascade-deleted Transaction.
balance_cents = opening_balance_cents + SUM(posted transactions.amount_cents).
The Account row is locked before creating a transaction, voiding a transaction, or deleting a ledger.
Business writes for creating an Account, creating a transaction, voiding a transaction, or deleting an Account, updates to the `object_refs.title/tags/status` projection, balance recalculations (where applicable), the parent Account DELETE audit, and the cascaded Transaction DELETE audits must all be committed in the same transaction.
If a write operation fails or is rejected within the transaction, no SUCCESS audit is retained; the failed/denied write attempt that was initiated records a FAILED or DENIED audit after the outcome is finalized.
```

## 8. Permissions and Errors

All endpoints require a Bearer JWT. Resource authorization is executed in the Accounting service, and the repo only applies fixed scope queries:

```text
user:      Can only list, read, and write Accounts / Transactions they own.
superuser: Can list, read, create transactions for, void, or delete existing Accounts / Transactions of any owner;
           can also create Accounts, but newly created Accounts always belong to the actor creating them.
```

Inaccessible resources and non-existent resources both manifest externally as:

```http
HTTP 404
{"error":{"code":"not_found","message":"Accounting resource not found"}}
```

Error responses follow the common envelope:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "..."
  }
}
```

The specific `message` will return corresponding Accounting error text depending on the parameter binding or service validation phase; clients handle branching based on `status` and `code`.

| Status | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Invalid creation request JSON/unknown fields, query, `ref_code`, amount sign, date, or enumerated values |
| `401` | `unauthorized` | Unauthenticated or missing authenticated Principal |
| `404` | `not_found` | Account / Transaction does not exist, or the current actor has no access rights |
| `409` | `conflict` | Voiding a Transaction that is already `voided` |
| `500` | `accounting_unavailable` | Accounting dependencies or internal operations failed |
