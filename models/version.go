package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Version représente une version taguée d'un plugin
type Version struct {
	ID         string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	PluginID   string    `gorm:"type:varchar(255);not null;index" json:"plugin_id"`
	Tag        string    `gorm:"type:varchar(100);not null" json:"tag"`
	Signature  string    `gorm:"type:text" json:"signature"` // Signature RSA base64
	Changelog  string    `gorm:"type:text" json:"changelog"`
	ZipPath    string    `gorm:"type:varchar(512)" json:"-"` // Chemin interne du ZIP
	SHA256Hash string    `gorm:"type:varchar(64)" json:"sha256_hash"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relation
	Plugin Plugin `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
}

// BeforeCreate génère un UUID avant l'insertion
func (v *Version) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	return nil
}

// ScanReport représente le résultat de l'analyse statique
type ScanReport struct {
	HasDangerousCode bool          `json:"has_dangerous_code"`
	Findings         []ScanFinding `json:"findings"`
	ScannedFiles     int           `json:"scanned_files"`
	TotalIssues      int           `json:"total_issues"`
}

// ScanFinding représente une découverte lors de l'analyse statique
type ScanFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Pattern  string `json:"pattern"`
	Severity string `json:"severity"` // critical, warning, info
	Context  string `json:"context"`  // Extrait de code pour contexte
}
