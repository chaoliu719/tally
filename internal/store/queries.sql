-- name: CreateAccount :one
INSERT INTO accounts (name, type, currency, comment, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListAccounts :many
SELECT
    a.id, a.name, a.type, a.currency, a.comment, a.created_at, a.updated_at,
    CAST(COALESCE(SUM(t.amount), 0) AS INTEGER) AS balance
FROM accounts a
LEFT JOIN transactions t ON t.account_id = a.id
GROUP BY a.id
ORDER BY a.id;

-- name: GetAccount :one
SELECT id, name, type, currency, comment, created_at, updated_at
FROM accounts
WHERE id = ?;

-- name: GetAccountBalance :one
SELECT CAST(COALESCE(SUM(amount), 0) AS INTEGER) AS balance
FROM transactions
WHERE account_id = ?;

-- name: UpdateAccount :one
UPDATE accounts
SET name = ?, type = ?, comment = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteAccount :exec
DELETE FROM accounts
WHERE id = ?;

-- name: CountTransactionsByAccount :one
SELECT COUNT(*)
FROM transactions
WHERE account_id = ?;

-- name: CreateCategory :one
INSERT INTO categories (name, parent_id, created_at, updated_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: ListCategories :many
SELECT id, name, parent_id, created_at, updated_at
FROM categories
ORDER BY id;

-- name: GetCategory :one
SELECT id, name, parent_id, created_at, updated_at
FROM categories
WHERE id = ?;

-- name: UpdateCategory :one
UPDATE categories
SET name = ?, parent_id = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM categories
WHERE id = ?;

-- name: CountChildCategories :one
SELECT COUNT(*)
FROM categories
WHERE parent_id = ?;

-- name: CountTransactionsByCategory :one
SELECT COUNT(*)
FROM transactions
WHERE category_id = ?;

-- name: ListCategoryDescendantIDs :many
WITH RECURSIVE descendants(id) AS (
    SELECT c.id FROM categories c WHERE c.parent_id = sqlc.arg(target_id)
    UNION ALL
    SELECT c.id FROM categories c
    JOIN descendants d ON c.parent_id = d.id
)
SELECT id FROM descendants;

-- name: CreateTransaction :one
INSERT INTO transactions (type, account_id, category_id, amount, time, comment, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTransaction :one
SELECT id, type, account_id, category_id, amount, time, comment, created_at, updated_at
FROM transactions
WHERE id = ?;

-- name: SearchTransactions :many
SELECT id, type, account_id, category_id, amount, time, comment, created_at, updated_at
FROM transactions
WHERE (sqlc.narg('account_id')  IS NULL OR account_id = sqlc.narg('account_id'))
  AND (sqlc.narg('category_id') IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('start_time')  IS NULL OR time >= sqlc.narg('start_time'))
  AND (sqlc.narg('end_time')    IS NULL OR time <= sqlc.narg('end_time'))
ORDER BY time, id;

-- name: UpdateTransaction :one
UPDATE transactions
SET type = ?, account_id = ?, category_id = ?, amount = ?, time = ?, comment = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteTransaction :exec
DELETE FROM transactions
WHERE id = ?;

-- name: SummarizeTransactionsByCurrency :many
-- Aggregates income/expense/balance_adjustment totals grouped by account
-- currency, over an optional [start_time, end_time] window. Used by
-- get_financial_summary (internal/tools/analytics.go). expense amounts are
-- stored negative, so SUM(-amount) turns them back into a positive total;
-- balance_adjustment is reported here but excluded from income/expense by
-- SummarizeTransactionsByCategory/ByAccount below.
SELECT
    a.currency AS currency,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0) AS INTEGER) AS income,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'expense' THEN -t.amount ELSE 0 END), 0) AS INTEGER) AS expense,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'balance_adjustment' THEN t.amount ELSE 0 END), 0) AS INTEGER) AS balance_adjustment
FROM transactions t
JOIN accounts a ON a.id = t.account_id
WHERE (sqlc.narg('start_time') IS NULL OR t.time >= sqlc.narg('start_time'))
  AND (sqlc.narg('end_time')   IS NULL OR t.time <= sqlc.narg('end_time'))
GROUP BY a.currency
ORDER BY a.currency;

-- name: SummarizeTransactionsByCategory :many
-- Aggregates income/expense totals grouped by category and account
-- currency, over an optional [start_time, end_time] window.
-- balance_adjustment transactions have no category and are excluded.
SELECT
    t.category_id AS category_id,
    a.currency AS currency,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0) AS INTEGER) AS income,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'expense' THEN -t.amount ELSE 0 END), 0) AS INTEGER) AS expense
FROM transactions t
JOIN accounts a ON a.id = t.account_id
WHERE t.type IN ('income', 'expense')
  AND (sqlc.narg('start_time') IS NULL OR t.time >= sqlc.narg('start_time'))
  AND (sqlc.narg('end_time')   IS NULL OR t.time <= sqlc.narg('end_time'))
GROUP BY t.category_id, a.currency
ORDER BY t.category_id, a.currency;

-- name: SummarizeTransactionsByAccount :many
-- Aggregates income/expense totals grouped by account, over an optional
-- [start_time, end_time] window. balance_adjustment transactions are
-- excluded (see SummarizeTransactionsByCurrency for their total).
SELECT
    t.account_id AS account_id,
    a.currency AS currency,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0) AS INTEGER) AS income,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'expense' THEN -t.amount ELSE 0 END), 0) AS INTEGER) AS expense
FROM transactions t
JOIN accounts a ON a.id = t.account_id
WHERE t.type IN ('income', 'expense')
  AND (sqlc.narg('start_time') IS NULL OR t.time >= sqlc.narg('start_time'))
  AND (sqlc.narg('end_time')   IS NULL OR t.time <= sqlc.narg('end_time'))
GROUP BY t.account_id, a.currency
ORDER BY t.account_id, a.currency;
