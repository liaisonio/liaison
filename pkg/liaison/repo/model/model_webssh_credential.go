package model

import (
	"time"

	"gorm.io/gorm"
)

// WebSSHCredential stores encrypted WebSSH password material for a proxy.
// Passwords are encrypted by the manager before persistence; plaintext never
// belongs in this model.
type WebSSHCredential struct {
	gorm.Model
	ProxyID           uint       `gorm:"column:proxy_id;type:int;not null;uniqueIndex:idx_webssh_credentials_proxy_user_username"`
	UserID            uint       `gorm:"column:user_id;type:int;not null;default:0;index;uniqueIndex:idx_webssh_credentials_proxy_user_username"`
	Username          string     `gorm:"column:username;type:varchar(255);not null;uniqueIndex:idx_webssh_credentials_proxy_user_username"`
	EncryptedPassword string     `gorm:"column:encrypted_password;type:text;not null"`
	Nonce             string     `gorm:"column:nonce;type:varchar(128);not null"`
	LastUsedAt        *time.Time `gorm:"column:last_used_at;type:datetime"`
}

func (WebSSHCredential) TableName() string {
	return "webssh_credentials"
}
