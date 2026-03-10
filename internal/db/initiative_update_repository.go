package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

// InitiativeUpdateRepository handles CRUD for the initiative activity log.
type InitiativeUpdateRepository struct {
	db *sql.DB
}

// NewInitiativeUpdateRepository creates a new repository.
func NewInitiativeUpdateRepository(db *sql.DB) *InitiativeUpdateRepository {
	return &InitiativeUpdateRepository{db: db}
}

// Create inserts a new activity log entry.
func (r *InitiativeUpdateRepository) Create(u *models.InitiativeUpdate) error {
	u.ID = uuid.New().String()
	u.CreatedAt = time.Now()
	_, err := r.db.Exec(
		`INSERT INTO initiative_updates (id, initiative_id, user_id, user_name, comment, type)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.InitiativeID, u.UserID, u.UserName, u.Comment, u.Type,
	)
	return err
}

// ListByInitiative returns all activity log entries for an initiative, oldest first.
func (r *InitiativeUpdateRepository) ListByInitiative(initiativeID string) ([]*models.InitiativeUpdate, error) {
	rows, err := r.db.Query(
		`SELECT id, initiative_id, user_id, user_name, comment, type, created_at
		 FROM initiative_updates
		 WHERE initiative_id = ?
		 ORDER BY created_at ASC`,
		initiativeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []*models.InitiativeUpdate
	for rows.Next() {
		u := &models.InitiativeUpdate{}
		if err := rows.Scan(&u.ID, &u.InitiativeID, &u.UserID, &u.UserName, &u.Comment, &u.Type, &u.CreatedAt); err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}
	return updates, rows.Err()
}

// DeleteByInitiative removes all log entries for a given initiative.
func (r *InitiativeUpdateRepository) DeleteByInitiative(initiativeID string) error {
	_, err := r.db.Exec(`DELETE FROM initiative_updates WHERE initiative_id = ?`, initiativeID)
	return err
}
