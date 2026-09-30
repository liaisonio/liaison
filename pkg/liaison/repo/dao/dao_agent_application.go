package dao

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAgentApplicationInUse = errors.New("Agent application is referenced")

func (d *dao) ListAgentApplications(ctx context.Context, owner uint, edge uint64, page, size int) ([]model.AgentApplication, int64, error) {
	rows := []model.AgentApplication{}
	var total int64
	q := d.getDB().WithContext(ctx).Model(&model.AgentApplication{}).Where("owner_id = ?", owner)
	if edge != 0 {
		q = q.Where("edge_id = ?", edge)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	counts := d.getDB().Model(&model.AgentAccess{}).Select("COUNT(*)").Where("owner_id = agent_applications.owner_id AND application_id = agent_applications.id")
	err := q.Select("agent_applications.*, (?) AS access_count", counts).Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

func (d *dao) SaveAgentApplication(ctx context.Context, row *model.AgentApplication, create bool) error {
	if create {
		if err := d.getDB().WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_id"}, {Name: "edge_id"}, {Name: "kind"}, {Name: "installation_id"}}, DoNothing: true}).Create(row).Error; err != nil {
			return err
		}
		var existing model.AgentApplication
		err := d.getDB().WithContext(ctx).Where("owner_id = ? AND edge_id = ? AND kind = ? AND installation_id = ?", row.OwnerID, row.EdgeID, row.Kind, row.InstallationID).First(&existing).Error
		if err == nil {
			*row = existing
		}
		return err
	}
	result := d.getDB().WithContext(ctx).Model(&model.AgentApplication{}).Where("owner_id = ? AND id = ?", row.OwnerID, row.ID).Update("name", row.Name)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *dao) DeleteAgentApplication(ctx context.Context, owner uint, id string) error {
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.AgentApplication
		if err := tx.Where("owner_id = ? AND id = ?", owner, id).First(&row).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.AgentAccess{}).Where("owner_id = ? AND (application_id = ? OR (edge_id = ? AND kind = ? AND installation_id = ?))", owner, id, row.EdgeID, row.Kind, row.InstallationID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrAgentApplicationInUse
		}
		return tx.Where("owner_id = ? AND id = ?", owner, id).Delete(&model.AgentApplication{}).Error
	})
}

// bindAgentApplication runs inside the access save transaction. Explicit references
// must match the legacy fields; older clients register/reuse a scoped installation.
func bindAgentApplication(tx *gorm.DB, access *model.AgentAccess) error {
	q := tx.Where("owner_id = ? AND edge_id = ? AND kind = ? AND installation_id = ?", access.OwnerID, access.EdgeID, access.Kind, access.InstallationID)
	var app model.AgentApplication
	if access.ApplicationID != "" {
		return q.Where("id = ?", access.ApplicationID).First(&app).Error
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	app = model.AgentApplication{ID: hex.EncodeToString(random[:]), OwnerID: access.OwnerID, EdgeID: access.EdgeID, Kind: access.Kind, InstallationID: access.InstallationID, Name: "Codex"}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_id"}, {Name: "edge_id"}, {Name: "kind"}, {Name: "installation_id"}}, DoNothing: true}).Create(&app).Error; err != nil {
		return err
	}
	app = model.AgentApplication{}
	if err := q.First(&app).Error; err != nil {
		return err
	}
	access.ApplicationID = app.ID
	return nil
}

// backfillAgentApplications changes metadata only. It never contacts an Edge.
func (d *dao) backfillAgentApplications() error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		var cursor string
		for {
			var rows []model.AgentAccess
			if err := tx.Where("(application_id IS NULL OR application_id = '') AND id > ?", cursor).Order("id").Limit(100).Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) == 0 {
				return nil
			}
			for i := range rows {
				row := &rows[i]
				if err := bindAgentApplication(tx, row); err != nil {
					return fmt.Errorf("backfill Agent application: %w", err)
				}
				if err := tx.Model(&model.AgentAccess{}).Where("id = ? AND owner_id = ?", row.ID, row.OwnerID).UpdateColumn("application_id", row.ApplicationID).Error; err != nil {
					return err
				}
				cursor = row.ID
			}
		}
	})
}

func (d *dao) GetAgentApplication(ctx context.Context, owner uint, id string) (*model.AgentApplication, error) {
	var app model.AgentApplication
	err := d.getDB().WithContext(ctx).Where("owner_id = ? AND id = ?", owner, id).First(&app).Error
	return &app, err
}
