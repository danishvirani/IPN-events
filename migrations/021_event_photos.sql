CREATE TABLE IF NOT EXISTS event_photos (
    id         TEXT PRIMARY KEY,
    event_id   TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL,
    thumbnail  TEXT NOT NULL,
    caption    TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_event_photos_event ON event_photos(event_id);

ALTER TABLE events ADD COLUMN completed INTEGER NOT NULL DEFAULT 0;
