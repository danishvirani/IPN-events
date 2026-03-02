package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"ipn-events/internal/auth"
	"ipn-events/internal/db"
	"ipn-events/internal/models"
)

const (
	oauthStateCookie  = "oauth_state"
	inviteTokenCookie = "invite_token"
)

type AuthHandler struct {
	sessionSvc  *auth.SessionService
	userRepo    *db.UserRepository
	inviteRepo  *db.InviteRepository
	googleAuth  *auth.GoogleAuth
	adminEmail  string
}

func NewAuthHandler(
	sessionSvc *auth.SessionService,
	userRepo *db.UserRepository,
	inviteRepo *db.InviteRepository,
	googleAuth *auth.GoogleAuth,
	adminEmail string,
) *AuthHandler {
	return &AuthHandler{
		sessionSvc: sessionSvc,
		userRepo:   userRepo,
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

// Initiate starts the Google OAuth flow for direct login (no invite).
func (h *AuthHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.googleAuth.AuthURL(state), http.StatusFound)
}

// InviteInitiate starts the Google OAuth flow from an invite link.
// It sets the invite_token cookie so Callback can look it up.
func (h *AuthHandler) InviteInitiate(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	// Set invite token cookie so Callback can access it after OAuth redirect
	http.SetCookie(w, &http.Cookie{
		Name:     inviteTokenCookie,
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

// ShowInvite renders the invite landing page.
func (h *AuthHandler) ShowInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := h.inviteRepo.GetByID(token)
	if err != nil || !inv.IsValid() {
		render(w, r, "web/templates/invite/accept.html", map[string]interface{}{
			"Error": "This invite link is invalid or has already been used.",
		})
		return
	}
	render(w, r, "web/templates/invite/accept.html", map[string]interface{}{
		"Invite": inv,
		"Token":  token,
	})
}

// Callback handles the OAuth callback from Google.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value != r.FormValue("state") {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}
	// Clear state cookie
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Path: "/", MaxAge: -1})

	// Exchange code for user info
	gu, err := h.googleAuth.GetUser(r.Context(), r.FormValue("code"))
	if err != nil {
		http.Error(w, "Failed to get user info from Google", http.StatusInternalServerError)
		return
	}

	var user *models.User

	// Check for invite flow
	inviteCookie, err := r.Cookie(inviteTokenCookie)
	if err == nil && inviteCookie.Value != "" {
		// Clear invite cookie
		http.SetCookie(w, &http.Cookie{Name: inviteTokenCookie, Path: "/", MaxAge: -1})

		inv, err := h.inviteRepo.GetByID(inviteCookie.Value)
		if err != nil || !inv.IsValid() {
			http.Redirect(w, r, "/login?error=invite_invalid", http.StatusSeeOther)
			return
		}
		if !strings.EqualFold(inv.Email, gu.Email) {
			http.Redirect(w, r, "/login?error=email_mismatch", http.StatusSeeOther)
			return
		}

		// Create the new user
		user, err = h.userRepo.Create(uuid.New().String(), gu.Name, gu.Email, gu.Sub, gu.Picture, models.RoleTeamMember)
		if err != nil {
			// User might already exist (e.g., re-invited after account existed)
			user, err = h.userRepo.FindByEmail(gu.Email)
			if err != nil {
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
		}
		_ = h.inviteRepo.MarkUsed(inv.ID)

	} else {
		// Normal login — look up by Google ID
		user, err = h.userRepo.FindByGoogleID(gu.Sub)
		if err != nil {
			// Not found — check admin bootstrap
			if strings.EqualFold(gu.Email, h.adminEmail) && h.adminEmail != "" {
				user, err = h.userRepo.Create(uuid.New().String(), gu.Name, gu.Email, gu.Sub, gu.Picture, models.RoleAdmin)
				if err != nil {
					// Admin might already exist with same email
					user, err = h.userRepo.FindByEmail(gu.Email)
					if err != nil {
						http.Error(w, "Failed to create admin user", http.StatusInternalServerError)
						return
					}
				}
			} else {
				http.Redirect(w, r, "/login?error=not_invited", http.StatusSeeOther)
				return
			}
		}
	}

	// Create session
	token, err := h.sessionSvc.Create(user.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

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

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}