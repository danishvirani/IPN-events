package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DBPath             string
	SessionDuration    time.Duration
	AdminEmail         string
	AdminPassword      string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleCallbackURL  string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("config: no .env file found, using environment variables")
	}

	hours := 72
	if h := os.Getenv("SESSION_DURATION_HOURS"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil {
			hours = parsed
		}
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./ipn-events.db"
	}

	callbackURL := os.Getenv("GOOGLE_CALLBACK_URL")
	if callbackURL == "" {
		callbackURL = "http://localhost:8080/auth/callback"
	}

	return &Config{
		Port:               port,
		DBPath:             dbPath,
		SessionDuration:    time.Duration(hours) * time.Hour,
		AdminEmail:         os.Getenv("ADMIN_EMAIL"),
		AdminPassword:      os.Getenv("ADMIN_PASSWORD"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleCallbackURL:  callbackURL,
	}
}
