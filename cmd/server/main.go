package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"path/filepath"

	"ipn-events/internal/auth"
	"ipn-events/internal/config"
	"ipn-events/internal/db"
	"ipn-events/internal/email"
	"ipn-events/web/handlers"
	webmw "ipn-events/web/middleware"
)

func main() {
	cfg := config.Load()

	// Ensure upload directory and photo subdirs exist
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Fatalf("cannot create upload dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.UploadDir, "photos", "thumbs"), 0755); err != nil {
		log.Fatalf("cannot create photo dirs: %v", err)
	}

	// Database
	sqlDB := db.Open(cfg.DBPath)
	defer sqlDB.Close()
	db.RunMigrations(sqlDB)
	db.SeedAdmin(sqlDB, cfg.AdminEmail, cfg.AdminPassword)

	// Repositories and services
	userRepo       := db.NewUserRepository(sqlDB)
	eventRepo      := db.NewEventRepository(sqlDB)
	sessionRepo    := db.NewSessionRepository(sqlDB)
	resetRepo      := db.NewPasswordResetRepository(sqlDB)
	inviteRepo     := db.NewInviteRepository(sqlDB)
	commentRepo    := db.NewCommentRepository(sqlDB)
	initiativeRepo := db.NewInitiativeRepository(sqlDB)
	sessionSvc     := auth.NewSessionService(sessionRepo, cfg.SessionDuration)

	// Base URL (for generating reset links)
	baseURL := cfg.GoogleCallbackURL
	if len(baseURL) > len("/auth/callback") {
		baseURL = baseURL[:len(baseURL)-len("/auth/callback")]
	}

	// Email service (Resend)
	emailSvc := email.NewService(cfg.ResendAPIKey, cfg.FromEmail, baseURL)
	if emailSvc.Enabled() {
		log.Printf("email: Resend configured, sending from %s", cfg.FromEmail)
	} else {
		log.Println("email: RESEND_API_KEY not set — reset links will appear in flash messages")
	}

	// Google OAuth
	googleAuth := auth.NewGoogleAuth(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.GoogleCallbackURL,
		cfg.AdminEmail,
	)

	// Periodic session cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := sessionSvc.DeleteExpired(); err != nil {
				log.Printf("session cleanup: %v", err)
			}
		}
	}()

	// Handlers
	authHandler            := handlers.NewAuthHandler(sessionSvc, userRepo, resetRepo, inviteRepo, googleAuth, cfg.AdminEmail)
	budgetRepo             := db.NewBudgetRepository(sqlDB)
	checklistRepo          := db.NewChecklistRepository(sqlDB)
	teamRepo               := db.NewTeamRepository(sqlDB)
	participantRepo        := db.NewParticipantRepository(sqlDB)
	initUpdateRepo         := db.NewInitiativeUpdateRepository(sqlDB)
	photoRepo              := db.NewPhotoRepository(sqlDB)
	dashboardHandler       := handlers.NewDashboardHandler(eventRepo, userRepo, budgetRepo, initiativeRepo)
	eventHandler           := handlers.NewEventHandler(eventRepo, commentRepo, initiativeRepo, budgetRepo, checklistRepo, teamRepo, participantRepo, photoRepo, cfg.UploadDir)
	adminEventHandler      := handlers.NewAdminEventHandler(eventRepo, commentRepo, initiativeRepo, budgetRepo, checklistRepo, teamRepo, userRepo, participantRepo, photoRepo, emailSvc, cfg.UploadDir)
	photoHandler           := handlers.NewPhotoHandler(eventRepo, photoRepo, cfg.UploadDir)
	budgetHandler          := handlers.NewBudgetHandler(eventRepo, budgetRepo)
	checklistHandler       := handlers.NewChecklistHandler(eventRepo, checklistRepo)
	teamHandler            := handlers.NewTeamHandler(eventRepo, teamRepo, userRepo)
	participantHandler     := handlers.NewParticipantHandler(eventRepo, participantRepo)
	guideHandler           := handlers.NewGuideHandler()
	inviteHandler          := handlers.NewInviteHandler(inviteRepo, userRepo, sessionSvc, googleAuth)
	adminUserHandler       := handlers.NewAdminUserHandler(userRepo, inviteRepo, resetRepo, baseURL, emailSvc)
	adminInitiativeHandler := handlers.NewAdminInitiativeHandler(initiativeRepo, initUpdateRepo, cfg.UploadDir)

	// Router
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Logger)
	r.Use(webmw.LoadSession(sessionSvc))

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Uploaded event images (stored on disk)
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadDir))))

	// Public
	r.Get("/login",         authHandler.ShowLogin)
	r.Post("/login",        authHandler.DoLogin)
	r.Post("/logout",       authHandler.DoLogout)
	r.Get("/auth/google",   authHandler.Initiate)
	r.Get("/auth/callback", authHandler.Callback)

	// Password reset (public — user clicks link from email)
	r.Get("/reset-password/{token}",  authHandler.ShowResetPassword)
	r.Post("/reset-password/{token}", authHandler.DoResetPassword)

	// Invite acceptance (public — user clicks link from email)
	r.Get("/invite/{token}",          inviteHandler.ShowAccept)
	r.Get("/invite/{token}/google",   inviteHandler.AcceptWithGoogle)
	r.Post("/invite/{token}/password", inviteHandler.AcceptWithPassword)

	// Getting-started guides (public — linked from welcome email)
	r.Get("/guide/{role}", guideHandler.Show)

	// Protected
	r.Group(func(r chi.Router) {
		r.Use(webmw.RequireAuth)

		r.Get("/", dashboardHandler.Show)
		r.Get("/member/dashboard", dashboardHandler.MemberDashboard)

		// Team member event routes
		r.Get("/events",           eventHandler.ListMine)
		r.Get("/events/new",       eventHandler.New)
		r.Post("/events",          eventHandler.Create)
		r.Get("/events/{id}",      eventHandler.Show)
		r.Get("/events/{id}/edit", eventHandler.Edit)
		r.Post("/events/{id}",     eventHandler.Update)
		r.Post("/events/{id}/comments", eventHandler.AddComment)
		r.Post("/events/{id}/photos",                     photoHandler.Upload)
		r.Post("/events/{id}/photos/{photoId}/delete",  photoHandler.Delete)
		r.Post("/events/{id}/photos/{photoId}/rotate",  photoHandler.Rotate)
		r.Post("/events/{id}/complete",              photoHandler.ToggleComplete)
		r.Post("/events/{id}/team/add",              teamHandler.AddMember)
		r.Post("/events/{id}/team/{mid}/delete",     teamHandler.DeleteMember)

		// Check-in routes (admin or event owner)
		r.Get("/admin/events/{id}/checkin",                        participantHandler.CheckinPage)
		r.Post("/admin/events/{id}/participants/{pid}/checkin",    participantHandler.ToggleCheckin)
		r.Post("/admin/events/{id}/participants/{pid}/paid",       participantHandler.TogglePaid)
		r.Post("/admin/events/{id}/participants/walkin",           participantHandler.AddWalkin)

		// Strategic Initiatives (all authenticated users: view + comment)
		r.Get("/admin/initiatives",                       adminInitiativeHandler.List)
		r.Get("/admin/initiatives/{id}",                  adminInitiativeHandler.Show)
		r.Post("/admin/initiatives/{id}/comments",        adminInitiativeHandler.AddComment)

		// Admin + Viewer shared routes (read-only access to events)
		r.Group(func(r chi.Router) {
			r.Use(webmw.RequireAdminOrViewer)

			r.Get("/admin/dashboard",          dashboardHandler.AdminDashboard)
			r.Get("/admin/gallery",            photoHandler.Gallery)
			r.Get("/admin/calendar",           adminEventHandler.Calendar)
			r.Get("/admin/roadmap",            adminEventHandler.Roadmap)
			r.Get("/admin/roadmap/pdf",        adminEventHandler.RoadmapPDF)
			r.Get("/admin/events/export-csv",  adminEventHandler.ExportCSV)
			r.Get("/admin/events",             adminEventHandler.List)
			r.Get("/admin/events/{id}",        adminEventHandler.Show)
			r.Get("/admin/budget",             budgetHandler.YearlyOverview)
		})

		// Admin-only routes (write operations)
		r.Group(func(r chi.Router) {
			r.Use(webmw.RequireAdmin)

			r.Get("/admin/events/csv-template",           adminEventHandler.DownloadTemplate)
			r.Get("/admin/events/import",                 adminEventHandler.ImportPage)
			r.Post("/admin/events/import",                adminEventHandler.ImportCSV)
			r.Get("/admin/events/{id}/edit",              adminEventHandler.AdminEdit)
			r.Post("/admin/events/{id}/edit",             adminEventHandler.AdminUpdate)
			r.Post("/admin/events/{id}/delete",           adminEventHandler.AdminDelete)
			r.Get("/admin/events/{id}/review-form",       adminEventHandler.ReviewForm)
			r.Post("/admin/events/{id}/approve",          adminEventHandler.Approve)
			r.Post("/admin/events/{id}/reject",           adminEventHandler.Reject)
			r.Post("/admin/events/{id}/set-date",         adminEventHandler.SetDate)
			r.Post("/admin/events/{id}/comments",         adminEventHandler.AddComment)
			r.Post("/admin/events/{id}/budget",                    budgetHandler.AddItem)
			r.Post("/admin/events/{id}/budget/{itemId}/delete",    budgetHandler.DeleteItem)
			r.Post("/admin/events/{id}/checklist/toggle",          checklistHandler.ToggleItem)
			r.Post("/admin/events/{id}/checklist/add",             checklistHandler.AddItem)
			r.Post("/admin/events/{id}/checklist/remove",          checklistHandler.RemoveItem)
			r.Post("/admin/events/{id}/team/add",                  teamHandler.AddMember)
			r.Post("/admin/events/{id}/team/{mid}/delete",         teamHandler.DeleteMember)
			r.Post("/admin/events/{id}/assign",                    teamHandler.AssignTo)
			r.Post("/admin/events/{id}/attendance",                adminEventHandler.UpdateAttendance)
			r.Post("/admin/events/{id}/photos",                    photoHandler.Upload)
			r.Post("/admin/events/{id}/photos/{photoId}/delete",   photoHandler.Delete)
			r.Post("/admin/events/{id}/photos/{photoId}/rotate",   photoHandler.Rotate)
			r.Post("/admin/events/{id}/complete",                  photoHandler.ToggleComplete)

			// Participant management (admin only)
			r.Get("/admin/events/{id}/participants",                       participantHandler.ListParticipants)
			r.Get("/admin/events/{id}/participants/csv-template",          participantHandler.DownloadTemplate)
			r.Post("/admin/events/{id}/participants/import",               participantHandler.ImportCSV)
			r.Get("/admin/events/{id}/participants/export",                participantHandler.ExportParticipants)
			r.Post("/admin/events/{id}/participants/{pid}/delete",         participantHandler.DeleteParticipant)
			r.Post("/admin/events/{id}/toggle-paid-event",                 participantHandler.TogglePaidEvent)
			r.Post("/admin/events/{id}/registration-mode",                 participantHandler.SetRegistrationMode)
			r.Post("/admin/events/{id}/update-counts",                     participantHandler.UpdateAttendance)

			// Cross-event registrants (admin only)
			r.Get("/admin/registrants",                                    participantHandler.ListRegistrants)
			r.Get("/admin/registrants/{key}",                              participantHandler.ShowRegistrant)

			// Strategic Initiatives (admin-only write operations)
			r.Get("/admin/initiatives/new",                   adminInitiativeHandler.NewForm)
			r.Post("/admin/initiatives",                      adminInitiativeHandler.Create)
			r.Get("/admin/initiatives/{id}/edit",             adminInitiativeHandler.EditForm)
			r.Post("/admin/initiatives/{id}",                 adminInitiativeHandler.Update)
			r.Post("/admin/initiatives/{id}/delete",          adminInitiativeHandler.Delete)
			r.Post("/admin/initiatives/{id}/documents",       adminInitiativeHandler.UploadDocument)
			r.Post("/admin/initiatives/{id}/documents/{docId}/delete", adminInitiativeHandler.DeleteDocument)

			r.Get("/admin/users",                          adminUserHandler.List)
			r.Get("/admin/users/new",                      adminUserHandler.NewUserForm)
			r.Post("/admin/users/invite",                  adminUserHandler.CreateInvite)
			r.Post("/admin/users/{id}/role",               adminUserHandler.UpdateRole)
			r.Post("/admin/users/invite/{id}/delete",      adminUserHandler.DeleteInvite)
			r.Post("/admin/users/invite/{id}/resend",      adminUserHandler.ResendInvite)
			r.Post("/admin/users/{id}/delete",             adminUserHandler.DeleteUser)
			r.Get("/admin/users/{id}/reset-password",      adminUserHandler.ResetPasswordForm)
			r.Post("/admin/users/{id}/reset-password",     adminUserHandler.ResetPassword)
			r.Post("/admin/users/{id}/send-reset",         adminUserHandler.GenerateResetLink)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	log.Printf("server listening on http://localhost:%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}