package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"ipn-events/internal/db"
	"ipn-events/internal/email"
	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

type AdminUserHandler struct {
	userRepo   *db.UserRepository
	inviteRepo *db.InviteRepository
	resetRepo  *db.PasswordResetRepository
	baseURL    string
	emailSvc   *email.Service
}

func NewAdminUserHandler(
	userRepo *db.UserRepository,
	inviteRepo *db.InviteRepository,
	resetRepo *db.PasswordResetRepository,
	baseURL string,
	emailSvc *email.Service,
) *AdminUserHandler {
	return &AdminUserHandler{
		userRepo:   userRepo,
		inviteRepo: inviteRepo,
		resetRepo:  resetRepo,
		baseURL:    baseURL,
		emailSvc:   emailSvc,
	}
}

func (h *AdminUserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.ListAll()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	invites, err := h.inviteRepo.ListAll()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	render(w, r, "web/templates/admin/users.html", map[string]interface{}{
		"Users":   users,
		"Invites": invites,
	})
}

// NewUserForm renders the invite form.
func (h *AdminUserHandler) NewUserForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, "web/templates/admin/user_new.html", nil)
}

// CreateInvite creates an invite and sends the email.
func (h *AdminUserHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	inviteEmail := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	role := r.FormValue("role")

	if inviteEmail == "" {
		setFlash(w, "error", "Email is required.")
		http.Redirect(w, r, "/admin/users/new", http.StatusSeeOther)
		return
	}
	if role != models.RoleTeamMember && role != models.RoleViewer && role != models.RoleAdmin {
		role = models.RoleTeamMember
	}

	// Check if user already exists
	existing, _ := h.userRepo.FindByEmail(inviteEmail)
	if existing != nil {
		setFlash(w, "error", fmt.Sprintf("%s already has an account.", inviteEmail))
		http.Redirect(w, r, "/admin/users/new", http.StatusSeeOther)
		return
	}

	currentUser := middleware.UserFromContext(r.Context())
	inv, err := h.inviteRepo.Create(inviteEmail, role, currentUser.ID, time.Now().Add(7*24*time.Hour))
	if err != nil {
		setFlash(w, "error", "Failed to create invite.")
		http.Redirect(w, r, "/admin/users/new", http.StatusSeeOther)
		return
	}

	if h.emailSvc != nil && h.emailSvc.Enabled() {
		if err := h.emailSvc.SendInvite(inviteEmail, inv.ID, role); err != nil {
			log.Printf("email: send invite to %s: %v", inviteEmail, err)
			link := fmt.Sprintf("%s/invite/%s", h.baseURL, inv.ID)
			setFlash(w, "error", fmt.Sprintf("Email failed. Share this link with %s: %s", inviteEmail, link))
		} else {
			setFlash(w, "success", fmt.Sprintf("Invitation sent to %s.", inviteEmail))
		}
	} else {
		link := fmt.Sprintf("%s/invite/%s", h.baseURL, inv.ID)
		setFlash(w, "success", fmt.Sprintf("Invite link for %s: %s", inviteEmail, link))
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// DeleteUser removes a user and revokes their access.
func (h *AdminUserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Prevent self-deletion
	currentUser := middleware.UserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == id {
		setFlash(w, "error", "You cannot delete your own account.")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	if err := h.userRepo.Delete(id); err != nil {
		setFlash(w, "error", "Failed to delete user.")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "User removed.")
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// DeleteInvite revokes a pending invitation.
func (h *AdminUserHandler) DeleteInvite(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.inviteRepo.Delete(id); err != nil {
		setFlash(w, "error", "Failed to revoke invitation.")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}
	setFlash(w, "success", "Invitation revoked.")
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// UpdateRole changes a user's role.
func (h *AdminUserHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	role := r.FormValue("role")

	if role != models.RoleTeamMember && role != models.RoleViewer && role != models.RoleAdmin {
		setFlash(w, "error", "Invalid role.")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	if err := h.userRepo.UpdateRole(id, role); err != nil {
		setFlash(w, "error", "Failed to update role.")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Role updated.")
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// ResetPasswordForm renders the admin set-new-password form for a DB user.
func (h *AdminUserHandler) ResetPasswordForm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, err := h.userRepo.FindByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, "web/templates/admin/user_reset_password.html", user)
}

// ResetPassword sets a new password directly (admin-side).
func (h *AdminUserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	if len(password) < 8 {
		setFlash(w, "error", "Password must be at least 8 characters.")
		http.Redirect(w, r, "/admin/users/"+id+"/reset-password", http.StatusSeeOther)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.userRepo.UpdatePassword(id, string(hash)); err != nil {
		setFlash(w, "error", "Failed to update password.")
		http.Redirect(w, r, "/admin/users/"+id+"/reset-password", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Password updated.")
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// GenerateResetLink creates a 24-hour password reset token.
func (h *AdminUserHandler) GenerateResetLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	user, err := h.userRepo.FindByID(id)
	if err != nil {
		setFlash(w, "error", "User not found.")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	pr, err := h.resetRepo.Create(id, time.Now().Add(24*time.Hour))
	if err != nil {
		setFlash(w, "error", "Failed to generate reset link.")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	if h.emailSvc != nil && h.emailSvc.Enabled() {
		if err := h.emailSvc.SendPasswordReset(user.Email, user.Name, pr.ID, user.Role); err != nil {
			log.Printf("email: send reset to %s: %v", user.Email, err)
			link := fmt.Sprintf("%s/reset-password/%s", h.baseURL, pr.ID)
			setFlash(w, "error", fmt.Sprintf("Email failed to send. Share this link with %s (valid 24 h): %s", user.Email, link))
		} else {
			setFlash(w, "success", fmt.Sprintf("Reset email sent to %s.", user.Email))
		}
	} else {
		link := fmt.Sprintf("%s/reset-password/%s", h.baseURL, pr.ID)
		setFlash(w, "success", "Reset link (valid 24 h): "+link)
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
