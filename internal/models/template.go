package models

import (
	"time"

	"gorm.io/gorm"
)

type Template struct {
	ID            string         `gorm:"primaryKey" json:"id"`
	Filename      string         `gorm:"not null" json:"filename"`
	OriginalName  string         `json:"original_name"`
	GCSPath       string         `gorm:"not null" json:"gcs_path"`
	FileSize      int64          `json:"file_size"`
	MimeType      string         `json:"mime_type"`
	Placeholders  string         `gorm:"type:json" json:"placeholders"` // JSON array of placeholder strings
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Documents []Document `gorm:"foreignKey:TemplateID" json:"documents,omitempty"`
}

type Document struct {
	ID         string         `gorm:"primaryKey" json:"id"`
	TemplateID string         `gorm:"not null;index" json:"template_id"`
	Filename   string         `gorm:"not null" json:"filename"`
	GCSPath    string         `gorm:"not null" json:"gcs_path"`
	FileSize   int64          `json:"file_size"`
	MimeType   string         `json:"mime_type"`
	Data       string         `gorm:"type:json" json:"data"` // JSON object of placeholder data used
	Status     string         `gorm:"default:'completed'" json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Template Template `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
}