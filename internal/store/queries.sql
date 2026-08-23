-- name: CreateSource :one
INSERT INTO sources (name, created_at, updated_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: ListSources :many
SELECT id, name, created_at, updated_at
FROM sources
ORDER BY id;

-- name: GetSource :one
SELECT id, name, created_at, updated_at
FROM sources
WHERE id = ?;

-- name: UpdateSource :one
UPDATE sources
SET name = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteSource :exec
DELETE FROM sources
WHERE id = ?;

-- name: CountTransactionsBySource :one
SELECT COUNT(*)
FROM transactions
WHERE source_id = ?;

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
INSERT INTO transactions (type, source_id, category_id, currency, amount, time, comment, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTransaction :one
SELECT id, type, source_id, category_id, currency, amount, time, comment, created_at, updated_at
FROM transactions
WHERE id = ?;

-- name: SearchTransactions :many
SELECT id, type, source_id, category_id, currency, amount, time, comment, created_at, updated_at
FROM transactions
WHERE (sqlc.narg('source_id')   IS NULL OR source_id = sqlc.narg('source_id'))
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
SET type = ?, source_id = ?, category_id = ?, currency = ?, amount = ?, time = ?, comment = ?, updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteTransaction :exec
DELETE FROM transactions
WHERE id = ?;

-- name: SummarizeTransactionsByCurrency :many
-- Aggregates income/expense totals grouped by the transaction's own
-- currency, over an optional [start_time, end_time] window. Used by
-- get_financial_summary (internal/tools/analytics.go). expense amounts are
-- stored negative, so SUM(-amount) turns them back into a positive total.
SELECT
    t.currency AS currency,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0) AS INTEGER) AS income,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'expense' THEN -t.amount ELSE 0 END), 0) AS INTEGER) AS expense
FROM transactions t
WHERE (sqlc.narg('start_time') IS NULL OR t.time >= sqlc.narg('start_time'))
  AND (sqlc.narg('end_time')   IS NULL OR t.time <= sqlc.narg('end_time'))
GROUP BY t.currency
ORDER BY t.currency;

-- name: SummarizeTransactionsByCategory :many
-- Aggregates income/expense totals grouped by category and currency, over
-- an optional [start_time, end_time] window.
SELECT
    t.category_id AS category_id,
    t.currency AS currency,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0) AS INTEGER) AS income,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'expense' THEN -t.amount ELSE 0 END), 0) AS INTEGER) AS expense
FROM transactions t
WHERE (sqlc.narg('start_time') IS NULL OR t.time >= sqlc.narg('start_time'))
  AND (sqlc.narg('end_time')   IS NULL OR t.time <= sqlc.narg('end_time'))
GROUP BY t.category_id, t.currency
ORDER BY t.category_id, t.currency;

-- name: SummarizeTransactionsBySource :many
-- Aggregates income/expense totals grouped by source and currency, over
-- an optional [start_time, end_time] window.
SELECT
    t.source_id AS source_id,
    t.currency AS currency,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0) AS INTEGER) AS income,
    CAST(COALESCE(SUM(CASE WHEN t.type = 'expense' THEN -t.amount ELSE 0 END), 0) AS INTEGER) AS expense
FROM transactions t
WHERE (sqlc.narg('start_time') IS NULL OR t.time >= sqlc.narg('start_time'))
  AND (sqlc.narg('end_time')   IS NULL OR t.time <= sqlc.narg('end_time'))
GROUP BY t.source_id, t.currency
ORDER BY t.source_id, t.currency;
