PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS users_new (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    google_id  TEXT UNIQUE,
    role       TEXT NOT NULL DEFAULT 'team_member',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO users_new (id, name, email, role, created_at, updated_at)
    SELECT id, name, email, role, created_at, updated_at FROM users;

DROP TABLE users;

ALTER TABLE users_new RENAME TO users;

CREATE TABLE IF NOT EXISTS invites (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    invited_by TEXT NOT NULL REFERENCES users(id),
    expires_at DATETIME NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_invites_email ON invites(email);

PRAGMA foreign_keys = ON;
