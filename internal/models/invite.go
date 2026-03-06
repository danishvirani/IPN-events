package models

import "time"

type Invite struct {
	ID        string
	Email     string
	Role      string
	InvitedBy string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (i *Invite) IsValid() bool {
	return i.UsedAt == nil && time.Now().Before(i.ExpiresAt)
}

func (i *Invite) Status() string {
	if i.UsedAt != nil {
		return "used"
	}
	if time.Now().After(i.ExpiresAt) {
		return "expired"
	}
	return "pending"
}
