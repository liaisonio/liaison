package model

import "gorm.io/gorm"

type ProxyStatus int

type AccessProtocol string

const (
	ProxyStatusRunning ProxyStatus = iota + 1
	ProxyStatusStopped
)

const (
	AccessProtocolTCP        AccessProtocol = "tcp"
	AccessProtocolAI        AccessProtocol = "aiapi"
	AccessProtocolHTTP       AccessProtocol = "http"
	AccessProtocolSSH        AccessProtocol = "ssh"
	AccessProtocolRDP        AccessProtocol = "rdp"
	AccessProtocolVNC        AccessProtocol = "vnc"
	AccessProtocolMySQL      AccessProtocol = "mysql"
	AccessProtocolPostgreSQL AccessProtocol = "postgresql"
	AccessProtocolRedis      AccessProtocol = "redis"
	AccessProtocolMongoDB    AccessProtocol = "mongodb"
	AccessProtocolWebSSH     AccessProtocol = "webssh"
	AccessProtocolWeb        AccessProtocol = "web"
)

type Proxy struct {
	gorm.Model
	ApplicationID  uint           `gorm:"column:application_id;type:int;not null"`
	Name           string         `gorm:"column:name;type:varchar(255);not null"`
	Port           int            `gorm:"column:port;type:int;not null"`
	Status         ProxyStatus    `gorm:"column:status;type:int;not null"`
	Description    string         `gorm:"column:description;type:varchar(255);not null"`
	AccessProtocol AccessProtocol `gorm:"column:access_protocol;type:varchar(32);not null;default:'';index"`
	// 以下用于中间使用
	Application *Application `gorm:"-"`
	Device      *Device      `gorm:"-"`
}

func (Proxy) TableName() string {
	return "proxies"
}
