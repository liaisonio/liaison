package dao

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ListEdgeAgentHistoryPages streams a bounded index range rather than allocating
// every payload up front. The connection is released before authorization rechecks.
func (d *dao) ListEdgeAgentHistoryPages(ctx context.Context, scope *model.EdgeAgentHistory, before uint64, limit, maxBytes int) ([]model.EdgeAgentHistoryPage, error) {
	if limit < 1 || limit > 20 || maxBytes < 512<<10 || maxBytes > 2<<20 {
		return nil, errors.New("invalid history page budget")
	}
	query := d.getDB().WithContext(ctx).Model(&model.EdgeAgentHistoryPage{}).Where("owner_id = ? AND access_id = ? AND edge_id = ? AND session_id = ? AND window < ?", scope.OwnerID, scope.AccessID, scope.EdgeID, scope.SessionID, before).Order("window DESC").Limit(limit)
	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pages []model.EdgeAgentHistoryPage
	total := 0
	for rows.Next() {
		var page model.EdgeAgentHistoryPage
		if err := query.ScanRows(rows, &page); err != nil {
			return nil, err
		}
		if len(page.Payload) > maxBytes {
			return nil, errors.New("history page exceeds budget")
		}
		if total+len(page.Payload) > maxBytes {
			break
		}
		pages = append(pages, page)
		total += len(page.Payload)
	}
	return pages, rows.Err()
}

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
