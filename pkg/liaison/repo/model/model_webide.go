package model

import "time"

// WebIDEApplication records an installation or an externally managed service.
// Unlike network applications a managed installation has no permanent TCP port.
type WebIDEApplication struct {
	ID             string    `gorm:"primaryKey;size:32" json:"id"`
	OwnerID        uint      `gorm:"not null;index;uniqueIndex:idx_webide_installation" json:"-"`
	EdgeID         uint64    `gorm:"not null;uniqueIndex:idx_webide_installation" json:"edge_id"`
	InstallationID string    `gorm:"not null;size:64;uniqueIndex:idx_webide_installation" json:"installation_id"`
	Name           string    `gorm:"not null;size:120" json:"name"`
	Mode           string    `gorm:"not null;size:16" json:"mode"`
	Port           int       `gorm:"not null" json:"port,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (WebIDEApplication) TableName() string { return "webide_applications" }

type WebIDEAccess struct {
	ID            string    `gorm:"primaryKey;size:32" json:"id"`
	OwnerID       uint      `gorm:"not null;index" json:"-"`
	ApplicationID string    `gorm:"not null;size:32;index" json:"application_id"`
	Name          string    `gorm:"not null;size:120" json:"name"`
	Enabled       bool      `gorm:"not null" json:"enabled"`
	Project       string    `gorm:"not null;size:4096" json:"project"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (WebIDEAccess) TableName() string { return "webide_accesses" }
