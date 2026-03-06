package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"ipn-events/internal/auth"
	"ipn-events/internal/db"
	"ipn-events/internal/models"
)

type InviteHandler struct {
	inviteRepo *db.InviteRepository
	userRepo   *db.UserRepository
	sessionSvc *auth.SessionService
	googleAuth *auth.GoogleAuth
}

func NewInviteHandler(
	inviteRepo *db.InviteRepository,
	userRepo *db.UserRepository,
	sessionSvc *auth.SessionService,
	googleAuth *auth.GoogleAuth,
) *InviteHandler {
	return &InviteHandler{
		inviteRepo: inviteRepo,
		userRepo:   userRepo,
		sessionSvc: sessionSvc,
		googleAuth: googleAuth,
	}
}

// ShowAccept renders the invite acceptance page.
func (h *InviteHandler) ShowAccept(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := h.inviteRepo.GetByID(token)
	if err != nil || !inv.IsValid() {
		render(w, r, "web/templates/invite/accept.html", map[string]interface{}{
			"Error": "This invite link is invalid or has expired.",
		})
		return
	}

	render(w, r, "web/templates/invite/accept.html", map[string]interface{}{
		"Invite":     inv,
		"Token":      token,
		"DefaultTab": "password",
	})
}

// AcceptWithGoogle redirects to Google OAuth with the invite token stored in a cookie.
func (h *InviteHandler) AcceptWithGoogle(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := h.inviteRepo.GetByID(token)
	if err != nil || !inv.IsValid() {
		http.Redirect(w, r, "/login?error=invite_invalid", http.StatusSeeOther)
		return
	}

	// Store invite token in cookie so the OAuth callback can use it
	http.SetCookie(w, &http.Cookie{
		Name:     "invite_token",
		Value:    token,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

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

// AcceptWithPassword creates a user account from the invite using a password.
func (h *InviteHandler) AcceptWithPassword(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := h.inviteRepo.GetByID(token)
	if err != nil || !inv.IsValid() {
		render(w, r, "web/templates/invite/accept.html", map[string]interface{}{
			"Error": "This invite link is invalid or has expired.",
		})
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	renderErr := func(msg string) {
		render(w, r, "web/templates/invite/accept.html", map[string]interface{}{
			"Invite":        inv,
			"Token":         token,
			"DefaultTab":    "password",
			"PasswordError": msg,
			"Name":          name,
		})
	}

	if name == "" {
		renderErr("Name is required.")
		return
	}
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters.")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match.")
		return
	}

	// Check if user already exists
	existing, _ := h.userRepo.FindByEmail(inv.Email)
	if existing != nil {
		// User already exists — update their password
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			renderErr("Something went wrong. Please try again.")
			return
		}
		_ = h.userRepo.UpdatePassword(existing.ID, string(hash))
		_ = h.inviteRepo.MarkUsed(token)

		sessionToken, err := h.sessionSvc.Create(existing.ID)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    sessionToken,
			Path:     "/",
			MaxAge:   int(72 * 3600),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Create new user
	role := inv.Role
	if role == "" {
		role = models.RoleTeamMember
	}
	user, err := h.userRepo.CreateWithPassword(name, inv.Email, password, role)
	if err != nil {
		renderErr("Failed to create account. Please try again.")
		return
	}

	_ = h.inviteRepo.MarkUsed(token)

	// Auto-login
	sessionToken, err := h.sessionSvc.Create(user.ID)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   int(72 * 3600),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to the getting-started guide for their role
	guideSlug := "member"
	if role == models.RoleViewer {
		guideSlug = "viewer"
	} else if role == models.RoleAdmin {
		guideSlug = "admin"
	}
	http.Redirect(w, r, "/guide/"+guideSlug, http.StatusSeeOther)
}

func init() {
	// Ensure uuid is used (it's needed by other handlers in this package)
	_ = uuid.New
}
