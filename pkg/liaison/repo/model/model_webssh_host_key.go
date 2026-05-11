package model

import "gorm.io/gorm"

// WebSSHHostKey stores the trusted SSH host key fingerprint for a proxy.
// It is target identity metadata, not a user credential.
type WebSSHHostKey struct {
	gorm.Model
	ProxyID           uint   `gorm:"column:proxy_id;type:int;not null;uniqueIndex"`
	Algorithm         string `gorm:"column:algorithm;type:varchar(64);not null"`
	FingerprintSHA256 string `gorm:"column:fingerprint_sha256;type:varchar(128);not null"`
	PublicKey         string `gorm:"column:public_key;type:text;not null"`
}

func (WebSSHHostKey) TableName() string {
	return "webssh_host_keys"
}
