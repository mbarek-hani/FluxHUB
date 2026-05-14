package services

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
)

type SessionStore struct {
	ttl time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		ttl: ttl,
	}
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *SessionStore) Create(userID string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(s.ttl)
	token := generateToken()

	session := models.Session{
		UserID:    userID,
		Token:     token,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}

	if err := database.DB.Create(&session).Error; err != nil {
		return "", err
	}

	return token, nil
}

func (s *SessionStore) Get(tokenString string) (*models.User, bool) {
	if tokenString == "" {
		return nil, false
	}

	var session models.Session
	if err := database.DB.Preload("User").Where("token = ? AND expires_at > ?", tokenString, time.Now()).First(&session).Error; err != nil {
		return nil, false
	}

	return &session.User, true
}

func (s *SessionStore) Destroy(tokenString string) {
	if tokenString == "" {
		return
	}
	database.DB.Where("token = ?", tokenString).Delete(&models.Session{})
}
