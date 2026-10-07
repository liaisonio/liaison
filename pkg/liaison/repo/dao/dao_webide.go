package dao

import (
	"context"
	"errors"
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrWebIDEInUse = errors.New("IDE application is referenced")

func (d *dao) WebIDEApplications(ctx context.Context, owner uint, page, size int) ([]model.WebIDEApplication, int64, error) {
	rows := []model.WebIDEApplication{}
	var total int64
	if owner == 0 || page < 1 || size < 1 || size > 100 {
		return nil, 0, errors.New("invalid IDE scope")
	}
	q := d.getDB().WithContext(ctx).Model(&model.WebIDEApplication{}).Where("owner_id = ?", owner)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}
func (d *dao) GetWebIDEApplication(ctx context.Context, owner uint, id string) (*model.WebIDEApplication, error) {
	var row model.WebIDEApplication
	err := d.getDB().WithContext(ctx).Where("owner_id = ? AND id = ?", owner, id).First(&row).Error
	return &row, err
}
func (d *dao) SaveWebIDEApplication(ctx context.Context, row *model.WebIDEApplication, create bool) error {
	if row == nil || row.OwnerID == 0 || len(row.ID) != 32 {
		return errors.New("invalid IDE application")
	}
	if create {
		return d.getDB().WithContext(ctx).Create(row).Error
	}
	r := d.getDB().WithContext(ctx).Model(&model.WebIDEApplication{}).Where("owner_id = ? AND id = ?", row.OwnerID, row.ID).Update("name", row.Name)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (d *dao) DeleteWebIDEApplication(ctx context.Context, owner uint, id string) error {
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the parent on databases supporting row locks. SQLite serializes
		// writes and rejects a concurrent snapshot upgrade rather than orphaning it.
		var app model.WebIDEApplication
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", owner, id).First(&app).Error; err != nil {
			return err
		}
		var n int64
		if err := tx.Model(&model.WebIDEAccess{}).Where("owner_id = ? AND application_id = ?", owner, id).Count(&n).Error; err != nil {
			return err
		}
		if n != 0 {
			return ErrWebIDEInUse
		}
		r := tx.Where("owner_id = ? AND id = ?", owner, id).Delete(&model.WebIDEApplication{})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
func (d *dao) WebIDEAccesses(ctx context.Context, owner uint, page, size int, names ...string) ([]model.WebIDEAccess, int64, error) {
	rows := []model.WebIDEAccess{}
	var total int64
	if owner == 0 || page < 1 || size < 1 || size > 100 {
		return nil, 0, errors.New("invalid IDE scope")
	}
	q := d.getDB().WithContext(ctx).Model(&model.WebIDEAccess{}).Where("owner_id = ?", owner)
	if len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		name := strings.TrimSpace(names[0])
		if len(name) > 480 {
			return nil, 0, errors.New("invalid IDE filter")
		}
		literal := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(name))
		q = q.Where("LOWER(name) LIKE ? ESCAPE '!'", "%"+literal+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}
func (d *dao) GetWebIDEAccess(ctx context.Context, owner uint, id string) (*model.WebIDEAccess, error) {
	var row model.WebIDEAccess
	err := d.getDB().WithContext(ctx).Where("owner_id = ? AND id = ?", owner, id).First(&row).Error
	return &row, err
}
func (d *dao) SaveWebIDEAccess(ctx context.Context, row *model.WebIDEAccess, create bool) error {
	if row == nil || row.OwnerID == 0 || len(row.ID) != 32 {
		return errors.New("invalid IDE access")
	}
	return d.getDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var app model.WebIDEApplication
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", row.OwnerID, row.ApplicationID).First(&app).Error; err != nil {
			return err
		}
		if create {
			return tx.Create(row).Error
		}
		r := tx.Model(&model.WebIDEAccess{}).Where("owner_id = ? AND id = ? AND application_id = ?", row.OwnerID, row.ID, row.ApplicationID).Updates(map[string]any{"name": row.Name, "enabled": row.Enabled, "project": row.Project})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
func (d *dao) DeleteWebIDEAccess(ctx context.Context, owner uint, id string) error {
	r := d.getDB().WithContext(ctx).Where("owner_id = ? AND id = ?", owner, id).Delete(&model.WebIDEAccess{})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
