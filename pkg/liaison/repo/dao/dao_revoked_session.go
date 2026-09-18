package dao

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func (d *dao) RevokeSession(ctx context.Context, hash string, expires time.Time) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", time.Now()).Delete(&model.RevokedSession{}).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.RevokedSession{TokenHash: hash, ExpiresAt: expires}).Error
	})
}

func (d *dao) IsSessionRevoked(ctx context.Context, hash string) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.RevokedSession{}).Where("token_hash = ?", hash).Count(&count).Error
	return count > 0, err
}
