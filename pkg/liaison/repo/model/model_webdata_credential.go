package model

import (
	"time"

	"gorm.io/gorm"
)

// WebDataCredential stores encrypted database/cache password material for a
// WebData entry. Passwords are encrypted by the manager before persistence.
type WebDataCredential struct {
	gorm.Model
	ProxyID           uint       `gorm:"column:proxy_id;type:int;not null;uniqueIndex:idx_webdata_credentials_identity"`
	UserID            uint       `gorm:"column:user_id;type:int;not null;index;uniqueIndex:idx_webdata_credentials_identity"`
	Name              string     `gorm:"column:name;type:varchar(255);not null;default:''"`
	Protocol          string     `gorm:"column:protocol;type:varchar(32);not null;uniqueIndex:idx_webdata_credentials_identity"`
	Username          string     `gorm:"column:username;type:varchar(255);not null;default:'';uniqueIndex:idx_webdata_credentials_identity"`
	Database          string     `gorm:"column:database_name;type:varchar(255);not null;default:'';uniqueIndex:idx_webdata_credentials_identity"`
	AuthDatabase      string     `gorm:"column:auth_database;type:varchar(255);not null;default:'';uniqueIndex:idx_webdata_credentials_identity"`
	RedisDB           int        `gorm:"column:redis_db;type:int;not null;default:0"`
	TLSMode           string     `gorm:"column:tls_mode;type:varchar(32);not null;default:''"`
	Schema            string     `gorm:"column:schema_name;type:varchar(255);not null;default:''"`
	AuthMechanism     string     `gorm:"column:auth_mechanism;type:varchar(64);not null;default:''"`
	DirectConnection  bool       `gorm:"column:direct_connection;type:boolean;not null;default:true"`
	ConnectionParams  string     `gorm:"column:connection_params;type:text;not null;default:''"`
	EncryptedPassword string     `gorm:"column:encrypted_password;type:text;not null"`
	Nonce             string     `gorm:"column:nonce;type:varchar(128);not null"`
	LastUsedAt        *time.Time `gorm:"column:last_used_at;type:datetime"`
}

func (WebDataCredential) TableName() string {
	return "webdata_credentials"
}
