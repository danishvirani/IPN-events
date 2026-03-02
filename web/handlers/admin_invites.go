package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"ipn-events/internal/db"
	webmw "ipn-events/web/middleware"
)

type AdminInviteHandler struct {
	inviteRepo *db.InviteRepository
	baseURL    string
}

func NewAdminInviteHandler(inviteRepo *db.InviteRepository, baseURL string) *AdminInviteHandler {
	return &AdminInviteHandler{inviteRepo: inviteRepo, baseURL: baseURL}
}

func (h *AdminInviteHandler) List(w http.ResponseWriter, r *http.Request) {
	invites, err := h.inviteRepo.ListAll()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	type inviteRow struct {
		ID        string
		Email     string
		Status    string
		ExpiresAt string
		InviteURL string
	}

	var rows []inviteRow
	for _, inv := range invites {
		rows = append(rows, inviteRow{
			ID:        inv.ID,
			Email:     inv.Email,
			Status:    inv.Status(),
			ExpiresAt: inv.ExpiresAt.Format("Jan 2, 2006"),
			InviteURL: fmt.Sprintf("%s/invite/%s", h.baseURL, inv.ID),
		})
	}

	render(w, r, "web/templates/admin/invites.html", rows)
}

func (h *AdminInviteHandler) New(w http.ResponseWriter, r *http.Request) {
	render(w, r, "web/templates/admin/invites.html", nil)
}

func (h *AdminInviteHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	if email == "" {
		setFlash(w, "error", "Email is required.")
		http.Redirect(w, r, "/admin/invites", http.StatusSeeOther)
		return
	}

	currentUser := webmw.UserFromContext(r.Context())
	if currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	inv, err := h.inviteRepo.Create(email, currentUser.ID, time.Now().Add(7*24*time.Hour))
	if err != nil {
		setFlash(w, "error", "Failed to create invite: "+err.Error())
		http.Redirect(w, r, "/admin/invites", http.StatusSeeOther)
		return
	}

	inviteURL := fmt.Sprintf("%s/invite/%s", h.baseURL, inv.ID)
	setFlash(w, "success", "Invite created for "+email+". Link: "+inviteURL)
	http.Redirect(w, r, "/admin/invites", http.StatusSeeOther)
}