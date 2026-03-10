package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
	webmw "ipn-events/web/middleware"
)

// TeamHandler handles project team member operations.
type TeamHandler struct {
	eventRepo *db.EventRepository
	teamRepo  *db.TeamRepository
	userRepo  *db.UserRepository
}

// NewTeamHandler creates a new TeamHandler.
func NewTeamHandler(eventRepo *db.EventRepository, teamRepo *db.TeamRepository, userRepo *db.UserRepository) *TeamHandler {
	return &TeamHandler{eventRepo: eventRepo, teamRepo: teamRepo, userRepo: userRepo}
}

// AddMember handles POST /admin/events/{id}/team/add and /events/{id}/team/add.
func (h *TeamHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	user := webmw.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(eventID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Authorization: admin or event owner
	if !user.IsAdmin() && e.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	role := strings.TrimSpace(r.FormValue("role"))
	if role == "other" {
		role = strings.TrimSpace(r.FormValue("role_other"))
	}

	redirectURL := h.redirectURL(user, eventID)

	if name == "" || role == "" {
		setFlash(w, "error", "Name and role are required.")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	member := &models.TeamMember{
		EventID: eventID,
		Name:    name,
		Role:    role,
		Phone:   strings.TrimSpace(r.FormValue("phone")),
		Email:   strings.TrimSpace(r.FormValue("email")),
	}

	if err := h.teamRepo.Create(member); err != nil {
		setFlash(w, "error", "Failed to add team member.")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Team member added.")
	http.Redirect(w, r, redirectURL+"#team-section", http.StatusSeeOther)
}

// DeleteMember handles POST /admin/events/{id}/team/{mid}/delete and /events/{id}/team/{mid}/delete.
func (h *TeamHandler) DeleteMember(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	memberID := chi.URLParam(r, "mid")
	user := webmw.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(eventID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Authorization: admin or event owner
	if !user.IsAdmin() && e.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Verify member belongs to this event
	member, err := h.teamRepo.GetByID(memberID)
	if err != nil || member.EventID != eventID {
		http.NotFound(w, r)
		return
	}

	redirectURL := h.redirectURL(user, eventID)

	if err := h.teamRepo.Delete(memberID); err != nil {
		setFlash(w, "error", "Failed to remove team member.")
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Team member removed.")
	http.Redirect(w, r, redirectURL+"#team-section", http.StatusSeeOther)
}

// AssignTo handles POST /admin/events/{id}/assign — changes the assigned user.
func (h *TeamHandler) AssignTo(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	userID := r.FormValue("assigned_to")
	if userID == "" {
		setFlash(w, "error", "Please select a user.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}

	if err := h.eventRepo.UpdateAssignedTo(eventID, userID); err != nil {
		setFlash(w, "error", "Failed to reassign event.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Event reassigned.")
	http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
}

func (h *TeamHandler) redirectURL(user *models.User, eventID string) string {
	if user.IsAdmin() {
		return "/admin/events/" + eventID
	}
	return "/events/" + eventID
}
