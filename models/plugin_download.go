package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PluginDownload struct {
	ID         string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	PluginID   string    `gorm:"type:varchar(36);not null;index" json:"plugin_id"`
	VersionTag string    `gorm:"type:varchar(100);not null" json:"version_tag"`
	CreatedAt  time.Time `json:"created_at"`
}

func (d *PluginDownload) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}
