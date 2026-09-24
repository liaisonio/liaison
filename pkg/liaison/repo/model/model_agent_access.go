package model

import "time"

type AgentAccessFilter struct {
	Name string
	Kind string
}

// AgentAccess stores an owner's reusable entry, not a running Agent session.
type AgentAccess struct {
	ID             string    `gorm:"primaryKey;size:32" json:"id"`
	OwnerID        uint      `gorm:"not null;index" json:"-"`
	Name           string    `gorm:"not null;size:120" json:"name"`
	Kind           string    `gorm:"not null;size:32" json:"kind"`
	EdgeID         uint64    `gorm:"not null;index" json:"edge_id"`
	InstallationID string    `gorm:"not null;size:32" json:"installation_id"`
	Project        string    `gorm:"not null" json:"project"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	SessionCount   int64     `gorm:"->;-:migration" json:"session_count"`
}
