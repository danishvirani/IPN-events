package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"ipn-events/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, name, email, COALESCE(google_id, ''), COALESCE(password_hash, ''), COALESCE(avatar_url, ''), role, created_at, updated_at
		 FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.GoogleID, &u.PasswordHash, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) FindByID(id string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, name, email, COALESCE(google_id, ''), COALESCE(password_hash, ''), COALESCE(avatar_url, ''), role, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.GoogleID, &u.PasswordHash, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) FindByGoogleID(googleID string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, name, email, COALESCE(google_id, ''), COALESCE(password_hash, ''), COALESCE(avatar_url, ''), role, created_at, updated_at
		 FROM users WHERE google_id = ?`, googleID,
	).Scan(&u.ID, &u.Name, &u.Email, &u.GoogleID, &u.PasswordHash, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) ListTeamMembers() ([]*models.User, error) {
	rows, err := r.db.Query(
		`SELECT id, name, email, role, created_at FROM users WHERE role IN (?, ?) ORDER BY created_at DESC`,
		models.RoleTeamMember, models.RoleViewer,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ListAll returns every user (all roles) with auth-type fields populated.
func (r *UserRepository) ListAll() ([]*models.User, error) {
	rows, err := r.db.Query(
		`SELECT id, name, email, COALESCE(google_id,''), COALESCE(password_hash,''), role, created_at
		 FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.GoogleID, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdatePassword replaces a user's password hash.
func (r *UserRepository) UpdatePassword(userID, passwordHash string) error {
	_, err := r.db.Exec(
		`UPDATE users SET password_hash = ?, updated_at = datetime('now') WHERE id = ?`,
		passwordHash, userID,
	)
	return err
}

func (r *UserRepository) Create(id, name, email, googleID, avatarURL, role string) (*models.User, error) {
	if id == "" {
		id = uuid.New().String()
	}
	u := &models.User{
		ID:        id,
		Name:      name,
		Email:     email,
		GoogleID:  googleID,
		AvatarURL: avatarURL,
		Role:      role,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO users (id, name, email, google_id, avatar_url, role) VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Name, u.Email, u.GoogleID, u.AvatarURL, u.Role,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) CreateWithPassword(name, email, password, role string) (*models.User, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}
	hash := string(b)
	u := &models.User{
		ID:           uuid.New().String(),
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_, err = r.db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Name, u.Email, u.PasswordHash, u.Role,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// UpdateRole changes the role of a user.
func (r *UserRepository) UpdateRole(userID, role string) error {
	_, err := r.db.Exec(
		`UPDATE users SET role = ?, updated_at = datetime('now') WHERE id = ?`,
		role, userID,
	)
	return err
}

// Delete removes a user by ID.
func (r *UserRepository) Delete(userID string) error {
	_, err := r.db.Exec(`DELETE FROM users WHERE id = ?`, userID)
	return err
}

// LinkGoogle stores the name, Google ID, and avatar URL on an existing user.
// Called when an existing user (e.g. password-seeded admin) first signs in with Google.
func (r *UserRepository) LinkGoogle(userID, name, googleID, avatarURL string) error {
	_, err := r.db.Exec(
		`UPDATE users SET name = ?, google_id = ?, avatar_url = ?, updated_at = datetime('now') WHERE id = ?`,
		name, googleID, avatarURL, userID,
	)
	return err
}
