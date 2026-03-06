-- Add role column and fix cascade on invited_by FK
PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS invites_new (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'team_member',
    invited_by TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    used_at    DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO invites_new (id, email, role, invited_by, expires_at, used_at, created_at)
    SELECT id, email, 'team_member', invited_by, expires_at, used_at, created_at FROM invites;

DROP TABLE IF EXISTS invites;

ALTER TABLE invites_new RENAME TO invites;

CREATE INDEX IF NOT EXISTS idx_invites_email ON invites(email);

PRAGMA foreign_keys = ON;
