package model

import "time"

// AgentApplication registers an existing installation, not a running process.
type AgentApplication struct {
	AccessCount    int64     `gorm:"->;-:migration" json:"access_count"`
	ID             string    `gorm:"primaryKey;size:32" json:"id"`
	OwnerID        uint      `gorm:"not null;uniqueIndex:idx_agent_installation" json:"-"`
	EdgeID         uint64    `gorm:"not null;uniqueIndex:idx_agent_installation" json:"edge_id"`
	Kind           string    `gorm:"not null;size:32;uniqueIndex:idx_agent_installation" json:"kind"`
	InstallationID string    `gorm:"not null;size:32;uniqueIndex:idx_agent_installation" json:"installation_id"`
	Name           string    `gorm:"not null;size:120" json:"name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (AgentApplication) TableName() string { return "agent_applications" }
