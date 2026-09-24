package model

import "time"

type EdgeAgentHistory struct {
	OwnerID       uint   `gorm:"primaryKey;autoIncrement:false;index:idx_edge_agent_history_scope,priority:1"`
	AccessID      string `gorm:"primaryKey;size:32;index:idx_edge_agent_history_scope,priority:2"`
	EdgeID        uint64 `gorm:"primaryKey;autoIncrement:false;index:idx_edge_agent_history_scope,priority:3"`
	SessionID     string `gorm:"primaryKey;size:32"`
	Revision      uint64 `gorm:"not null;default:0"`
	Payload       []byte
	TitleOverride []byte
	Closed        bool `gorm:"not null;default:false;index:idx_edge_agent_history_sync,priority:2"`
	Deleted       bool `gorm:"not null;default:false;index:idx_edge_agent_history_scope,priority:4;index:idx_edge_agent_history_sync,priority:1"`
	CreatedAt     time.Time
	UpdatedAt     time.Time `gorm:"index:idx_edge_agent_history_scope,priority:5"`
	SyncedAt      time.Time `gorm:"index:idx_edge_agent_history_sync,priority:3"`
}
