package dao

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func (d *dao) CreateEdgeUninstallTask(ctx context.Context, task *model.EdgeUninstallTask) (bool, error) {
	result := d.getDB().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(task)
	return result.RowsAffected == 1, result.Error
}
func (d *dao) GetEdgeUninstallTask(ctx context.Context, edge uint64, id string) (*model.EdgeUninstallTask, error) {
	var task model.EdgeUninstallTask
	q := d.getDB().WithContext(ctx).Where("edge_id = ?", edge)
	if id == "" {
		q = q.Where("active_edge = ?", edge)
	} else {
		q = q.Where("id = ?", id)
	}
	if err := q.First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}
func (d *dao) GetEdgeUninstallTaskByID(ctx context.Context, id string) (*model.EdgeUninstallTask, error) {
	var task model.EdgeUninstallTask
	if err := d.getDB().WithContext(ctx).Where("id = ?", id).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}
func (d *dao) SetEdgeUninstallResult(ctx context.Context, id, status, reason string) error {
	if status != "running" && status != "completed" && status != "failed" && status != "unknown" {
		return errors.New("invalid uninstall state")
	}
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.EdgeUninstallTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&task).Error; err != nil {
			return err
		}
		if task.Status == "completed" || task.Status == "failed" {
			if status == task.Status {
				return nil
			}
			return errors.New("uninstall task is terminal")
		}
		updates := map[string]any{"status": status, "reason": reason, "updated_at": time.Now()}
		if status == "completed" || status == "failed" {
			updates["active_edge"] = nil
		}
		if status == "completed" {
			if err := tx.Where("edge_id = ?", task.EdgeID).Delete(&model.AccessKey{}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Edge{}).Where("id = ?", task.EdgeID).Updates(map[string]any{"status": model.EdgeStatusStopped, "online": model.EdgeOnlineStatusOffline}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&task).Updates(updates).Error
	})
}
