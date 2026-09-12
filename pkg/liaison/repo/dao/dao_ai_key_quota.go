package dao

import (
	"context"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

func (d *dao) GetAIKeyUsage(ctx context.Context, proxy, user, key uint) (*model.AIKeyUsage, error) {
	var result model.AIKeyUsage
	// One statement observes the limit and all settled usage together. No 30-day
	// filter: this is a lifetime threshold, independent of request-log retention.
	tx := d.getDB().WithContext(ctx).Table("ai_keys AS k").
		Select("k.id AS key_id, k.token_limit, COALESCE(SUM(COALESCE(u.input_tokens, 0) + COALESCE(u.output_tokens, 0)), 0) AS used_tokens, COALESCE(SUM(CASE WHEN u.id IS NOT NULL AND (u.input_tokens IS NULL OR u.output_tokens IS NULL OR u.complete = 0) THEN 1 ELSE 0 END), 0) AS unknown_requests").
		Joins("LEFT JOIN llm_token_usage AS u ON u.key_id = k.id AND u.proxy_id = k.proxy_id AND u.user_id = k.user_id").
		Where("k.id = ? AND k.proxy_id = ? AND k.user_id = ? AND k.revoked_at IS NULL", key, proxy, user).
		Group("k.id, k.token_limit").Scan(&result)
	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &result, nil
}

func (d *dao) UpdateAIKeyQuota(ctx context.Context, proxy, user, key uint, limit *int64) error {
	tx := d.getDB().WithContext(ctx).Model(&model.AIKey{}).
		Where("id = ? AND proxy_id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?", key, proxy, user, time.Now()).Update("token_limit", limit)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
