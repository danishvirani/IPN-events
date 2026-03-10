-- Participants table: registered attendees for an event
CREATE TABLE IF NOT EXISTS event_participants (
    id            TEXT PRIMARY KEY,
    event_id      TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    first_name    TEXT NOT NULL DEFAULT '',
    last_name     TEXT NOT NULL DEFAULT '',
    email         TEXT NOT NULL DEFAULT '',
    phone         TEXT NOT NULL DEFAULT '',
    jamatkhana    TEXT NOT NULL DEFAULT '',
    gender        TEXT NOT NULL DEFAULT '',
    company       TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT '',
    is_walkin     INTEGER NOT NULL DEFAULT 0,
    checked_in    INTEGER NOT NULL DEFAULT 0,
    checked_in_at DATETIME,
    paid          INTEGER NOT NULL DEFAULT 0,
    paid_at       DATETIME,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Unique constraint: one email per event (for upsert matching on CSV re-upload)
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_participants_event_email
    ON event_participants(event_id, email)
    WHERE email != '';

-- Add is_paid_event flag to events table
ALTER TABLE events ADD COLUMN is_paid_event INTEGER NOT NULL DEFAULT 0;
