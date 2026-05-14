package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PluginStatus string

const (
	StatusPending  PluginStatus = "pending"
	StatusApproved PluginStatus = "approved"
	StatusRejected PluginStatus = "rejected"
)

type Plugin struct {
	ID             string         `gorm:"type:varchar(36);primaryKey" json:"id"`
	Name           string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	RepoURL        string         `gorm:"type:varchar(512);not null" json:"repo_url"`
	DeveloperID    string         `gorm:"type:varchar(36);not null;index" json:"developer_id"`
	CurrentVersion string         `gorm:"type:varchar(50)" json:"current_version"`
	Status         PluginStatus   `gorm:"type:varchar(20);default:'pending';index" json:"status"`
	Description    string         `gorm:"type:text" json:"description"`
	Author         string         `gorm:"type:varchar(255)" json:"author"`
	Licence        string         `gorm:"type:varchar(100)" json:"licence"`
	ScanResult     string         `gorm:"type:text" json:"scan_result,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Versions  []Version  `gorm:"foreignKey:PluginID" json:"versions,omitempty"`
	Developer *User `gorm:"foreignKey:DeveloperID;references:ID" json:"developer,omitempty"`
}

func (p *Plugin) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

func (p *Plugin) IsApproved() bool {
	return p.Status == StatusApproved
}
