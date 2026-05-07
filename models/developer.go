package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Developer struct {
	ID        string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	Username  string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"type:varchar(255);not null" json:"-"`
	FullName  string    `gorm:"type:varchar(255)" json:"full_name"`
	Company   string    `gorm:"type:varchar(255)" json:"company"`
	Website   string    `gorm:"type:varchar(512)" json:"website"`
	Bio       string    `gorm:"type:text" json:"bio"`
	Verified  bool      `gorm:"default:false" json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Plugins []Plugin `gorm:"foreignKey:DeveloperID;references:ID" json:"plugins,omitempty"`
}

func (d *Developer) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}

func (d *Developer) SetPassword(plain string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	d.Password = string(hashed)
	return nil
}

func (d *Developer) CheckPassword(plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(d.Password), []byte(plain)) == nil
}

func (d *Developer) AvatarLetter() string {
	if len(d.FullName) > 0 {
		return string(d.FullName[0])
	}
	if len(d.Username) > 0 {
		return string(d.Username[0])
	}
	return "?"
}
