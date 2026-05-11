package dao

import (
	"errors"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

func (d *dao) GetWebSSHHostKeyByProxyID(proxyID uint) (*model.WebSSHHostKey, error) {
	var hostKey model.WebSSHHostKey
	if err := d.getDB().Where("proxy_id = ?", proxyID).First(&hostKey).Error; err != nil {
		return nil, err
	}
	return &hostKey, nil
}

func (d *dao) UpsertWebSSHHostKey(hostKey *model.WebSSHHostKey) error {
	if hostKey == nil {
		return errors.New("webssh host key is nil")
	}
	var existing model.WebSSHHostKey
	err := d.getDB().Where("proxy_id = ?", hostKey.ProxyID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return d.getDB().Create(hostKey).Error
		}
		return err
	}
	existing.Algorithm = hostKey.Algorithm
	existing.FingerprintSHA256 = hostKey.FingerprintSHA256
	existing.PublicKey = hostKey.PublicKey
	return d.getDB().Save(&existing).Error
}

func (d *dao) DeleteWebSSHHostKeyByProxyID(proxyID uint) error {
	return d.getDB().Where("proxy_id = ?", proxyID).Delete(&model.WebSSHHostKey{}).Error
}
