-- Schema source of truth for tally's SQLite database. Embedded into the
-- binary and executed at startup (see db.go) -- there is no separate
-- migration framework, since there is only ever one schema version.

CREATE TABLE IF NOT EXISTS sources (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  INTEGER NOT NULL,           -- unix seconds
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS categories (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    parent_id   INTEGER NOT NULL DEFAULT 0, -- 0 = top-level; otherwise the id of any other category,
                                             -- nesting depth unrestricted
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS transactions (
    id           INTEGER PRIMARY KEY,
    type         TEXT NOT NULL,             -- income / expense
    source_id    INTEGER NOT NULL REFERENCES sources(id),
    category_id  INTEGER NOT NULL REFERENCES categories(id),
    currency     TEXT NOT NULL,             -- ISO 4217 code; validated against the static
                                             -- currency table in Go code, not a DB constraint
    amount       INTEGER NOT NULL,          -- signed, in the transaction currency's smallest unit
    time         INTEGER NOT NULL,          -- unix seconds
    comment      TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_transactions_source_time   ON transactions(source_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_category_time ON transactions(category_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_time           ON transactions(time);
