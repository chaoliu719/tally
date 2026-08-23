-- Schema source of truth for tally's SQLite database. Embedded into the
-- binary and executed at startup (see db.go) -- there is no separate
-- migration framework, since there is only ever one schema version.

CREATE TABLE IF NOT EXISTS accounts (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,              -- cash / checking_account / credit_card / virtual /
                                             -- debt / receivables / investment /
                                             -- savings_account / certificate_of_deposit
    currency    TEXT NOT NULL,              -- ISO 4217 code; validated against the static
                                             -- currency table in Go code, not a DB constraint
    comment     TEXT NOT NULL DEFAULT '',
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
    type         TEXT NOT NULL,             -- income / expense / balance_adjustment
    account_id   INTEGER NOT NULL REFERENCES accounts(id),
    category_id  INTEGER REFERENCES categories(id),  -- NULL for balance_adjustment
    amount       INTEGER NOT NULL,          -- signed, in the account currency's smallest unit
    time         INTEGER NOT NULL,          -- unix seconds
    comment      TEXT NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    CHECK (
        (type IN ('income', 'expense') AND category_id IS NOT NULL) OR
        (type = 'balance_adjustment' AND category_id IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_transactions_account_time  ON transactions(account_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_category_time ON transactions(category_id, time);
CREATE INDEX IF NOT EXISTS idx_transactions_time           ON transactions(time);
