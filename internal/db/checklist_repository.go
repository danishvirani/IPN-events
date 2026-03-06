package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

type ChecklistRepository struct {
	db *sql.DB
}

func NewChecklistRepository(db *sql.DB) *ChecklistRepository {
	return &ChecklistRepository{db: db}
}

// AddItem inserts a checklist item for an event (activates it).
// Uses INSERT OR IGNORE so re-adding is idempotent.
func (r *ChecklistRepository) AddItem(eventID, itemKey, userID string) error {
	id := uuid.New().String()
	now := time.Now()
	_, err := r.db.Exec(
		`INSERT OR IGNORE INTO event_checklist_items (id, event_id, item_key, status, updated_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, eventID, itemKey, models.ChecklistStatusPending, userID, now, now,
	)
	return err
}

// RemoveItem deletes a checklist item from an event (deactivates it).
func (r *ChecklistRepository) RemoveItem(eventID, itemKey string) error {
	_, err := r.db.Exec(
		`DELETE FROM event_checklist_items WHERE event_id = ? AND item_key = ?`,
		eventID, itemKey,
	)
	return err
}

// ToggleStatus flips an item between pending and done.
func (r *ChecklistRepository) ToggleStatus(eventID, itemKey, userID string) error {
	_, err := r.db.Exec(
		`UPDATE event_checklist_items
		 SET status = CASE WHEN status = 'pending' THEN 'done' ELSE 'pending' END,
		     updated_by = ?,
		     updated_at = datetime('now')
		 WHERE event_id = ? AND item_key = ?`,
		userID, eventID, itemKey,
	)
	return err
}

// ListByEvent returns all active checklist items for an event.
func (r *ChecklistRepository) ListByEvent(eventID string) ([]models.ChecklistItem, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, item_key, status, COALESCE(updated_by, ''), created_at, updated_at
		 FROM event_checklist_items
		 WHERE event_id = ?
		 ORDER BY created_at ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.ChecklistItem
	for rows.Next() {
		var item models.ChecklistItem
		if err := rows.Scan(&item.ID, &item.EventID, &item.ItemKey, &item.Status,
			&item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// InitializeDefaults adds the non-selectable (always-present) items for an event.
// Called once when an event is approved. conditionFn evaluates conditional items.
func (r *ChecklistRepository) InitializeDefaults(eventID, userID string, conditionFn func(string) bool) error {
	for _, gdef := range models.ChecklistGroups() {
		if gdef.Selectable {
			continue
		}
		for _, def := range models.CatalogByGroup(gdef.GroupKey) {
			if def.Condition != "" && !conditionFn(def.Condition) {
				continue
			}
			if err := r.AddItem(eventID, def.Key, userID); err != nil {
				return err
			}
		}
	}
	return nil
}
