package dao

import (
	"context"
	"errors"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *dao) GetAIApplication(ctx context.Context, id uint) (*model.AIApplication, error) {
	var v model.AIApplication
	err := d.getDB().WithContext(ctx).First(&v, "application_id = ?", id).Error
	return &v, err
}
func (d *dao) SaveAIApplication(ctx context.Context, v *model.AIApplication) error {
	return d.getDB().WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(v).Error
}
func (d *dao) GetAIAccess(ctx context.Context, id uint) (*model.AIAccess, error) {
	var v model.AIAccess
	err := d.getDB().WithContext(ctx).First(&v, "proxy_id = ?", id).Error
	return &v, err
}
func (d *dao) SaveAIAccess(ctx context.Context, v *model.AIAccess) error {
	return d.getDB().WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(v).Error
}
func (d *dao) CreateAIKey(ctx context.Context, v *model.AIKey) error {
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.AIKey{}).Where("proxy_id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?", v.ProxyID, v.UserID, time.Now()).Count(&count).Error; err != nil {
			return err
		}
		if count >= 100 {
			return errors.New("AI API key limit reached")
		}
		return tx.Create(v).Error
	})
}
func (d *dao) GetAIKey(ctx context.Context, digest string) (*model.AIKey, error) {
	var v model.AIKey
	err := d.getDB().WithContext(ctx).Where("digest = ? AND revoked_at IS NULL AND expires_at > ?", digest, time.Now()).First(&v).Error
	return &v, err
}
func (d *dao) ListAIKeys(ctx context.Context, proxy, user uint) ([]model.AIKey, error) {
	var v []model.AIKey
	err := d.getDB().WithContext(ctx).Where("proxy_id = ? AND user_id = ? AND revoked_at IS NULL", proxy, user).Order("id DESC").Limit(100).Find(&v).Error
	return v, err
}
func (d *dao) RevokeAIKey(ctx context.Context, proxy, user, id uint) error {
	return d.getDB().WithContext(ctx).Model(&model.AIKey{}).Where("proxy_id = ? AND user_id = ? AND id = ?", proxy, user, id).Update("revoked_at", time.Now()).Error
}
func (d *dao) RecordAIRequest(ctx context.Context, v *model.AIRequest) error {
	if v == nil || v.RequestID == "" || v.UserID == 0 || v.ProxyID == 0 {
		return errors.New("invalid LLM usage attribution")
	}
	if (v.InputTokens != nil && *v.InputTokens < 0) || (v.OutputTokens != nil && *v.OutputTokens < 0) {
		return errors.New("invalid LLM token count")
	}
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if v.CreatedAt.IsZero() {
			v.CreatedAt = time.Now().UTC()
		}
		v.CreatedAt = v.CreatedAt.UTC()
		usage := model.LLMTokenUsage{RequestID: v.RequestID, ProxyID: v.ProxyID, UserID: v.UserID, KeyID: v.KeyID, Model: v.Model, InputTokens: v.InputTokens, OutputTokens: v.OutputTokens, Complete: v.Complete, CreatedAt: v.CreatedAt}
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "request_id"}}, DoNothing: true}).Create(&usage)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		} // Retry must not double-count.
		if err := tx.Create(v).Error; err != nil {
			return err
		}
		// Bounded retention per user/access, no prompt/response contents.
		return tx.Exec("DELETE FROM ai_requests WHERE proxy_id = ? AND user_id = ? AND id NOT IN (SELECT id FROM ai_requests WHERE proxy_id = ? AND user_id = ? ORDER BY id DESC LIMIT 500)", v.ProxyID, v.UserID, v.ProxyID, v.UserID).Error
	})
}

func (d *dao) GetLLMTokenUsage(ctx context.Context, proxy, user uint, since time.Time) (*model.LLMTokenUsageReport, error) {
	if proxy == 0 || user == 0 || since.IsZero() {
		return nil, errors.New("invalid LLM usage scope")
	}
	since = since.UTC()
	report := &model.LLMTokenUsageReport{Since: since, Records: []model.LLMTokenUsage{}}
	err := d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := func() *gorm.DB {
			return tx.Model(&model.LLMTokenUsage{}).Where("proxy_id = ? AND user_id = ? AND created_at >= ?", proxy, user, since)
		}
		if err := query().Select("COUNT(*) AS requests, COALESCE(SUM(CASE WHEN input_tokens IS NULL OR output_tokens IS NULL THEN 1 ELSE 0 END), 0) AS unknown_requests, SUM(input_tokens) AS input_tokens, SUM(output_tokens) AS output_tokens").Scan(&report.Summary).Error; err != nil {
			return err
		}
		return query().Order("created_at DESC, id DESC").Limit(100).Find(&report.Records).Error
	})
	return report, err
}
func (d *dao) ListAIRequests(ctx context.Context, proxy, user uint, limit int) ([]model.AIRequest, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	var v []model.AIRequest
	err := d.getDB().WithContext(ctx).Where("proxy_id = ? AND user_id = ?", proxy, user).Order("id DESC").Limit(limit).Find(&v).Error
	return v, err
}
