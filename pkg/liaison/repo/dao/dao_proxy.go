package dao

import "github.com/liaisonio/liaison/pkg/liaison/repo/model"

func (d *dao) CreateProxy(proxy *model.Proxy) error {
	return d.getDB().Create(proxy).Error
}

func (d *dao) GetProxyByID(id uint) (*model.Proxy, error) {
	var proxy model.Proxy
	if err := d.getDB().Where("id = ?", id).First(&proxy).Error; err != nil {
		return nil, err
	}
	return &proxy, nil
}

func (d *dao) CountProxies(query *ListProxiesQuery) (int64, error) {
	var count int64
	db := d.getDB()
	if query != nil {
		if query.Name != "" {
			db = db.Where("name LIKE ?", "%"+query.Name+"%")
		}
		if len(query.ApplicationIDs) > 0 {
			db = db.Where("application_id IN ?", query.ApplicationIDs)
		}
	}
	if err := db.Model(&model.Proxy{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (d *dao) ListProxies(query *ListProxiesQuery) ([]*model.Proxy, error) {
	db := d.getDB()
	// page & page_size
	if query.Page > 0 && query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}
	if len(query.IDs) > 0 {
		db = db.Where("id IN ?", query.IDs)
	}
	if len(query.ApplicationIDs) > 0 {
		db = db.Where("application_id IN ?", query.ApplicationIDs)
	}
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	// 应用排序
	if query.Order != "" {
		if query.Desc {
			db = db.Order(query.Order + " DESC")
		} else {
			db = db.Order(query.Order + " ASC")
		}
	}
	var proxies []*model.Proxy
	if err := db.Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

func (d *dao) UpdateProxy(proxy *model.Proxy) error {
	updates := map[string]interface{}{}
	if proxy.Name != "" {
		updates["name"] = proxy.Name
	}
	if proxy.Status != 0 {
		updates["status"] = proxy.Status
	}
	if proxy.Description != "" {
		updates["description"] = proxy.Description
	}
	if proxy.Port > 0 {
		updates["port"] = proxy.Port
	}
	return d.getDB().Model(&model.Proxy{}).Where("id = ?", proxy.ID).Updates(updates).Error
}

func (d *dao) DeleteProxy(id uint) error {
	return d.getDB().Delete(&model.Proxy{}, id).Error
}
