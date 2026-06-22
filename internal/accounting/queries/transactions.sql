-- This file defines typed minimal Accounting ledger query templates for sqlc generation.

-- name: ListAccountsAll :many
SELECT a.id, a.owner_id, object_ref.ref_code, a.name, a.type, a.currency,
       a.opening_balance_cents, a.balance_cents, a.created_at, a.updated_at
FROM accounts AS a
JOIN object_refs AS object_ref
  ON object_ref.owner_id = a.owner_id
 AND object_ref.object_type = 'account'
 AND object_ref.object_id = a.id
ORDER BY a.updated_at DESC, object_ref.ref_code DESC
LIMIT $1 OFFSET $2;

-- name: ListTransactionsAll :many
SELECT t.id, t.owner_id, transaction_ref.ref_code, account_ref.ref_code AS account_ref_code,
       t.occurred_on, t.kind, t.amount_cents, t.title, t.note, t.status, t.created_at, t.updated_at
FROM transactions AS t
JOIN object_refs AS transaction_ref
  ON transaction_ref.owner_id = t.owner_id
 AND transaction_ref.object_type = 'transaction'
 AND transaction_ref.object_id = t.id
JOIN object_refs AS account_ref
  ON account_ref.owner_id = t.owner_id
 AND account_ref.object_type = 'account'
 AND account_ref.object_id = t.account_id
ORDER BY t.occurred_on DESC, t.id DESC
LIMIT $1 OFFSET $2;
