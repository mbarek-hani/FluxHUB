package services

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type SessionKind string

const (
	SessionAdmin     SessionKind = "admin"
	SessionDeveloper SessionKind = "developer"
)

var jwtSecret = []byte("super-secret-flux-key-change-me-in-production")

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
	ttl time.Duration
}

type sessionClaims struct {
	UserID   string      `json:"uid"`
	Username string      `json:"uname"`
	Email    string      `json:"eml,omitempty"`
	FullName string      `json:"fname,omitempty"`
	Kind     SessionKind `json:"kind"`
	jwt.RegisteredClaims
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		ttl: ttl,
	}
}

func (s *SessionStore) Create(userID, username, email, fullName string, kind SessionKind) (string, error) {
	now := time.Now()
	expiresAt := now.Add(s.ttl)

	claims := sessionClaims{
		UserID:   userID,
		Username: username,
		Email:    email,
		FullName: fullName,
		Kind:     kind,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func (s *SessionStore) Get(tokenString string) (*Session, bool) {
	if tokenString == "" {
		return nil, false
	}

	token, err := jwt.ParseWithClaims(tokenString, &sessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, false
	}

	claims, ok := token.Claims.(*sessionClaims)
	if !ok {
		return nil, false
	}

	return &Session{
		ID:        tokenString, // Just use the token as the ID
		UserID:    claims.UserID,
		Username:  claims.Username,
		Email:     claims.Email,
		FullName:  claims.FullName,
		Kind:      claims.Kind,
		CreatedAt: claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
	}, true
}

func (s *SessionStore) Destroy(sessionID string) {
	// Stateless JWT, no server-side destroy needed.
	// The client removes the cookie.
}
