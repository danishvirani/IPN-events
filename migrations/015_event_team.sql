-- Add assigned_to column to events (defaults to creator via user_id)
ALTER TABLE events ADD COLUMN assigned_to TEXT REFERENCES users(id);

-- Backfill: set assigned_to = user_id for all existing events
UPDATE events SET assigned_to = user_id WHERE assigned_to IS NULL;

-- Project team members table
CREATE TABLE IF NOT EXISTS event_team_members (
    id         TEXT PRIMARY KEY,
    event_id   TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL,
    phone      TEXT NOT NULL DEFAULT '',
    email      TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
