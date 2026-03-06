package models

import "time"

type PasswordReset struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (pr *PasswordReset) IsValid() bool {
	return pr.UsedAt == nil && time.Now().Before(pr.ExpiresAt)
}
