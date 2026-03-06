package models

import "time"

const (
	CommentTypeComment   = "comment"
	CommentTypeApproval  = "approval"
	CommentTypeRejection = "rejection"
	CommentTypeResubmit  = "resubmit"
)

type EventComment struct {
	ID        string
	EventID   string
	UserID    string
	UserName  string
	Comment   string
	Type      string // comment | approval | rejection | resubmit
	CreatedAt time.Time
}

// IsSystem returns true for auto-generated entries (approval/rejection/resubmit).
func (c *EventComment) IsSystem() bool {
	return c.Type != CommentTypeComment
}

// TypeLabel returns a human-readable label for the comment type.
func (c *EventComment) TypeLabel() string {
	switch c.Type {
	case CommentTypeApproval:
		return "Approved"
	case CommentTypeRejection:
		return "Rejected"
	case CommentTypeResubmit:
		return "Resubmitted"
	default:
		return "Comment"
	}
}

// TypeBadgeClass returns Tailwind classes for the type badge.
func (c *EventComment) TypeBadgeClass() string {
	switch c.Type {
	case CommentTypeApproval:
		return "bg-green-100 text-green-700"
	case CommentTypeRejection:
		return "bg-red-100 text-red-700"
	case CommentTypeResubmit:
		return "bg-yellow-100 text-yellow-800"
	default:
		return "bg-blue-100 text-blue-700"
	}
}
