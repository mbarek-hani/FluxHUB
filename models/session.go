package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Session struct {
	ID        string    `gorm:"type:varchar(36);primaryKey"`
	UserID    string    `gorm:"type:varchar(36);not null;index"`
	Token     string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
	
	User User `gorm:"foreignKey:UserID;references:ID"`
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}
