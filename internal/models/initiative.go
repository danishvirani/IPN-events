package models

import "time"

// Initiative represents a strategic initiative defined by admins.
type Initiative struct {
	ID        string
	Name      string
	Objective string
	Documents []InitiativeDocument
	CreatedAt time.Time
	UpdatedAt time.Time
}

// InitiativeDocument represents an uploaded document attached to an initiative.
type InitiativeDocument struct {
	ID           string
	InitiativeID string
	Filename     string // UUID-based filename on disk
	OriginalName string // original upload filename
	CreatedAt    time.Time
}
