package dao

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *dao) ReadAgentModelConfig(ctx context.Context) ([]byte, error) {
	var row model.AgentModelSetting
	err := d.getDB().WithContext(ctx).First(&row, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return row.Payload, err
}
func (d *dao) WriteAgentModelConfig(ctx context.Context, payload []byte) error {
	return d.getDB().WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&model.AgentModelSetting{ID: 1, Payload: payload}).Error
}
