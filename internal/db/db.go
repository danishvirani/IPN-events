package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	"ipn-events/internal/models"
)

func Open(path string) *sql.DB {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("db: open: %v", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if err := db.Ping(); err != nil {
		log.Fatalf("db: ping: %v", err)
	}
	return db
}

func RunMigrations(db *sql.DB) {
	// Ensure migrations tracking table exists
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		filename TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		log.Fatalf("db: create schema_migrations: %v", err)
	}

	migrations := []string{
		"migrations/001_schema.sql",
		"migrations/002_google_sso.sql",
		"migrations/003_add_password_hash.sql",
		"migrations/004_add_avatar.sql",
		"migrations/005_add_recurrence_date.sql",
		"migrations/006_add_location_fields.sql",
		"migrations/007_add_recurrence_end_date.sql",
		"migrations/008_add_time_image.sql",
		"migrations/009_add_password_resets.sql",
		"migrations/010_fix_invites.sql",
		"migrations/011_event_comments.sql",
		"migrations/012_strategic_initiatives.sql",
		"migrations/013_event_budget.sql",
		"migrations/014_event_checklist.sql",
		"migrations/015_event_team.sql",
		"migrations/016_event_attendance.sql",
		"migrations/017_initiative_updates.sql",
		"migrations/018_add_last_login.sql",
		"migrations/019_event_participants.sql",
		"migrations/020_registration_mode.sql",
	}

	for _, path := range migrations {
		// Skip already-applied migrations
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE filename = ?`, path).Scan(&count); err != nil {
			log.Fatalf("db: check migration %s: %v", path, err)
		}
		if count > 0 {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("db: read migration %s: %v", path, err)
		}

		stmts := strings.Split(string(data), ";")
		for _, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				log.Fatalf("db: exec migration %s: %v\nStatement: %s", path, err, stmt)
			}
		}

		if _, err := db.Exec(`INSERT INTO schema_migrations (filename) VALUES (?)`, path); err != nil {
			log.Fatalf("db: record migration %s: %v", path, err)
		}
		fmt.Printf("db: applied %s\n", path)
	}
}

// SeedAdmin creates the initial admin user (password-based) if none exists.
// Safe to call on every startup; no-op when an admin already exists.
func SeedAdmin(db *sql.DB, email, password string) {
	if email == "" || password == "" {
		log.Println("db: ADMIN_EMAIL or ADMIN_PASSWORD not set, skipping password seed")
		return
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, models.RoleAdmin).Scan(&count); err != nil {
		log.Fatalf("db: seed admin check: %v", err)
	}
	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		log.Fatalf("db: seed admin hash: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role) VALUES (?, ?, ?, ?, ?)`,
		uuid.New().String(), "Admin", email, string(hash), models.RoleAdmin,
	); err != nil {
		log.Fatalf("db: seed admin insert: %v", err)
	}
	fmt.Printf("db: admin user created: %s\n", email)
}
