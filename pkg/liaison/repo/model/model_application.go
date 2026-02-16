package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type ApplicationType string

const (
	ApplicationTypeTCP  ApplicationType = "tcp"  // TCP 应用
	ApplicationTypeHTTP ApplicationType = "http" // HTTP 应用
)

// UintSlice 为 []uint 实现 Scanner 和 Valuer 接口，用于 SQLite JSON 字段处理
type UintSlice []uint

// Scan 实现 Scanner 接口，用于从数据库读取数据
func (u *UintSlice) Scan(value interface{}) error {
	if value == nil {
		*u = UintSlice{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, u)
	case string:
		return json.Unmarshal([]byte(v), u)
	default:
		// 如果数据库返回的不是 JSON 格式，返回空切片
		*u = UintSlice{}
		return nil
	}
}

// Value 实现 Valuer 接口，用于向数据库写入数据
func (u UintSlice) Value() (driver.Value, error) {
	if len(u) == 0 {
		return "[]", nil
	}
	return json.Marshal(u)
}

// Application is a software running on a device that edge-proxy can proxy to.
type Application struct {
	gorm.Model
	EdgeIDs         UintSlice       `gorm:"column:edge_ids;type:json;not null"` // 关联的edge id
	DeviceID        uint            `gorm:"column:device_id;type:int;not null"`
	Name            string          `gorm:"column:name;type:varchar(255);not null"`
	Description     string          `gorm:"column:description;type:varchar(255);default:''"` // 描述
	IP              string          `gorm:"column:ip;type:varchar(255);not null"`
	Port            int             `gorm:"column:port;type:int;not null"`
	HeartbeatAt     time.Time       `gorm:"column:heartbeat_at;type:datetime;not null"`
	ApplicationType ApplicationType `gorm:"column:application_type;type:varchar(255);not null"`
	// 以下用于中间使用
	Device *Device `gorm:"-"`
	Proxy  *Proxy  `gorm:"-"`
}

func (Application) TableName() string {
	return "applications"
}
