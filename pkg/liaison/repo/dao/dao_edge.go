package dao

import (
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func (d *dao) GetEdge(id uint64) (*model.Edge, error) {
	var edge model.Edge
	if err := d.getDB().Where("id = ?", id).First(&edge).Error; err != nil {
		return nil, err
	}
	return &edge, nil
}

func (d *dao) GetEdgeByAccessKey(accessKey string) (*model.AccessKey, *model.Edge, error) {
	var ak model.AccessKey
	if err := d.getDB().Where("access_key = ?", accessKey).First(&ak).Error; err != nil {
		return nil, nil, err
	}
	edge, err := d.GetEdge(uint64(ak.EdgeID))
	if err != nil {
		return nil, nil, err
	}
	return &ak, edge, nil
}

func (d *dao) CreateEdge(edge *model.Edge) error {
	return d.getDB().Create(edge).Error
}

func (d *dao) GetEdgeByDeviceID(deviceID uint) (*model.Edge, error) {
	// 通过 EdgeDevice 关系表查询（Host 类型）
	hostType := model.EdgeDeviceRelationHost
	edgeDevices, err := d.GetEdgeDevicesByDeviceID(deviceID, &hostType)
	if err != nil {
		return nil, err
	}
	if len(edgeDevices) == 0 {
		return nil, nil
	}
	// 返回第一个关联的 Edge
	return d.GetEdge(edgeDevices[0].EdgeID)
}

func (d *dao) CountEdges(query *ListEdgesQuery) (int64, error) {
	var count int64
	db := d.getDB()
	if len(query.EdgeIDs) > 0 {
		db = db.Where("id IN ?", query.EdgeIDs)
	} else if len(query.DeviceIDs) > 0 {
		// 兼容旧逻辑：通过 EdgeDevice 关系表查询
		db = db.Joins("JOIN edge_devices ON edge_devices.edge_id = edges.id").
			Where("edge_devices.device_id IN ? AND edge_devices.type = ?", query.DeviceIDs, model.EdgeDeviceRelationHost)
	}
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if err := db.Model(&model.Edge{}).Distinct("edges.id").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (d *dao) ListEdges(query *ListEdgesQuery) ([]*model.Edge, error) {
	db := d.getDB()
	if query.Page > 0 && query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}
	if len(query.EdgeIDs) > 0 {
		db = db.Where("id IN ?", query.EdgeIDs)
	} else if len(query.DeviceIDs) > 0 {
		// 兼容旧逻辑：通过 EdgeDevice 关系表查询
		db = db.Joins("JOIN edge_devices ON edge_devices.edge_id = edges.id").
			Where("edge_devices.device_id IN ? AND edge_devices.type = ?", query.DeviceIDs, model.EdgeDeviceRelationHost)
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
	var edges []*model.Edge
	if err := db.Distinct("edges.*").Find(&edges).Error; err != nil {
		return nil, err
	}
	return edges, nil
}

// 更新Name Description Status
func (d *dao) UpdateEdge(edge *model.Edge) error {
	updates := make(map[string]interface{})
	if edge.Name != "" {
		updates["name"] = edge.Name
	}
	if edge.Description != "" {
		updates["description"] = edge.Description
	}
	if edge.Status != 0 {
		updates["status"] = edge.Status
	}
	if len(updates) > 0 {
		if err := d.getDB().Model(&model.Edge{}).Where("id = ?", edge.ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func (d *dao) UpdateEdgeOnlineStatus(edgeID uint64, onlineStatus model.EdgeOnlineStatus) error {
	return d.getDB().Model(&model.Edge{}).Where("id = ?", edgeID).Update("online", onlineStatus).Error
}

func (d *dao) UpdateEdgeHeartbeatAt(edgeID uint64, heartbeatAt time.Time) error {
	return d.getDB().Model(&model.Edge{}).Where("id = ?", edgeID).Update("heartbeat_at", heartbeatAt).Error
}

func (d *dao) UpdateEdgeDeviceID(edgeID uint64, deviceID uint) error {
	return d.getDB().Model(&model.Edge{}).Where("id = ?", edgeID).Update("device_id", deviceID).Error
}

func (d *dao) DeleteEdge(id uint64) error {
	return d.getDB().Delete(&model.Edge{}, id).Error
}
