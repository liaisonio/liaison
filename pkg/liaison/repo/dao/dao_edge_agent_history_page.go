package dao

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *dao) SaveEdgeAgentHistoryPage(ctx context.Context, page *model.EdgeAgentHistoryPage) error {
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		scope := &model.EdgeAgentHistory{OwnerID: page.OwnerID, AccessID: page.AccessID, EdgeID: page.EdgeID, SessionID: page.SessionID}
		var parent model.EdgeAgentHistory
		if err := historyScope(tx, scope).Where("deleted = ?", false).First(&parent).Error; err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(page).Error
	})
}

func (d *dao) PreviousEdgeAgentHistoryPage(ctx context.Context, scope *model.EdgeAgentHistory, before uint64) (*model.EdgeAgentHistoryPage, error) {
	var page model.EdgeAgentHistoryPage
	err := d.getDB().WithContext(ctx).Where("owner_id = ? AND access_id = ? AND edge_id = ? AND session_id = ? AND window < ?", scope.OwnerID, scope.AccessID, scope.EdgeID, scope.SessionID, before).Order("window DESC").First(&page).Error
	return &page, err
}
