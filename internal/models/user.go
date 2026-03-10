package models

import "time"

const (
	RoleAdmin      = "admin"
	RoleTeamMember = "team_member"
	RoleViewer     = "viewer"
)

type User struct {
	ID           string
	Name         string
	Email        string
	GoogleID     string
	PasswordHash string
	AvatarURL    string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u *User) IsViewer() bool {
	return u.Role == RoleViewer
}

func (u *User) CanViewAdmin() bool {
	return u.Role == RoleAdmin || u.Role == RoleViewer
}
