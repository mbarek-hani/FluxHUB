package services

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type SessionKind string

const (
	SessionAdmin     SessionKind = "admin"
	SessionDeveloper SessionKind = "developer"
)

type Session struct {
	ID        string
	UserID    string
	Username  string
	Email     string
	FullName  string
	Kind      SessionKind
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
	go store.cleanup()
	return store
}

func (s *SessionStore) Create(userID, username, email, fullName string, kind SessionKind) (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	sessionID := hex.EncodeToString(token)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[sessionID] = &Session{
		ID:        sessionID,
		UserID:    userID,
		Username:  username,
		Email:     email,
		FullName:  fullName,
		Kind:      kind,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.ttl),
	}

	return sessionID, nil
}

func (s *SessionStore) Get(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if time.Now().After(sess.ExpiresAt) {
		delete(s.sessions, sessionID)
		return nil, false
	}
	return sess, true
}

func (s *SessionStore) Destroy(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, sess := range s.sessions {
			if now.After(sess.ExpiresAt) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}
