package dao

import "github.com/liaisonio/liaison/pkg/liaison/repo/model"

func (d *dao) CreateAccessKey(accessKey *model.AccessKey) error {
	return d.getDB().Create(accessKey).Error
}

func (d *dao) GetAccessKeyByID(id uint) (*model.AccessKey, error) {
	var accessKey model.AccessKey
	if err := d.getDB().Where("id = ?", id).First(&accessKey).Error; err != nil {
		return nil, err
	}
	return &accessKey, nil
}

func (d *dao) DeleteAccessKeysByEdgeID(edgeID uint64) error {
	return d.getDB().Where("edge_id = ?", edgeID).Delete(&model.AccessKey{}).Error
}
