-- Main initiatives table
CREATE TABLE IF NOT EXISTS strategic_initiatives (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    objective   TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Uploaded documents per initiative
CREATE TABLE IF NOT EXISTS initiative_documents (
    id              TEXT PRIMARY KEY,
    initiative_id   TEXT NOT NULL REFERENCES strategic_initiatives(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL,
    original_name   TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_init_docs_initiative ON initiative_documents(initiative_id);

-- Many-to-many: events <-> initiatives
CREATE TABLE IF NOT EXISTS event_initiatives (
    event_id       TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    initiative_id  TEXT NOT NULL REFERENCES strategic_initiatives(id) ON DELETE CASCADE,
    PRIMARY KEY (event_id, initiative_id)
);

CREATE INDEX IF NOT EXISTS idx_event_init_event ON event_initiatives(event_id);
CREATE INDEX IF NOT EXISTS idx_event_init_init ON event_initiatives(initiative_id);
