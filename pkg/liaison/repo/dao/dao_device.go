package dao

import (
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func (d *dao) CreateDevice(device *model.Device) error {
	return d.getDB().Create(device).Error
}

func (d *dao) CreateEthernetInterface(iface *model.EthernetInterface) error {
	return d.getDB().Create(iface).Error
}

func (d *dao) GetEthernetInterface(deviceID uint, ip, netmask, name, mac string) (*model.EthernetInterface, error) {
	var iface model.EthernetInterface
	if err := d.getDB().Where("device_id = ? AND ip = ? AND netmask = ? AND name = ? AND mac = ?",
		deviceID, ip, netmask, name, mac).First(&iface).Error; err != nil {
		return nil, err
	}
	return &iface, nil
}

func (d *dao) GetEthernetInterfacesByDeviceID(deviceID uint) ([]*model.EthernetInterface, error) {
	var interfaces []*model.EthernetInterface
	if err := d.getDB().Where("device_id = ?", deviceID).Find(&interfaces).Error; err != nil {
		return nil, err
	}
	return interfaces, nil
}

func (d *dao) UpdateEthernetInterface(iface *model.EthernetInterface) error {
	return d.getDB().Model(&model.EthernetInterface{}).Where("id = ?", iface.ID).Updates(map[string]interface{}{
		"name":    iface.Name,
		"mac":     iface.MAC,
		"ip":      iface.IP,
		"netmask": iface.Netmask,
	}).Error
}

func (d *dao) DeleteEthernetInterface(id uint) error {
	return d.getDB().Delete(&model.EthernetInterface{}, id).Error
}

func (d *dao) GetDeviceByID(id uint) (*model.Device, error) {
	var device model.Device
	if err := d.getDB().Where("id = ?", id).First(&device).Error; err != nil {
		return nil, err
	}
	if err := d.getDB().Where("device_id = ?", id).Find(&device.Interfaces).Error; err != nil {
		return nil, err
	}
	// 根据 heartbeat_at 更新在线状态
	d.updateDeviceOnlineStatusByHeartbeat([]*model.Device{&device})
	return &device, nil
}

func (d *dao) GetDeviceByFingerprint(fingerprint string) (*model.Device, error) {
	var device model.Device
	if err := d.getDB().Where("fingerprint = ?", fingerprint).First(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

// GetDeviceByIP 根据 IP 地址查找 Device（通过 ethernet_interfaces 表）
func (d *dao) GetDeviceByIP(ip string) (*model.Device, error) {
	var device model.Device
	// 通过 JOIN ethernet_interfaces 表来查找包含该 IP 的 Device
	err := d.getDB().
		Joins("JOIN ethernet_interfaces ON ethernet_interfaces.device_id = devices.id").
		Where("ethernet_interfaces.ip = ?", ip).
		First(&device).Error
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (d *dao) CountDevices(query *ListDevicesQuery) (int64, error) {
	var count int64
	db := d.getDB()
	if len(query.IDs) > 0 {
		db = db.Where("devices.id IN ?", query.IDs)
	}
	if query.Name != "" {
		db = db.Where("devices.name LIKE ?", "%"+query.Name+"%")
	}
	if query.IP != "" {
		// 通过JOIN ethernet_interfaces表来搜索IP
		db = db.Joins("JOIN ethernet_interfaces ON ethernet_interfaces.device_id = devices.id").
			Where("ethernet_interfaces.ip LIKE ?", "%"+query.IP+"%")
	}
	if err := db.Model(&model.Device{}).Distinct("devices.id").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (d *dao) ListDevices(query *ListDevicesQuery) ([]*model.Device, error) {
	db := d.getDB()
	if query.Page > 0 && query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}
	if len(query.IDs) > 0 {
		db = db.Where("devices.id IN ?", query.IDs)
	}
	if query.Name != "" {
		db = db.Where("devices.name LIKE ?", "%"+query.Name+"%")
	}
	if query.IP != "" {
		// 通过JOIN ethernet_interfaces表来搜索IP
		db = db.Joins("JOIN ethernet_interfaces ON ethernet_interfaces.device_id = devices.id").
			Where("ethernet_interfaces.ip LIKE ?", "%"+query.IP+"%")
	}
	// 应用排序
	if query.Order != "" {
		orderField := query.Order
		if query.Order == "id" {
			orderField = "devices.id"
		}
		if query.Desc {
			db = db.Order(orderField + " DESC")
		} else {
			db = db.Order(orderField + " ASC")
		}
	}
	var devices []*model.Device
	if err := db.Distinct("devices.*").Find(&devices).Error; err != nil {
		return nil, err
	}
	// 根据 heartbeat_at 更新在线状态
	d.updateDeviceOnlineStatusByHeartbeat(devices)
	return devices, nil
}

// updateDeviceOnlineStatusByHeartbeat 根据 heartbeat_at 更新设备的在线状态
// 如果 heartbeat_at 在最近1分钟内，则在线，否则离线
func (d *dao) updateDeviceOnlineStatusByHeartbeat(devices []*model.Device) {
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)

	for _, device := range devices {
		// 如果 heartbeat_at 在最近1分钟内，设置为在线
		if device.HeartbeatAt.After(oneMinuteAgo) {
			if device.Online != model.DeviceOnlineStatusOnline {
				device.Online = model.DeviceOnlineStatusOnline
				// 异步更新数据库中的状态
				go func(deviceID uint) {
					d.getDB().Model(&model.Device{}).Where("id = ?", deviceID).Update("online", model.DeviceOnlineStatusOnline)
				}(device.ID)
			}
		} else {
			// 如果 heartbeat_at 超过1分钟，设置为离线
			if device.Online != model.DeviceOnlineStatusOffline {
				device.Online = model.DeviceOnlineStatusOffline
				// 异步更新数据库中的状态
				go func(deviceID uint) {
					d.getDB().Model(&model.Device{}).Where("id = ?", deviceID).Update("online", model.DeviceOnlineStatusOffline)
				}(device.ID)
			}
		}
	}
}

func (d *dao) UpdateDevice(device *model.Device) error {
	updates := map[string]interface{}{}
	if device.Name != "" {
		updates["name"] = device.Name
	}
	if device.Description != "" {
		updates["description"] = device.Description
	}
	if device.CPU > 0 {
		updates["cpu"] = device.CPU
	}
	if device.Memory > 0 {
		updates["memory"] = device.Memory
	}
	if device.Disk > 0 {
		updates["disk"] = device.Disk
	}
	if device.OS != "" {
		updates["os"] = device.OS
	}
	if device.OSVersion != "" {
		updates["os_version"] = device.OSVersion
	}
	if device.HostName != "" {
		updates["host_name"] = device.HostName
	}
	if device.CPUUsage > 0 {
		updates["cpu_usage"] = device.CPUUsage
	}
	if device.MemoryUsage > 0 {
		updates["memory_usage"] = device.MemoryUsage
	}
	if device.DiskUsage > 0 {
		updates["disk_usage"] = device.DiskUsage
	}
	if len(updates) == 0 {
		return nil
	}
	return d.getDB().Model(&model.Device{}).Where("id = ?", device.ID).Updates(updates).Error
}

func (d *dao) UpdateDeviceUsage(deviceID uint, cpuUsage, memoryUsage, diskUsage float32) error {
	return d.getDB().Model(&model.Device{}).Where("id = ?", deviceID).Omit("updated_at").Updates(map[string]interface{}{
		"cpu_usage":    cpuUsage,
		"memory_usage": memoryUsage,
		"disk_usage":   diskUsage,
		"heartbeat_at": time.Now(),
	}).Error
}

// UpdateDeviceHeartbeat 更新设备心跳时间，并设置设备为在线状态
func (d *dao) UpdateDeviceHeartbeat(deviceID uint) error {
	now := time.Now()
	return d.getDB().Model(&model.Device{}).Where("id = ?", deviceID).Updates(map[string]interface{}{
		"heartbeat_at": now,
		"online":       model.DeviceOnlineStatusOnline,
	}).Error
}

// DeleteDevice 删除设备及其关联数据
func (d *dao) DeleteDevice(id uint) error {
	tx := d.getDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除设备的网卡
	if err := tx.Where("device_id = ?", id).Delete(&model.EthernetInterface{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 删除 EdgeDevice 关系
	if err := tx.Where("device_id = ?", id).Delete(&model.EdgeDevice{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 删除设备本身
	if err := tx.Delete(&model.Device{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
