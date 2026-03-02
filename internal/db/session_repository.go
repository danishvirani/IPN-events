package db

import (
	"database/sql"
	"time"

	"ipn-events/internal/models"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(token, userID string, expiresAt time.Time) error {
	_, err := r.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expiresAt,
	)
	return err
}

// GetUser returns the user associated with a valid (non-expired) session token.
func (r *SessionRepository) GetUser(token string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(`
		SELECT u.id, u.name, u.email, COALESCE(u.google_id, ''), COALESCE(u.password_hash, ''), COALESCE(u.avatar_url, ''), u.role, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = ? AND s.expires_at > datetime('now')
	`, token).Scan(&u.ID, &u.Name, &u.Email, &u.GoogleID, &u.PasswordHash, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *SessionRepository) Delete(token string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
	return err
}

func (r *SessionRepository) DeleteExpired() error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	return err
}
