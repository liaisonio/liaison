package dao

import (
	"errors"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

func (d *dao) ListWebDataCredentialsByProxyAndUser(proxyID, userID uint, protocol string) ([]*model.WebDataCredential, error) {
	var credentials []*model.WebDataCredential
	err := d.getDB().
		Where("proxy_id = ? AND user_id = ? AND protocol = ?", proxyID, userID, protocol).
		Order("last_used_at DESC, id DESC").
		Find(&credentials).Error
	return credentials, err
}

func (d *dao) GetWebDataCredential(proxyID, userID uint, protocol, username, database, authDatabase string) (*model.WebDataCredential, error) {
	var credential model.WebDataCredential
	if err := d.getDB().
		Where("proxy_id = ? AND user_id = ? AND protocol = ? AND username = ? AND database_name = ? AND auth_database = ?",
			proxyID, userID, protocol, username, database, authDatabase).
		First(&credential).Error; err != nil {
		return nil, err
	}
	return &credential, nil
}

func (d *dao) GetWebDataCredentialByID(id, proxyID, userID uint) (*model.WebDataCredential, error) {
	var credential model.WebDataCredential
	if err := d.getDB().
		Where("id = ? AND proxy_id = ? AND user_id = ?", id, proxyID, userID).
		First(&credential).Error; err != nil {
		return nil, err
	}
	return &credential, nil
}

func (d *dao) UpsertWebDataCredential(credential *model.WebDataCredential) error {
	if credential == nil {
		return errors.New("webdata credential is nil")
	}
	var existing model.WebDataCredential
	err := d.getDB().
		Where("proxy_id = ? AND user_id = ? AND protocol = ? AND username = ? AND database_name = ? AND auth_database = ?",
			credential.ProxyID, credential.UserID, credential.Protocol, credential.Username, credential.Database, credential.AuthDatabase).
		First(&existing).Error
	if err == nil {
		now := time.Now()
		return d.getDB().Model(&existing).Updates(map[string]interface{}{
			"name":               credential.Name,
			"redis_db":           credential.RedisDB,
			"tls_mode":           credential.TLSMode,
			"schema_name":        credential.Schema,
			"auth_mechanism":     credential.AuthMechanism,
			"direct_connection":  credential.DirectConnection,
			"connection_params":  credential.ConnectionParams,
			"encrypted_password": credential.EncryptedPassword,
			"nonce":              credential.Nonce,
			"last_used_at":       &now,
		}).Error
	}
	return d.getDB().Create(credential).Error
}

func (d *dao) UpdateWebDataCredential(credential *model.WebDataCredential, updatePassword bool) error {
	if credential == nil || credential.ID == 0 {
		return errors.New("webdata credential is nil")
	}
	updates := map[string]interface{}{
		"name":              credential.Name,
		"protocol":          credential.Protocol,
		"username":          credential.Username,
		"database_name":     credential.Database,
		"auth_database":     credential.AuthDatabase,
		"redis_db":          credential.RedisDB,
		"tls_mode":          credential.TLSMode,
		"schema_name":       credential.Schema,
		"auth_mechanism":    credential.AuthMechanism,
		"direct_connection": credential.DirectConnection,
		"connection_params": credential.ConnectionParams,
	}
	if updatePassword {
		updates["encrypted_password"] = credential.EncryptedPassword
		updates["nonce"] = credential.Nonce
	}
	return d.getDB().Model(&model.WebDataCredential{}).
		Where("id = ? AND proxy_id = ? AND user_id = ?", credential.ID, credential.ProxyID, credential.UserID).
		Updates(updates).Error
}

func (d *dao) TouchWebDataCredential(proxyID, userID uint, protocol, username, database, authDatabase string) error {
	now := time.Now()
	return d.getDB().Model(&model.WebDataCredential{}).
		Where("proxy_id = ? AND user_id = ? AND protocol = ? AND username = ? AND database_name = ? AND auth_database = ?",
			proxyID, userID, protocol, username, database, authDatabase).
		Update("last_used_at", &now).Error
}

func (d *dao) TouchWebDataCredentialByID(id, proxyID, userID uint) error {
	now := time.Now()
	return d.getDB().Model(&model.WebDataCredential{}).
		Where("id = ? AND proxy_id = ? AND user_id = ?", id, proxyID, userID).
		Update("last_used_at", &now).Error
}

func (d *dao) DeleteWebDataCredential(proxyID, userID uint, protocol, username, database, authDatabase string) error {
	return d.getDB().Unscoped().
		Where("proxy_id = ? AND user_id = ? AND protocol = ? AND username = ? AND database_name = ? AND auth_database = ?",
			proxyID, userID, protocol, username, database, authDatabase).
		Delete(&model.WebDataCredential{}).Error
}

func (d *dao) DeleteWebDataCredentialByID(id, proxyID, userID uint) error {
	return d.getDB().Unscoped().
		Where("id = ? AND proxy_id = ? AND user_id = ?", id, proxyID, userID).
		Delete(&model.WebDataCredential{}).Error
}

func (d *dao) DeleteWebDataCredentialByProxyID(proxyID uint) error {
	return d.getDB().Unscoped().Where("proxy_id = ?", proxyID).Delete(&model.WebDataCredential{}).Error
}

func (d *dao) CreateWebDataAudit(audit *model.WebDataAudit) error {
	if audit == nil {
		return errors.New("webdata audit is nil")
	}
	return d.getDB().Create(audit).Error
}

func (d *dao) ListWebDataAudits(query *ListWebDataAuditsQuery) ([]*model.WebDataAudit, error) {
	var audits []*model.WebDataAudit
	db := d.applyWebDataAuditFilters(d.getDB().Model(&model.WebDataAudit{}), query)
	limit := 100
	if query != nil {
		if query.Limit > 0 {
			limit = query.Limit
		}
		if query.Offset > 0 {
			db = db.Offset(query.Offset)
		}
	}
	if limit > 500 {
		limit = 500
	}
	db = db.Limit(limit)
	err := db.Order("created_at DESC, id DESC").Find(&audits).Error
	return audits, err
}

func (d *dao) CountWebDataAudits(query *ListWebDataAuditsQuery) (int64, error) {
	var count int64
	db := d.applyWebDataAuditFilters(d.getDB().Model(&model.WebDataAudit{}), query)
	if err := db.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (d *dao) applyWebDataAuditFilters(db *gorm.DB, query *ListWebDataAuditsQuery) *gorm.DB {
	if query == nil {
		return db
	}
	if query.UserID > 0 {
		db = db.Where("user_id = ?", query.UserID)
	}
	if query.ProxyID > 0 {
		db = db.Where("proxy_id = ?", query.ProxyID)
	}
	if query.Protocol != "" {
		db = db.Where("protocol = ?", query.Protocol)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.Success != nil {
		db = db.Where("success = ?", *query.Success)
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", *query.EndTime)
	}
	if query.Keyword != "" {
		keyword := "%" + query.Keyword + "%"
		db = db.Where(
			"statement_preview LIKE ? OR statement_sha256 LIKE ? OR error LIKE ? OR details LIKE ? OR user_email LIKE ? OR proxy_name LIKE ? OR application_name LIKE ?",
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
		)
	}
	return db
}
