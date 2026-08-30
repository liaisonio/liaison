package dao

import (
	"errors"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

type ListManagementAuditsQuery struct {
	UserID    uint
	Module    string
	Action    string
	Success   *bool
	Keyword   string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

func (d *dao) CreateManagementAudit(audit *model.ManagementAudit) error {
	if audit == nil {
		return errors.New("management audit is nil")
	}
	return d.getDB().Create(audit).Error
}

func (d *dao) ListManagementAudits(query *ListManagementAuditsQuery) ([]*model.ManagementAudit, error) {
	var audits []*model.ManagementAudit
	db := d.applyManagementAuditFilters(d.getDB().Model(&model.ManagementAudit{}), query)
	limit := 20
	if query != nil {
		if query.Limit > 0 {
			limit = query.Limit
		}
		if query.Offset > 0 {
			db = db.Offset(query.Offset)
		}
	}
	if limit > 500 {
		limit = 500
	}
	if err := db.Order("created_at DESC, id DESC").Limit(limit).Find(&audits).Error; err != nil {
		return nil, err
	}
	return audits, nil
}

func (d *dao) CountManagementAudits(query *ListManagementAuditsQuery) (int64, error) {
	var count int64
	db := d.applyManagementAuditFilters(d.getDB().Model(&model.ManagementAudit{}), query)
	if err := db.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (d *dao) applyManagementAuditFilters(db *gorm.DB, query *ListManagementAuditsQuery) *gorm.DB {
	if query == nil {
		return db
	}
	if query.UserID > 0 {
		db = db.Where("user_id = ?", query.UserID)
	}
	if query.Module != "" {
		db = db.Where("module = ?", query.Module)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.Success != nil {
		db = db.Where("success = ?", *query.Success)
	}
	if query.StartTime != nil {
		db = db.Where("created_at >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("created_at <= ?", *query.EndTime)
	}
	if query.Keyword != "" {
		keyword := "%" + query.Keyword + "%"
		db = db.Where("user_email LIKE ? OR module LIKE ? OR action LIKE ? OR resource LIKE ? OR client_ip LIKE ?", keyword, keyword, keyword, keyword, keyword)
	}
	return db
}
