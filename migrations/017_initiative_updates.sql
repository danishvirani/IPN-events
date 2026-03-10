CREATE TABLE IF NOT EXISTS initiative_updates (
    id              TEXT PRIMARY KEY,
    initiative_id   TEXT NOT NULL REFERENCES strategic_initiatives(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_name       TEXT NOT NULL,
    comment         TEXT NOT NULL,
    type            TEXT NOT NULL DEFAULT 'comment',
    created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_initiative_updates_initiative ON initiative_updates(initiative_id);
CREATE INDEX IF NOT EXISTS idx_initiative_updates_created ON initiative_updates(created_at);
