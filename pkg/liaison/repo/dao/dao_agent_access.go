package dao

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"strings"
)

func (d *dao) ListAgentAccesses(ctx context.Context, owner uint, page, size int, filters ...model.AgentAccessFilter) ([]model.AgentAccess, int64, error) {
	rows := []model.AgentAccess{}
	var total int64
	q := d.getDB().WithContext(ctx).Model(&model.AgentAccess{}).Where("owner_id = ?", owner)
	if len(filters) > 0 {
		if name := strings.TrimSpace(filters[0].Name); name != "" {
			pattern := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(name))
			q = q.Where("LOWER(name) LIKE ? ESCAPE '!'", "%"+pattern+"%")
		}
		if filters[0].Kind != "" {
			q = q.Where("kind = ?", filters[0].Kind)
		}
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	counts := d.getDB().Model(&model.EdgeAgentHistory{}).Select("COUNT(*)").Where("owner_id = agent_accesses.owner_id AND access_id = agent_accesses.id AND edge_id = agent_accesses.edge_id AND deleted = ?", false)
	err := q.Select("agent_accesses.*, (?) AS session_count", counts).Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}
func (d *dao) GetAgentAccess(ctx context.Context, owner uint, id string) (*model.AgentAccess, error) {
	var row model.AgentAccess
	err := d.getDB().WithContext(ctx).Where("owner_id = ? AND id = ?", owner, id).First(&row).Error
	return &row, err
}
func (d *dao) SaveAgentAccess(ctx context.Context, row *model.AgentAccess, create bool) error {
	if create {
		return d.getDB().WithContext(ctx).Create(row).Error
	}
	result := d.getDB().WithContext(ctx).Model(&model.AgentAccess{}).Where("owner_id = ? AND id = ?", row.OwnerID, row.ID).Updates(map[string]any{"name": row.Name, "kind": row.Kind, "edge_id": row.EdgeID, "installation_id": row.InstallationID, "project": row.Project})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (d *dao) DeleteAgentAccess(ctx context.Context, owner uint, id string) error {
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("owner_id = ? AND id = ?", owner, id).Delete(&model.AgentAccess{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("owner_id = ? AND access_id = ?", owner, id).Delete(&model.EdgeAgentHistoryPage{}).Error; err != nil {
			return err
		}
		return tx.Where("owner_id = ? AND access_id = ?", owner, id).Delete(&model.EdgeAgentHistory{}).Error
	})
}
