package model

import (
	"time"

	"gorm.io/gorm"
)

// ManagementAudit stores control-plane user operations without request bodies
// or secrets. Records are append-only from the application's perspective.
type ManagementAudit struct {
	ID         uint           `gorm:"column:id;primaryKey;autoIncrement"`
	CreatedAt  time.Time      `gorm:"column:created_at;autoCreateTime;index"`
	UpdatedAt  time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;index"`
	UserID     uint           `gorm:"column:user_id;type:int;not null;default:0;index"`
	UserEmail  string         `gorm:"column:user_email;type:varchar(255);not null;default:''"`
	Module     string         `gorm:"column:module;type:varchar(64);not null;default:'';index"`
	Action     string         `gorm:"column:action;type:varchar(64);not null;default:'';index"`
	Resource   string         `gorm:"column:resource;type:varchar(255);not null;default:''"`
	Method     string         `gorm:"column:method;type:varchar(16);not null;default:''"`
	ClientIP   string         `gorm:"column:client_ip;type:varchar(64);not null;default:''"`
	Success    bool           `gorm:"column:success;type:boolean;not null;index"`
	StatusCode int            `gorm:"column:status_code;type:int;not null;default:0"`
	ElapsedMS  int64          `gorm:"column:elapsed_ms;type:bigint;not null;default:0"`
}

func (ManagementAudit) TableName() string { return "management_audits" }
