package dao

import (
	"errors"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

func (d *dao) ListWebSSHCredentialsByProxyAndUser(proxyID, userID uint) ([]*model.WebSSHCredential, error) {
	var credentials []*model.WebSSHCredential
	if err := d.getDB().
		Where("proxy_id = ? AND user_id = ?", proxyID, userID).
		Order("last_used_at IS NULL ASC").
		Order("last_used_at DESC").
		Order("updated_at DESC").
		Find(&credentials).Error; err != nil {
		return nil, err
	}
	return credentials, nil
}

func (d *dao) GetWebSSHCredential(proxyID, userID uint, username string) (*model.WebSSHCredential, error) {
	var credential model.WebSSHCredential
	if err := d.getDB().
		Where("proxy_id = ? AND user_id = ? AND username = ?", proxyID, userID, username).
		First(&credential).Error; err != nil {
		return nil, err
	}
	return &credential, nil
}

func (d *dao) UpsertWebSSHCredential(credential *model.WebSSHCredential) error {
	if credential == nil {
		return errors.New("webssh credential is nil")
	}
	var existing model.WebSSHCredential
	err := d.getDB().
		Where("proxy_id = ? AND user_id = ? AND username = ?", credential.ProxyID, credential.UserID, credential.Username).
		First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return d.getDB().Create(credential).Error
		}
		return err
	}
	existing.Username = credential.Username
	existing.EncryptedPassword = credential.EncryptedPassword
	existing.Nonce = credential.Nonce
	return d.getDB().Save(&existing).Error
}

func (d *dao) TouchWebSSHCredential(proxyID, userID uint, username string) error {
	now := time.Now()
	return d.getDB().Model(&model.WebSSHCredential{}).
		Where("proxy_id = ? AND user_id = ? AND username = ?", proxyID, userID, username).
		Update("last_used_at", &now).Error
}

func (d *dao) DeleteWebSSHCredential(proxyID, userID uint, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("SSH 用户名不能为空")
	}
	return d.getDB().Unscoped().
		Where("proxy_id = ? AND user_id = ? AND username = ?", proxyID, userID, username).
		Delete(&model.WebSSHCredential{}).Error
}

func (d *dao) DeleteWebSSHCredentialByProxyID(proxyID uint) error {
	return d.getDB().Unscoped().Where("proxy_id = ?", proxyID).Delete(&model.WebSSHCredential{}).Error
}

func (d *dao) migrateWebSSHCredentials() error {
	if d.db.Migrator().HasIndex(&model.WebSSHCredential{}, "idx_webssh_credentials_proxy_id") {
		if err := d.db.Migrator().DropIndex(&model.WebSSHCredential{}, "idx_webssh_credentials_proxy_id"); err != nil {
			return err
		}
	}
	return d.db.Unscoped().Where("user_id = ?", 0).Delete(&model.WebSSHCredential{}).Error
}
