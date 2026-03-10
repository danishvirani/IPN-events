package models

import "time"

// Participant represents a registered attendee for an event.
type Participant struct {
	ID          string
	EventID     string
	FirstName   string
	LastName    string
	Email       string
	Phone       string
	Jamatkhana  string
	Gender      string
	Company     string
	Role        string
	IsWalkin    bool
	CheckedIn   bool
	CheckedInAt *time.Time
	Paid        bool
	PaidAt      *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FullName returns "FirstName LastName".
func (p *Participant) FullName() string {
	if p.FirstName == "" && p.LastName == "" {
		return ""
	}
	if p.LastName == "" {
		return p.FirstName
	}
	if p.FirstName == "" {
		return p.LastName
	}
	return p.FirstName + " " + p.LastName
}

// ParticipantCounts holds aggregate counts for an event's participants.
type ParticipantCounts struct {
	Total     int
	CheckedIn int
	Paid      int
}

// RegistrantEvent is a summary of an event a registrant attended.
type RegistrantEvent struct {
	EventID   string
	EventName string
	EventDate string
	CheckedIn bool
	Paid      bool
}

// Registrant is a deduplicated person across all events.
type Registrant struct {
	Key         string // stable ID (first participant ID in cluster)
	Name        string
	FirstName   string
	LastName    string
	Company     string
	Title       string
	Emails      []string
	Phones      []string
	Jamatkhanas []string
	Events      []RegistrantEvent
	TotalEvents int
}
