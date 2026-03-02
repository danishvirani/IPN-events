package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
)

const SessionCookieName = "session_id"

type SessionService struct {
	repo            *db.SessionRepository
	sessionDuration time.Duration
}

func NewSessionService(repo *db.SessionRepository, duration time.Duration) *SessionService {
	return &SessionService{repo: repo, sessionDuration: duration}
}

func (s *SessionService) Create(userID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(s.sessionDuration)
	if err := s.repo.Create(token, userID, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func (s *SessionService) GetUser(token string) (*models.User, error) {
	return s.repo.GetUser(token)
}

func (s *SessionService) Delete(token string) error {
	return s.repo.Delete(token)
}

func (s *SessionService) DeleteExpired() error {
	return s.repo.DeleteExpired()
}
