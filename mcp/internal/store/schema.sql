-- Schema source of truth for tally's SQLite database. Embedded into the
-- binary and executed at startup (see db.go) -- there is no separate
-- migration framework, since there is only ever one schema version.

CREATE TABLE IF NOT EXISTS ledgers (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    comment     TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,           -- unix seconds
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sources (
    id          INTEGER PRIMARY KEY,
    ledger_id   INTEGER NOT NULL REFERENCES ledgers(id),
    name        TEXT NOT NULL,
    created_at  INTEGER NOT NULL,           -- unix seconds
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS categories (
    id          INTEGER PRIMARY KEY,
    ledger_id   INTEGER NOT NULL REFERENCES ledgers(id),
    name        TEXT NOT NULL,
    parent_id   INTEGER NOT NULL DEFAULT 0, -- 0 = top-level; otherwise the id of any other category
                                             -- in the same ledger, nesting depth unrestricted
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS transactions (
    id           INTEGER PRIMARY KEY,
    ledger_id    INTEGER NOT NULL REFERENCES ledgers(id), -- denormalized: must match source_id's
                                                            -- ledger_id, enforced in Go, not by a DB
                                                            -- constraint (see design.md)
    type         TEXT NOT NULL,             -- income / expense
    source_id    INTEGER NOT NULL REFERENCES sources(id),
    category_id  INTEGER NOT NULL REFERENCES categories(id),
    currency     TEXT NOT NULL,             -- ISO 4217 code; validated against the static
                                             -- currency table in Go code, not a DB constraint
    amount       INTEGER NOT NULL,          -- signed, in the transaction currency's smallest unit
    time         TEXT NOT NULL,             -- local wall-clock date-time as the user stated it,
                                             -- format "YYYY-MM-DD HH:MM:SS", no timezone marker;
                                             -- never interpreted or converted, compared only as a
                                             -- string (lexicographic order equals chronological
                                             -- order for this fixed-width, zero-padded format)
    comment      TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sources_ledger             ON sources(ledger_id);
CREATE INDEX IF NOT EXISTS idx_categories_ledger           ON categories(ledger_id);
CREATE INDEX IF NOT EXISTS idx_transactions_ledger_time    ON transactions(ledger_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_source_time    ON transactions(source_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_category_time  ON transactions(category_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_time           ON transactions(time);
