package model

import (
	"time"

	"gorm.io/gorm"
)

// WebDataAudit stores access audit records without result data or secrets.
type WebDataAudit struct {
	ID               uint `gorm:"primarykey;autoIncrement"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
	UserID           uint           `gorm:"column:user_id;type:int;not null;index"`
	UserEmail        string         `gorm:"column:user_email;type:varchar(255);not null;default:'';index"`
	ProxyID          uint           `gorm:"column:proxy_id;type:int;not null;index"`
	ProxyName        string         `gorm:"column:proxy_name;type:varchar(255);not null;default:''"`
	ApplicationID    uint           `gorm:"column:application_id;type:int;not null;index"`
	ApplicationName  string         `gorm:"column:application_name;type:varchar(255);not null;default:''"`
	Protocol         string         `gorm:"column:protocol;type:varchar(32);not null;index"`
	Action           string         `gorm:"column:action;type:varchar(64);not null;default:'execute';index"`
	StatementPreview string         `gorm:"column:statement_preview;type:text;not null"`
	StatementSHA256  string         `gorm:"column:statement_sha256;type:varchar(64);not null;index"`
	Success          bool           `gorm:"column:success;type:boolean;not null;index"`
	Error            string         `gorm:"column:error;type:text;not null;default:''"`
	ElapsedMS        int64          `gorm:"column:elapsed_ms;type:bigint;not null;default:0"`
	Details          string         `gorm:"column:details;type:text;not null;default:'{}'"`
}

func (WebDataAudit) TableName() string {
	return "access_audits"
}
