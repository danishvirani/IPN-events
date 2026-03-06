CREATE TABLE IF NOT EXISTS event_comments (
    id         TEXT PRIMARY KEY,
    event_id   TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_name  TEXT NOT NULL,
    comment    TEXT NOT NULL,
    type       TEXT NOT NULL DEFAULT 'comment',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_event_comments_event_id ON event_comments(event_id);
CREATE INDEX IF NOT EXISTS idx_event_comments_created_at ON event_comments(created_at);
