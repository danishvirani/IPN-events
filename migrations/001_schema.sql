CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'team_member',
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id    ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS events (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    quarter       TEXT,
    year          INTEGER,
    description   TEXT NOT NULL,
    outcome       TEXT,
    impact        TEXT,
    status        TEXT NOT NULL DEFAULT 'pending',
    admin_comment TEXT,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id);
CREATE INDEX IF NOT EXISTS idx_events_status  ON events(status);

CREATE TABLE IF NOT EXISTS event_inputs (
    id                  TEXT PRIMARY KEY,
    event_id            TEXT NOT NULL UNIQUE REFERENCES events(id) ON DELETE CASCADE,
    financial_resources TEXT,
    facilities          TEXT,
    human_support       TEXT,
    technology          TEXT,
    partnerships        TEXT
);

CREATE TABLE IF NOT EXISTS event_activities (
    id                     TEXT PRIMARY KEY,
    event_id               TEXT NOT NULL UNIQUE REFERENCES events(id) ON DELETE CASCADE,
    structured_programming TEXT,
    engagement_design      TEXT,
    content_delivery       TEXT,
    community_building     TEXT
);

CREATE TABLE IF NOT EXISTS event_output_items (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_output_items_event_id ON event_output_items(event_id);

CREATE TABLE IF NOT EXISTS event_support_requests (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    description TEXT NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_support_requests_event_id ON event_support_requests(event_id);
