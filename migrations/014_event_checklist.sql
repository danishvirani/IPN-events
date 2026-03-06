CREATE TABLE IF NOT EXISTS event_checklist_items (
    id         TEXT PRIMARY KEY,
    event_id   TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    item_key   TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done')),
    updated_by TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(event_id, item_key)
);

CREATE INDEX IF NOT EXISTS idx_checklist_event ON event_checklist_items(event_id);
