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

	"ipn-events/internal/auth"
	"ipn-events/internal/config"
	"ipn-events/internal/db"
	"ipn-events/web/handlers"
	webmw "ipn-events/web/middleware"
)

func main() {
	cfg := config.Load()

	// Ensure upload directory exists
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Fatalf("cannot create upload dir: %v", err)
	}

	// Database
	sqlDB := db.Open(cfg.DBPath)
	defer sqlDB.Close()
	db.RunMigrations(sqlDB)
	db.SeedAdmin(sqlDB, cfg.AdminEmail, cfg.AdminPassword)

	// Repositories and services
	userRepo    := db.NewUserRepository(sqlDB)
	eventRepo   := db.NewEventRepository(sqlDB)
	sessionRepo := db.NewSessionRepository(sqlDB)
	inviteRepo  := db.NewInviteRepository(sqlDB)
	sessionSvc  := auth.NewSessionService(sessionRepo, cfg.SessionDuration)

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

	// Base URL for invite links
	baseURL := cfg.GoogleCallbackURL
	// Strip "/auth/callback" to get the base URL
	if len(baseURL) > len("/auth/callback") {
		baseURL = baseURL[:len(baseURL)-len("/auth/callback")]
	}

	// Handlers
	authHandler       := handlers.NewAuthHandler(sessionSvc, userRepo, inviteRepo, googleAuth, cfg.AdminEmail)
	dashboardHandler  := handlers.NewDashboardHandler(eventRepo, userRepo)
	eventHandler      := handlers.NewEventHandler(eventRepo, cfg.UploadDir)
	adminEventHandler := handlers.NewAdminEventHandler(eventRepo)
	adminUserHandler  := handlers.NewAdminUserHandler(userRepo)
	adminInviteHandler := handlers.NewAdminInviteHandler(inviteRepo, baseURL)

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

	// Invite flow (public)
	r.Get("/invite/{token}",          authHandler.ShowInvite)
	r.Get("/invite/{token}/google",   authHandler.InviteInitiate)
	r.Post("/invite/{token}/password", authHandler.InviteSetPassword)

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

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(webmw.RequireAdmin)

			r.Get("/admin/dashboard", dashboardHandler.AdminDashboard)

			r.Get("/admin/calendar",      adminEventHandler.Calendar)
			r.Get("/admin/roadmap",       adminEventHandler.Roadmap)
			r.Get("/admin/roadmap/pdf",   adminEventHandler.RoadmapPDF)

			r.Get("/admin/events",                        adminEventHandler.List)
			r.Get("/admin/events/csv-template",           adminEventHandler.DownloadTemplate)
			r.Get("/admin/events/import",                 adminEventHandler.ImportPage)
			r.Post("/admin/events/import",                adminEventHandler.ImportCSV)
			r.Get("/admin/events/{id}",                   adminEventHandler.Show)
			r.Get("/admin/events/{id}/review-form",       adminEventHandler.ReviewForm)
			r.Post("/admin/events/{id}/approve",          adminEventHandler.Approve)
			r.Post("/admin/events/{id}/reject",           adminEventHandler.Reject)
			r.Post("/admin/events/{id}/set-date",         adminEventHandler.SetDate)

			r.Get("/admin/users",      adminUserHandler.List)
			r.Get("/admin/users/new",  adminUserHandler.New)
			r.Post("/admin/users",     adminUserHandler.Create)

			r.Get("/admin/invites",  adminInviteHandler.List)
			r.Post("/admin/invites", adminInviteHandler.Create)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
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