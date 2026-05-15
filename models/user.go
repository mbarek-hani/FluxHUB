package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
)

type User struct {
	ID        string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	Role      Role      `gorm:"type:varchar(20);not null;default:'developer'" json:"role"`
	Username  string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"type:varchar(255);uniqueIndex" json:"email"`
	Password  string    `gorm:"type:varchar(255)" json:"-"` // Can be empty for GitHub users
	GithubID  string    `gorm:"type:varchar(255);uniqueIndex" json:"github_id"`
	FullName  string    `gorm:"type:varchar(255)" json:"full_name"`
	Company   string    `gorm:"type:varchar(255)" json:"company"`
	Website   string    `gorm:"type:varchar(512)" json:"website"`
	Bio       string    `gorm:"type:text" json:"bio"`
	Verified  bool      `gorm:"default:false" json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations (only for developers)
	Plugins []Plugin `gorm:"foreignKey:DeveloperID;references:ID" json:"plugins,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

func (u *User) SetPassword(plain string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashed)
	return nil
}

func (u *User) CheckPassword(plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plain)) == nil
}

func (u *User) AvatarLetter() string {
	if len(u.FullName) > 0 {
		return string(u.FullName[0])
	}
	if len(u.Username) > 0 {
		return string(u.Username[0])
	}
	return "?"
}
