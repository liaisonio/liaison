package dao

import "github.com/liaisonio/liaison/pkg/liaison/repo/model"

func (d *dao) CreateApplication(application *model.Application) error {
	return d.getDB().Create(application).Error
}

func (d *dao) GetApplicationByID(id uint) (*model.Application, error) {
	var application model.Application
	if err := d.getDB().Where("id = ?", id).First(&application).Error; err != nil {
		return nil, err
	}
	return &application, nil
}

func (d *dao) DeleteApplication(id uint) error {
	return d.getDB().Delete(&model.Application{}, id).Error
}

func (d *dao) UpdateApplication(application *model.Application) error {
	return d.getDB().Save(application).Error
}

func (d *dao) CountApplications(query *ListApplicationsQuery) (int64, error) {
	db := d.getDB()
	if len(query.DeviceIDs) > 0 {
		db = db.Where("device_id IN ?", query.DeviceIDs)
	}
	if len(query.IDs) > 0 {
		db = db.Where("id IN ?", query.IDs)
	}
	if query.ApplicationType != "" {
		db = db.Where("application_type = ?", query.ApplicationType)
	}
	var count int64
	if err := db.Model(&model.Application{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (d *dao) ListApplications(query *ListApplicationsQuery) ([]*model.Application, error) {
	var applications []*model.Application
	db := d.getDB()
	// device_ids
	if len(query.DeviceIDs) > 0 {
		db = db.Where("device_id IN ?", query.DeviceIDs)
	}
	// application_type
	if query.ApplicationType != "" {
		db = db.Where("application_type = ?", query.ApplicationType)
	}
	// page & page_size
	if query.Page > 0 && query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}
	// ids
	if len(query.IDs) > 0 {
		db = db.Where("id IN ?", query.IDs)
	}
	// 应用排序
	if query.Order != "" {
		if query.Desc {
			db = db.Order(query.Order + " DESC")
		} else {
			db = db.Order(query.Order + " ASC")
		}
	}

	if err := db.Find(&applications).Error; err != nil {
		return nil, err
	}
	return applications, nil
}
