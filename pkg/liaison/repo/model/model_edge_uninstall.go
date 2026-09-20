package model

import "time"

// Terminal tasks are retained as operation history. ActiveEdge is a nullable
// uniqueness lock: unknown outcomes remain locked rather than being reissued.
type EdgeUninstallTask struct {
	ID         string    `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	EdgeID     uint64    `gorm:"column:edge_id;index;not null" json:"edge_id"`
	ActiveEdge *uint64   `gorm:"column:active_edge;uniqueIndex" json:"-"`
	CreatedBy  uint      `gorm:"column:created_by;not null" json:"created_by"`
	InstanceID string    `gorm:"column:instance_id;type:varchar(80);not null" json:"instance_id"`
	Status     string    `gorm:"column:status;type:varchar(32);not null" json:"status"`
	Reason     string    `gorm:"column:reason;type:varchar(80);not null;default:''" json:"reason"`
	TokenHash  string    `gorm:"column:token_hash;type:varchar(64);not null" json:"-"`
	ExpiresAt  time.Time `gorm:"column:expires_at;not null" json:"expires_at"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (EdgeUninstallTask) TableName() string { return "edge_uninstall_tasks" }
