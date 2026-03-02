package middleware

import (
	"context"
	"net/http"

	"ipn-events/internal/auth"
	"ipn-events/internal/models"
)

type contextKey string

const contextKeyUser contextKey = "user"

// LoadSession reads the session cookie and, if valid, stores the user in the request context.
// Always runs; does not redirect on failure.
func LoadSession(sessionSvc *auth.SessionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err == nil {
				user, err := sessionSvc.GetUser(cookie.Value)
				if err == nil {
					ctx := context.WithValue(r.Context(), contextKeyUser, user)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth redirects to /login if the user is not authenticated.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			http.Redirect(w, r, "/login?redirect="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin returns 403 if the authenticated user is not an admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil || !u.IsAdmin() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdminOrViewer returns 403 unless the user is an admin or read-only viewer.
func RequireAdminOrViewer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil || !u.CanViewAdmin() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromContext retrieves the user stored by LoadSession, or nil.
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(contextKeyUser).(*models.User)
	return u
}
