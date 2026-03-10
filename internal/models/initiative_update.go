package models

import "time"

const (
	UpdateTypeComment    = "comment"
	UpdateTypeEdit       = "update"
	UpdateTypeDocAdded   = "document_added"
	UpdateTypeDocRemoved = "document_removed"
)

// InitiativeUpdate is a single entry in an initiative's activity log.
type InitiativeUpdate struct {
	ID           string
	InitiativeID string
	UserID       string
	UserName     string
	Comment      string
	Type         string // comment | update | document_added | document_removed
	CreatedAt    time.Time
}

// IsSystem returns true for auto-generated log entries (non-user comments).
func (u *InitiativeUpdate) IsSystem() bool {
	return u.Type != UpdateTypeComment
}

// TypeLabel returns a human-readable label for the entry type.
func (u *InitiativeUpdate) TypeLabel() string {
	switch u.Type {
	case UpdateTypeEdit:
		return "Updated"
	case UpdateTypeDocAdded:
		return "Document Added"
	case UpdateTypeDocRemoved:
		return "Document Removed"
	default:
		return "Comment"
	}
}

// TypeBadgeClass returns Tailwind CSS classes for the type badge.
func (u *InitiativeUpdate) TypeBadgeClass() string {
	switch u.Type {
	case UpdateTypeEdit:
		return "bg-indigo-100 text-indigo-800"
	case UpdateTypeDocAdded:
		return "bg-green-100 text-green-800"
	case UpdateTypeDocRemoved:
		return "bg-red-100 text-red-800"
	default:
		return "bg-blue-100 text-blue-800"
	}
}
