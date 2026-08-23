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
  AND (
    sqlc.narg('after_time') IS NULL
    OR time > sqlc.narg('after_time')
    OR (time = sqlc.narg('after_time') AND id > sqlc.narg('after_id'))
  )
ORDER BY time, id
LIMIT sqlc.arg('limit');

-- name: UpdateTransaction :one
UPDATE transactions
SET type = ?, account_id = ?, category_id = ?, amount = ?, time = ?, comment = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteTransaction :exec
DELETE FROM transactions
WHERE id = ?;
