package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

type PasswordResetRepository struct {
	db *sql.DB
}

func NewPasswordResetRepository(db *sql.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) Create(userID string, expiresAt time.Time) (*models.PasswordReset, error) {
	pr := &models.PasswordReset{
		ID:        uuid.New().String(),
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	_, err := r.db.Exec(
		`INSERT INTO password_resets (id, user_id, expires_at) VALUES (?, ?, ?)`,
		pr.ID, pr.UserID, pr.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

func (r *PasswordResetRepository) GetByID(id string) (*models.PasswordReset, error) {
	pr := &models.PasswordReset{}
	var usedAt sql.NullTime
	err := r.db.QueryRow(
		`SELECT id, user_id, expires_at, used_at, created_at FROM password_resets WHERE id = ?`, id,
	).Scan(&pr.ID, &pr.UserID, &pr.ExpiresAt, &usedAt, &pr.CreatedAt)
	if err != nil {
		return nil, err
	}
	if usedAt.Valid {
		t := usedAt.Time
		pr.UsedAt = &t
	}
	return pr, nil
}

func (r *PasswordResetRepository) MarkUsed(id string) error {
	_, err := r.db.Exec(
		`UPDATE password_resets SET used_at = datetime('now') WHERE id = ?`, id,
	)
	return err
}
