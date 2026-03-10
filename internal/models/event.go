package models

import (
	"fmt"
	"time"
)

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

const (
	SupportTypeSpace        = "space"
	SupportTypeAnnouncement = "announcement"
	SupportTypeMaterials    = "materials"
)

const (
	RecurrenceNone      = "none"
	RecurrenceWeekly    = "weekly"
	RecurrenceBiWeekly  = "bi-weekly"
	RecurrenceMonthly   = "monthly"
	RecurrenceQuarterly = "quarterly"
	RecurrenceAnnual    = "annual"
)

const (
	ScopeRegional = "regional"
	ScopeLocal    = "local"
)

const (
	VenueTypeInternal = "internal"
	VenueTypeExternal = "external"
)

var Jamatkhanas = []string{
	"Atlanta Northeast",
	"Atlanta Northwest",
	"Atlanta Headquarters",
	"Atlanta South",
	"Duluth",
	"Birmingham",
	"Chattanooga",
	"Memphis",
	"Spartanburg",
	"Nashville",
	"Knoxville",
}

type Event struct {
	ID        string
	UserID    string
	UserName  string
	UserEmail string

	Name        string
	Quarter     string
	Year        int
	Recurrence        string
	RecurrenceEndDate string // YYYY-MM-DD; empty = no end date (runs through year)
	EventDate         string
	StartTime         string // HH:MM (24h)
	EndTime           string // HH:MM (24h)
	ImagePath         string // relative path under uploads dir, e.g. "abc123.jpg"
	Description string

	City            string
	Scope           string
	ScopeJamatkhana string
	VenueType       string
	VenueJamatkhana string
	VenueAddress    string

	Input       EventInput
	Activities  EventActivities
	OutputItems []OutputItem
	Outcome     string
	Impact      string

	SupportRequests []SupportRequest
	Initiatives     []Initiative
	InitiativeIDs   []string `json:"-"` // used during form submission

	AssignedToID   string // user ID of the person assigned to this event
	AssignedToName string // denormalized for display

	RegistrationCount  int    // 0 = not set; meaningful only for approved events
	ParticipationCount int    // 0 = not set; meaningful only for approved events
	IsPaidEvent        bool
	RegistrationMode   string // "full" (default) or "count_only"
	AttendanceCount    int    // manual override for actual attendance

	Status       string
	AdminComment string

	CreatedAt time.Time
	UpdatedAt time.Time
}

type EventInput struct {
	FinancialResources string
	Facilities         string
	HumanSupport       string
	Technology         string
	Partnerships       string
}

type EventActivities struct {
	StructuredProgramming string
	EngagementDesign      string
	ContentDelivery       string
	CommunityBuilding     string
}

type OutputItem struct {
	ID          string `json:"id"`
	EventID     string `json:"eventId"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
}

type SupportRequest struct {
	ID          string `json:"id"`
	EventID     string `json:"eventId"`
	Type        string `json:"type"`
	Description string `json:"description"`
	VenueType   string `json:"venueType"`
	VenueDetail string `json:"venueDetail"`
	SortOrder   int    `json:"sortOrder"`
}

// TeamMember represents a person on the event's project team.
type TeamMember struct {
	ID        string
	EventID   string
	Name      string
	Role      string
	Phone     string
	Email     string
	SortOrder int
	CreatedAt time.Time
}

func (e *Event) QuarterYear() string {
	if e.Quarter == "" && e.Year == 0 {
		return "TBD"
	}
	if e.Quarter == "" {
		return fmt.Sprintf("%d", e.Year)
	}
	if e.Year == 0 {
		return e.Quarter
	}
	return fmt.Sprintf("%s %d", e.Quarter, e.Year)
}

func (e *Event) StatusBadgeClass() string {
	switch e.Status {
	case StatusApproved:
		return "bg-green-100 text-green-800"
	case StatusRejected:
		return "bg-red-100 text-red-800"
	default:
		return "bg-yellow-100 text-yellow-800"
	}
}
