package handlers

import (
	"net/http"

	"ipn-events/internal/db"
	"ipn-events/web/middleware"
)

type DashboardHandler struct {
	eventRepo *db.EventRepository
	userRepo  *db.UserRepository
}

func NewDashboardHandler(eventRepo *db.EventRepository, userRepo *db.UserRepository) *DashboardHandler {
	return &DashboardHandler{eventRepo: eventRepo, userRepo: userRepo}
}

func (h *DashboardHandler) Show(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user.IsAdmin() {
		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/member/dashboard", http.StatusSeeOther)
	}
}

type memberDashData struct {
	Pending  int
	Approved int
	Rejected int
}

func (h *DashboardHandler) MemberDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	pending, approved, rejected, _ := h.eventRepo.CountByStatus(user.ID)
	render(w, r, "web/templates/dashboard/member.html", memberDashData{
		Pending:  pending,
		Approved: approved,
		Rejected: rejected,
	})
}

type adminDashData struct {
	TotalPending  int
	TotalApproved int
	TotalRejected int
	TotalUsers    int
}

func (h *DashboardHandler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	pending, approved, rejected, _ := h.eventRepo.CountByStatus("")
	members, _ := h.userRepo.ListTeamMembers()
	render(w, r, "web/templates/dashboard/admin.html", adminDashData{
		TotalPending:  pending,
		TotalApproved: approved,
		TotalRejected: rejected,
		TotalUsers:    len(members),
	})
}
