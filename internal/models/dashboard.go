package models

// DashboardStats holds aggregate event counts for the admin dashboard.
type DashboardStats struct {
	Total              int
	Pending            int
	Approved           int
	Rejected           int
	TotalRegistrations int
	TotalParticipants  int
}

// QuarterCount holds a quarter label and its event count.
type QuarterCount struct {
	Quarter string
	Count   int
}

// InitiativeCount holds an initiative and how many events are tagged to it.
type InitiativeCount struct {
	ID    string
	Name  string
	Count int
}
