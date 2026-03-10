package handlers

import (
	"net/http"
	"time"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

type DashboardHandler struct {
	eventRepo      *db.EventRepository
	userRepo       *db.UserRepository
	budgetRepo     *db.BudgetRepository
	initiativeRepo *db.InitiativeRepository
}

func NewDashboardHandler(
	eventRepo *db.EventRepository,
	userRepo *db.UserRepository,
	budgetRepo *db.BudgetRepository,
	initiativeRepo *db.InitiativeRepository,
) *DashboardHandler {
	return &DashboardHandler{
		eventRepo:      eventRepo,
		userRepo:       userRepo,
		budgetRepo:     budgetRepo,
		initiativeRepo: initiativeRepo,
	}
}

func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user.CanViewAdmin() { // admin or viewer
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/member/dashboard", http.StatusSeeOther)
	}
}

// ── Member Dashboard ──

type memberDashData struct {
	Pending      int
	Approved     int
	Rejected     int
	RecentEvents []*models.Event
}

func (h *DashboardHandler) MemberDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	pending, approved, rejected, _ := h.eventRepo.CountByStatus(user.ID)

	// Get user's recent events (all of them, we'll limit in template or here)
	allEvents, _ := h.eventRepo.ListByUser(user.ID)
	var recent []*models.Event
	limit := 5
	if len(allEvents) < limit {
		limit = len(allEvents)
	}
	recent = allEvents[:limit]

	render(w, r, "web/templates/dashboard/member.html", memberDashData{
		Pending:      pending,
		Approved:     approved,
		Rejected:     rejected,
		RecentEvents: recent,
	})
}

// ── Admin Dashboard ──

type adminDashData struct {
	// Aggregate metrics
	Stats *models.DashboardStats

	// Team
	TotalUsers int

	// Budget (cents)
	BudgetIncome  int
	BudgetExpense int

	// Lists
	RecentEvents   []*models.Event
	UpcomingEvents []*models.Event

	// Charts
	QuarterCounts    []models.QuarterCount
	InitiativeCounts []models.InitiativeCount
	CurrentYear      int
}

// BudgetBalance returns income - expense in cents.
func (d adminDashData) BudgetBalance() int {
	return d.BudgetIncome - d.BudgetExpense
}

// BudgetIncomeDollars returns income in dollars.
func (d adminDashData) BudgetIncomeDollars() float64 {
	return float64(d.BudgetIncome) / 100.0
}

// BudgetExpenseDollars returns expense in dollars.
func (d adminDashData) BudgetExpenseDollars() float64 {
	return float64(d.BudgetExpense) / 100.0
}

// BudgetBalanceDollars returns balance in dollars.
func (d adminDashData) BudgetBalanceDollars() float64 {
	return float64(d.BudgetBalance()) / 100.0
}

// MaxQuarterCount returns the highest count among quarter data (for scaling bars).
func (d adminDashData) MaxQuarterCount() int {
	max := 0
	for _, qc := range d.QuarterCounts {
		if qc.Count > max {
			max = qc.Count
		}
	}
	if max == 0 {
		return 1 // avoid divide by zero
	}
	return max
}

// MaxInitiativeCount returns the highest count among initiatives (for scaling bars).
func (d adminDashData) MaxInitiativeCount() int {
	max := 0
	for _, ic := range d.InitiativeCounts {
		if ic.Count > max {
			max = ic.Count
		}
	}
	if max == 0 {
		return 1
	}
	return max
}

func (h *DashboardHandler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	currentYear := time.Now().Year()

	stats, _ := h.eventRepo.DashboardStats()
	if stats == nil {
		stats = &models.DashboardStats{}
	}

	members, _ := h.userRepo.ListTeamMembers()
	recentEvents, _ := h.eventRepo.RecentEvents(5)
	upcomingEvents, _ := h.eventRepo.UpcomingEvents(5)
	quarterCounts, _ := h.eventRepo.CountByQuarter(currentYear)
	initiativeCounts, _ := h.eventRepo.InitiativeEventCounts()

	budgetIncome, budgetExpense, _ := h.budgetRepo.CurrentYearBalance()

	render(w, r, "web/templates/dashboard/admin.html", adminDashData{
		Stats:            stats,
		TotalUsers:       len(members),
		BudgetIncome:     budgetIncome,
		BudgetExpense:    budgetExpense,
		RecentEvents:     recentEvents,
		UpcomingEvents:   upcomingEvents,
		QuarterCounts:    quarterCounts,
		InitiativeCounts: initiativeCounts,
		CurrentYear:      currentYear,
	})
}
