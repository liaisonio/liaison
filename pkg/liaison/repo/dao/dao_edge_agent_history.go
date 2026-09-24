package dao

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func historyScope(db *gorm.DB, r *model.EdgeAgentHistory) *gorm.DB {
	return db.Model(&model.EdgeAgentHistory{}).Where("owner_id = ? AND access_id = ? AND edge_id = ? AND session_id = ?", r.OwnerID, r.AccessID, r.EdgeID, r.SessionID)
}
func (d *dao) SaveEdgeAgentHistory(ctx context.Context, r *model.EdgeAgentHistory) error {
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entry model.AgentAccess
		if err := tx.Where("owner_id = ? AND id = ? AND edge_id = ?", r.OwnerID, r.AccessID, r.EdgeID).First(&entry).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(r).Error; err != nil {
			return err
		}
		return historyScope(tx, r).Where("deleted = ? AND revision < ?", false, r.Revision).Updates(map[string]any{"payload": r.Payload, "revision": r.Revision, "closed": r.Closed, "updated_at": r.UpdatedAt, "synced_at": r.SyncedAt}).Error
	})
}
func (d *dao) GetEdgeAgentHistory(ctx context.Context, scope *model.EdgeAgentHistory) (*model.EdgeAgentHistory, error) {
	var row model.EdgeAgentHistory
	err := historyScope(d.getDB().WithContext(ctx), scope).First(&row).Error
	return &row, err
}
func (d *dao) ListEdgeAgentHistories(ctx context.Context, scope *model.EdgeAgentHistory, page int) ([]model.EdgeAgentHistory, int64, error) {
	rows := []model.EdgeAgentHistory{}
	var total int64
	q := d.getDB().WithContext(ctx).Model(&model.EdgeAgentHistory{}).Where("owner_id = ? AND access_id = ? AND edge_id = ? AND deleted = ?", scope.OwnerID, scope.AccessID, scope.EdgeID, false)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("updated_at DESC, session_id DESC").Offset((page - 1) * 50).Limit(50).Find(&rows).Error
	return rows, total, err
}
func (d *dao) MutateEdgeAgentHistory(ctx context.Context, r *model.EdgeAgentHistory, remove bool) error {
	if remove {
		return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			result := historyScope(tx, r).Updates(map[string]any{"deleted": true, "closed": true, "payload": nil, "title_override": nil})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
			return tx.Where("owner_id = ? AND access_id = ? AND edge_id = ? AND session_id = ?", r.OwnerID, r.AccessID, r.EdgeID, r.SessionID).Delete(&model.EdgeAgentHistoryPage{}).Error
		})
	}
	values := map[string]any{"title_override": r.TitleOverride, "updated_at": time.Now()}
	q := historyScope(d.getDB().WithContext(ctx), r).Where("deleted = ?", false)
	result := q.Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (d *dao) PendingEdgeAgentHistories(ctx context.Context) ([]model.EdgeAgentHistory, error) {
	var rows []model.EdgeAgentHistory
	err := d.getDB().WithContext(ctx).Where("deleted = ? AND closed = ?", false, false).Order("synced_at ASC").Limit(50).Find(&rows).Error
	return rows, err
}
func (d *dao) TouchEdgeAgentHistory(ctx context.Context, r *model.EdgeAgentHistory, gone bool) error {
	values := map[string]any{"synced_at": time.Now()}
	if gone {
		values["closed"] = true
	}
	return historyScope(d.getDB().WithContext(ctx), r).Updates(values).Error
}
