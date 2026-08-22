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

-- name: CreateCategory :one
INSERT INTO categories (name, type, parent_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListCategories :many
SELECT id, name, type, parent_id, created_at, updated_at
FROM categories
ORDER BY id;

-- name: GetCategory :one
SELECT id, name, type, parent_id, created_at, updated_at
FROM categories
WHERE id = ?;

-- name: CreateTransaction :one
INSERT INTO transactions (type, account_id, category_id, amount, time, comment, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTransaction :one
SELECT id, type, account_id, category_id, amount, time, comment, created_at
FROM transactions
WHERE id = ?;

-- name: SearchTransactions :many
SELECT id, type, account_id, category_id, amount, time, comment, created_at
FROM transactions
WHERE (sqlc.narg('account_id')  IS NULL OR account_id = sqlc.narg('account_id'))
  AND (sqlc.narg('category_id') IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('start_time')  IS NULL OR time >= sqlc.narg('start_time'))
  AND (sqlc.narg('end_time')    IS NULL OR time <= sqlc.narg('end_time'))
ORDER BY time, id;
