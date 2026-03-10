package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

// TeamRepository handles CRUD for event team members.
type TeamRepository struct {
	db *sql.DB
}

// NewTeamRepository creates a new TeamRepository.
func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Create inserts a new team member.
func (r *TeamRepository) Create(m *models.TeamMember) error {
	m.ID = uuid.New().String()
	m.CreatedAt = time.Now()

	// Get next sort order for this event
	var maxSort sql.NullInt64
	_ = r.db.QueryRow(
		`SELECT MAX(sort_order) FROM event_team_members WHERE event_id = ?`,
		m.EventID,
	).Scan(&maxSort)
	m.SortOrder = int(maxSort.Int64) + 1

	_, err := r.db.Exec(
		`INSERT INTO event_team_members (id, event_id, name, role, phone, email, sort_order, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.EventID, m.Name, m.Role, m.Phone, m.Email, m.SortOrder, m.CreatedAt,
	)
	return err
}

// Delete removes a single team member.
func (r *TeamRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM event_team_members WHERE id = ?`, id)
	return err
}

// GetByID fetches a single team member (for ownership verification).
func (r *TeamRepository) GetByID(id string) (*models.TeamMember, error) {
	m := &models.TeamMember{}
	err := r.db.QueryRow(
		`SELECT id, event_id, name, role, phone, email, sort_order, created_at
		 FROM event_team_members WHERE id = ?`, id,
	).Scan(&m.ID, &m.EventID, &m.Name, &m.Role, &m.Phone, &m.Email, &m.SortOrder, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ListByEvent returns all team members for an event.
func (r *TeamRepository) ListByEvent(eventID string) ([]*models.TeamMember, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, name, role, phone, email, sort_order, created_at
		 FROM event_team_members
		 WHERE event_id = ?
		 ORDER BY sort_order ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*models.TeamMember
	for rows.Next() {
		m := &models.TeamMember{}
		if err := rows.Scan(&m.ID, &m.EventID, &m.Name, &m.Role, &m.Phone, &m.Email, &m.SortOrder, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
