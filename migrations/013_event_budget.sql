-- Budget line items for approved events (income + expenses in one table)
CREATE TABLE IF NOT EXISTS event_budget_items (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    type        TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    category    TEXT NOT NULL,
    description TEXT,
    quantity    INTEGER NOT NULL DEFAULT 1,
    unit_amount INTEGER NOT NULL DEFAULT 0,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_budget_items_event ON event_budget_items(event_id);
