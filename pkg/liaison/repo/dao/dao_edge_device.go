package dao

import (
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

// CreateEdgeDevice 创建 Edge 和 Device 的关系
func (d *dao) CreateEdgeDevice(edgeDevice *model.EdgeDevice) error {
	return d.getDB().Create(edgeDevice).Error
}

// GetEdgeDevice 获取 Edge 和 Device 的关系
func (d *dao) GetEdgeDevice(edgeID uint64, deviceID uint, relationType model.EdgeDeviceRelationType) (*model.EdgeDevice, error) {
	var edgeDevice model.EdgeDevice
	err := d.getDB().Where("edge_id = ? AND device_id = ? AND type = ?", edgeID, deviceID, relationType).First(&edgeDevice).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &edgeDevice, nil
}

// GetEdgeDevicesByEdgeID 根据 Edge ID 获取所有关系
func (d *dao) GetEdgeDevicesByEdgeID(edgeID uint64, relationType *model.EdgeDeviceRelationType) ([]*model.EdgeDevice, error) {
	var edgeDevices []*model.EdgeDevice
	db := d.getDB().Where("edge_id = ?", edgeID)
	if relationType != nil {
		db = db.Where("type = ?", *relationType)
	}
	err := db.Find(&edgeDevices).Error
	return edgeDevices, err
}

// GetEdgeDevicesByDeviceID 根据 Device ID 获取所有关系
func (d *dao) GetEdgeDevicesByDeviceID(deviceID uint, relationType *model.EdgeDeviceRelationType) ([]*model.EdgeDevice, error) {
	var edgeDevices []*model.EdgeDevice
	db := d.getDB().Where("device_id = ?", deviceID)
	if relationType != nil {
		db = db.Where("type = ?", *relationType)
	}
	err := db.Find(&edgeDevices).Error
	return edgeDevices, err
}

// DeleteEdgeDevice 删除 Edge 和 Device 的关系
func (d *dao) DeleteEdgeDevice(edgeID uint64, deviceID uint, relationType model.EdgeDeviceRelationType) error {
	return d.getDB().Where("edge_id = ? AND device_id = ? AND type = ?", edgeID, deviceID, relationType).Delete(&model.EdgeDevice{}).Error
}

// DeleteEdgeDevicesByEdgeID 删除 Edge 的所有关系（可指定类型）
func (d *dao) DeleteEdgeDevicesByEdgeID(edgeID uint64, relationType *model.EdgeDeviceRelationType) error {
	db := d.getDB().Where("edge_id = ?", edgeID)
	if relationType != nil {
		db = db.Where("type = ?", *relationType)
	}
	return db.Delete(&model.EdgeDevice{}).Error
}

// DeleteEdgeDevicesByDeviceID 删除 Device 的所有关系（可指定类型）
func (d *dao) DeleteEdgeDevicesByDeviceID(deviceID uint, relationType *model.EdgeDeviceRelationType) error {
	db := d.getDB().Where("device_id = ?", deviceID)
	if relationType != nil {
		db = db.Where("type = ?", *relationType)
	}
	return db.Delete(&model.EdgeDevice{}).Error
}
