package handlers

import (
	"net/http"
	"strings"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
)

type AdminUserHandler struct {
	userRepo *db.UserRepository
}

func NewAdminUserHandler(userRepo *db.UserRepository) *AdminUserHandler {
	return &AdminUserHandler{userRepo: userRepo}
}

func (h *AdminUserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.ListTeamMembers()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	render(w, r, "web/templates/admin/users.html", users)
}

func (h *AdminUserHandler) New(w http.ResponseWriter, r *http.Request) {
	render(w, r, "web/templates/admin/user_new.html", nil)
}

func (h *AdminUserHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	role := r.FormValue("role")

	if name == "" || email == "" || password == "" {
		setFlash(w, "error", "Name, email, and password are all required.")
		http.Redirect(w, r, "/admin/users/new", http.StatusSeeOther)
		return
	}
	if len(password) < 8 {
		setFlash(w, "error", "Password must be at least 8 characters.")
		http.Redirect(w, r, "/admin/users/new", http.StatusSeeOther)
		return
	}
	if role != models.RoleTeamMember && role != models.RoleViewer {
		role = models.RoleTeamMember
	}

	_, err := h.userRepo.CreateWithPassword(name, email, password, role)
	if err != nil {
		setFlash(w, "error", "A user with that email already exists.")
		http.Redirect(w, r, "/admin/users/new", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Account created for "+email)
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
