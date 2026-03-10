package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"ipn-events/internal/auth"
	"ipn-events/internal/db"
	"ipn-events/internal/models"
)

const oauthStateCookie = "oauth_state"

type AuthHandler struct {
	sessionSvc *auth.SessionService
	userRepo   *db.UserRepository
	resetRepo  *db.PasswordResetRepository
	inviteRepo *db.InviteRepository
	googleAuth *auth.GoogleAuth
	adminEmail string
}

func NewAuthHandler(
	sessionSvc *auth.SessionService,
	userRepo *db.UserRepository,
	resetRepo *db.PasswordResetRepository,
	inviteRepo *db.InviteRepository,
	googleAuth *auth.GoogleAuth,
	adminEmail string,
) *AuthHandler {
	return &AuthHandler{
		sessionSvc: sessionSvc,
		userRepo:   userRepo,
		resetRepo:  resetRepo,
		inviteRepo: inviteRepo,
		googleAuth: googleAuth,
		adminEmail: adminEmail,
	}
}

func (h *AuthHandler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	render(w, r, "web/templates/auth/login.html", map[string]string{
		"Error": r.URL.Query().Get("error"),
	})
}

// Initiate starts the Google OAuth flow.
func (h *AuthHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.googleAuth.AuthURL(state), http.StatusFound)
}

// Callback handles the OAuth callback from Google.
// New users are auto-created as team_member; the admin can adjust roles from the user list.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value != r.FormValue("state") {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Path: "/", MaxAge: -1})

	gu, err := h.googleAuth.GetUser(r.Context(), r.FormValue("code"))
	if err != nil {
		http.Error(w, "Failed to get user info from Google", http.StatusInternalServerError)
		return
	}

	// Check for invite token cookie (set by AcceptWithGoogle)
	var inviteToken string
	var inviteRole string
	if c, err := r.Cookie("invite_token"); err == nil && c.Value != "" {
		inviteToken = c.Value
		if inv, err := h.inviteRepo.GetByID(inviteToken); err == nil && inv.IsValid() {
			inviteRole = inv.Role
		}
		// Clear the cookie regardless
		http.SetCookie(w, &http.Cookie{Name: "invite_token", Path: "/", MaxAge: -1})
	}

	// Try to find existing user by Google ID
	user, err := h.userRepo.FindByGoogleID(gu.Sub)
	if err != nil {
		// Not found — bootstrap admin or auto-create team member
		if strings.EqualFold(gu.Email, h.adminEmail) && h.adminEmail != "" {
			user, err = h.userRepo.Create(uuid.New().String(), gu.Name, gu.Email, gu.Sub, gu.Picture, models.RoleAdmin)
			if err != nil {
				// Admin already exists (seeded with password) — link Google profile
				user, err = h.userRepo.FindByEmail(gu.Email)
				if err != nil {
					http.Error(w, "Failed to create admin user", http.StatusInternalServerError)
					return
				}
				_ = h.userRepo.LinkGoogle(user.ID, gu.Name, gu.Sub, gu.Picture)
				user.Name = gu.Name
				user.GoogleID = gu.Sub
				user.AvatarURL = gu.Picture
			}
		} else {
			// Determine role: use invite role if present, otherwise default to team_member
			role := models.RoleTeamMember
			if inviteRole != "" {
				role = inviteRole
			}
			user, err = h.userRepo.Create(uuid.New().String(), gu.Name, gu.Email, gu.Sub, gu.Picture, role)
			if err != nil {
				// User exists by email but different Google ID — link their account
				user, err = h.userRepo.FindByEmail(gu.Email)
				if err != nil {
					http.Error(w, "Failed to create user", http.StatusInternalServerError)
					return
				}
				_ = h.userRepo.LinkGoogle(user.ID, gu.Name, gu.Sub, gu.Picture)
				// If accepting an invite, update their role to the invited role
				if inviteRole != "" {
					_ = h.userRepo.UpdateRole(user.ID, inviteRole)
					user.Role = inviteRole
				}
			}
		}
	} else if inviteRole != "" {
		// Existing user signing in via invite — update their role
		_ = h.userRepo.UpdateRole(user.ID, inviteRole)
		user.Role = inviteRole
	}

	// Mark invite as used if present
	if inviteToken != "" {
		_ = h.inviteRepo.MarkUsed(inviteToken)
	}

	token, err := h.sessionSvc.Create(user.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	_ = h.userRepo.UpdateLastLogin(user.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(72 * 3600),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// DoLogin handles password-based login for DB users.
func (h *AuthHandler) DoLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")

	user, err := h.userRepo.FindByEmail(email)
	if err != nil || user.PasswordHash == "" || !auth.CheckPassword(user.PasswordHash, password) {
		http.Redirect(w, r, "/login?error=invalid", http.StatusSeeOther)
		return
	}

	token, err := h.sessionSvc.Create(user.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	_ = h.userRepo.UpdateLastLogin(user.ID)

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(72 * 3600),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) DoLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil {
		_ = h.sessionSvc.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   auth.SessionCookieName,
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ShowResetPassword renders the password-reset form for a valid token.
func (h *AuthHandler) ShowResetPassword(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	pr, err := h.resetRepo.GetByID(token)
	if err != nil || !pr.IsValid() {
		render(w, r, "web/templates/auth/reset_password.html", map[string]interface{}{
			"Error": "This reset link is invalid or has expired.",
		})
		return
	}
	render(w, r, "web/templates/auth/reset_password.html", map[string]interface{}{
		"Token": token,
	})
}

// DoResetPassword validates the token and updates the user's password.
func (h *AuthHandler) DoResetPassword(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	pr, err := h.resetRepo.GetByID(token)
	if err != nil || !pr.IsValid() {
		render(w, r, "web/templates/auth/reset_password.html", map[string]interface{}{
			"Error": "This reset link is invalid or has expired.",
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	renderErr := func(msg string) {
		render(w, r, "web/templates/auth/reset_password.html", map[string]interface{}{
			"Token": token,
			"Error": msg,
		})
	}

	if len(password) < 8 {
		renderErr("Password must be at least 8 characters.")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match.")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if err := h.userRepo.UpdatePassword(pr.UserID, string(hash)); err != nil {
		renderErr("Failed to update password. Please try again.")
		return
	}
	_ = h.resetRepo.MarkUsed(token)

	// Log the user in automatically
	sessionToken, err := h.sessionSvc.Create(pr.UserID)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	_ = h.userRepo.UpdateLastLogin(pr.UserID)
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   int(72 * 3600),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
