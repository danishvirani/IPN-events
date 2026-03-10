package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

type InviteRepository struct {
	db *sql.DB
}

func NewInviteRepository(db *sql.DB) *InviteRepository {
	return &InviteRepository{db: db}
}

func (r *InviteRepository) Create(email, role, invitedByID string, expiresAt time.Time) (*models.Invite, error) {
	inv := &models.Invite{
		ID:        uuid.New().String(),
		Email:     email,
		Role:      role,
		InvitedBy: invitedByID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO invites (id, email, role, invited_by, expires_at) VALUES (?, ?, ?, ?, ?)`,
		inv.ID, inv.Email, inv.Role, inv.InvitedBy, inv.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *InviteRepository) GetByID(id string) (*models.Invite, error) {
	inv := &models.Invite{}
	var usedAt sql.NullTime
	err := r.db.QueryRow(
		`SELECT id, email, role, invited_by, expires_at, used_at, created_at
		 FROM invites WHERE id = ?`, id,
	).Scan(&inv.ID, &inv.Email, &inv.Role, &inv.InvitedBy, &inv.ExpiresAt, &usedAt, &inv.CreatedAt)
	if err != nil {
		return nil, err
	}
	if usedAt.Valid {
		t := usedAt.Time
		inv.UsedAt = &t
	}
	return inv, nil
}

func (r *InviteRepository) MarkUsed(id string) error {
	_, err := r.db.Exec(
		`UPDATE invites SET used_at = datetime('now') WHERE id = ?`, id,
	)
	return err
}

func (r *InviteRepository) UpdateExpiry(id string, expiresAt time.Time) error {
	_, err := r.db.Exec(`UPDATE invites SET expires_at = ? WHERE id = ?`, expiresAt, id)
	return err
}

func (r *InviteRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM invites WHERE id = ?`, id)
	return err
}

func (r *InviteRepository) ListAll() ([]*models.Invite, error) {
	rows, err := r.db.Query(
		`SELECT id, email, role, invited_by, expires_at, used_at, created_at
		 FROM invites WHERE used_at IS NULL ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []*models.Invite
	for rows.Next() {
		inv := &models.Invite{}
		var usedAt sql.NullTime
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.Role, &inv.InvitedBy, &inv.ExpiresAt, &usedAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		if usedAt.Valid {
			t := usedAt.Time
			inv.UsedAt = &t
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}
