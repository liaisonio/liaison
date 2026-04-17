package frontierbound

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/geminio"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
)

// 上报设备使用情况
func (fb *frontierBound) reportDeviceUsage(ctx context.Context, req geminio.Request, rsp geminio.Response) {
	var usage proto.DeviceUsage
	if err := json.Unmarshal(req.Data(), &usage); err != nil {
		rsp.SetError(err)
		return
	}

	// 通过指纹查找设备
	deviceModel, err := fb.repo.GetDeviceByFingerprint(usage.Fingerprint)
	if err != nil {
		log.Errorf("get device by fingerprint: %v error: %s", usage.Fingerprint, err)
		rsp.SetError(err)
		return
	}

	// 更新设备使用情况和心跳时间（UpdateDeviceUsage 已经会更新 heartbeat_at）
	err = fb.repo.UpdateDeviceUsage(deviceModel.ID, usage.CPUUsage, usage.MemoryUsage, usage.DiskUsage)
	if err != nil {
		rsp.SetError(err)
		return
	}

	// 更新 edge 心跳（edge 定期上报设备信息时更新心跳）
	// 优先从 request 的 ClientID 获取
	var edgeID uint64
	if clientID := req.ClientID(); clientID > 0 {
		edgeID = clientID
	} else {
		// 如果无法从 ClientID 获取，通过设备查找关联的 edge
		hostType := model.EdgeDeviceRelationHost
		edgeDevices, err := fb.repo.GetEdgeDevicesByDeviceID(deviceModel.ID, &hostType)
		if err == nil && len(edgeDevices) > 0 {
			edgeID = edgeDevices[0].EdgeID
		}
	}

	if edgeID > 0 {
		fb.updateEdgeHeartbeat(edgeID)
	} else {
		log.Debugf("cannot get edgeID from device usage request for heartbeat update")
	}
}

// 上报设备
func (fb *frontierBound) reportDevice(ctx context.Context, req geminio.Request, rsp geminio.Response) {
	var device proto.Device
	if err := json.Unmarshal(req.Data(), &device); err != nil {
		rsp.SetError(err)
		return
	}

	tx := fb.repo.Begin()
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// 根据设备指纹查找设备
	deviceModel, err := tx.GetDeviceByFingerprint(device.Fingerprint)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("get device by fingerprint error: %s", err)
		rsp.SetError(err)
		return
	}

	if deviceModel == nil {
		deviceModel = &model.Device{
			Name:        device.HostName,
			Fingerprint: device.Fingerprint,
			HostName:    device.HostName,
			CPU:         device.CPU,
			Memory:      device.Memory,
			Disk:        device.Disk,
			OS:          device.OS,
			OSVersion:   device.OSVersion,
			CPUUsage:    device.DeviceUsage.CPUUsage,
			MemoryUsage: device.DeviceUsage.MemoryUsage,
			DiskUsage:   device.DeviceUsage.DiskUsage,
		}
		if err := tx.CreateDevice(deviceModel); err != nil {
			log.Errorf("create device error: %s", err)
			rsp.SetError(err)
			return
		}
		// 创建设备后，更新心跳时间
		if err := tx.UpdateDeviceHeartbeat(deviceModel.ID); err != nil {
			log.Errorf("update device heartbeat error: %s", err)
			rsp.SetError(err)
			return
		}
	} else {
		updates := &model.Device{
			Fingerprint: device.Fingerprint,
			HostName:    device.HostName,
			CPU:         device.CPU,
			Memory:      device.Memory,
			Disk:        device.Disk,
			OS:          device.OS,
			OSVersion:   device.OSVersion,
			CPUUsage:    device.DeviceUsage.CPUUsage,
			MemoryUsage: device.DeviceUsage.MemoryUsage,
			DiskUsage:   device.DeviceUsage.DiskUsage,
		}
		updates.ID = deviceModel.ID
		if deviceModel.Name == "" {
			updates.Name = device.HostName
		}
		if err := tx.UpdateDevice(updates); err != nil {
			log.Errorf("update device error: %s", err)
			rsp.SetError(err)
			return
		}
		// 更新设备后，更新心跳时间
		if err := tx.UpdateDeviceHeartbeat(deviceModel.ID); err != nil {
			log.Errorf("update device heartbeat error: %s", err)
			rsp.SetError(err)
			return
		}
	}

	// 收集上报的所有接口（使用 ip+netmask+name+mac 作为唯一标识）
	type interfaceKey struct {
		IP      string
		Netmask string
		Name    string
		MAC     string
	}
	reportedInterfaces := make(map[interfaceKey]bool)
	for _, iface := range device.Interfaces {
		for _, ipMask := range iface.IPMasks {
			key := interfaceKey{
				IP:      ipMask.IP,
				Netmask: ipMask.Netmask,
				Name:    iface.Name,
				MAC:     iface.MAC,
			}
			reportedInterfaces[key] = true
		}
	}

	// 创建或更新接口
	for _, iface := range device.Interfaces {
		for _, ipMask := range iface.IPMasks {
			// 查找是否已存在该接口（根据 device_id、ip、netmask、name、mac 唯一标识）
			existingIface, err := tx.GetEthernetInterface(deviceModel.ID, ipMask.IP, ipMask.Netmask, iface.Name, iface.MAC)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				log.Errorf("get ethernet interface error: %s", err)
				rsp.SetError(err)
				return
			}

			interfaceModel := &model.EthernetInterface{
				DeviceID: deviceModel.ID,
				Name:     iface.Name,
				MAC:      iface.MAC,
				IP:       ipMask.IP,
				Netmask:  ipMask.Netmask,
			}

			if existingIface == nil {
				// 不存在，创建
				if err := tx.CreateEthernetInterface(interfaceModel); err != nil {
					log.Errorf("create ethernet interface error: %s", err)
					rsp.SetError(err)
					return
				}
			} else {
				// 存在，更新（虽然字段都相同，但可能其他字段需要更新）
				interfaceModel.ID = existingIface.ID
				if err := tx.UpdateEthernetInterface(interfaceModel); err != nil {
					log.Errorf("update ethernet interface error: %s", err)
					rsp.SetError(err)
					return
				}
			}
		}
	}

	// 删除不在上报列表中的接口
	existingInterfaces, err := tx.GetEthernetInterfacesByDeviceID(deviceModel.ID)
	if err != nil {
		log.Errorf("get ethernet interfaces error: %s", err)
		rsp.SetError(err)
		return
	}
	for _, existingIface := range existingInterfaces {
		key := interfaceKey{
			IP:      existingIface.IP,
			Netmask: existingIface.Netmask,
			Name:    existingIface.Name,
			MAC:     existingIface.MAC,
		}
		if !reportedInterfaces[key] {
			// 该接口不在上报列表中，删除
			if err := tx.DeleteEthernetInterface(existingIface.ID); err != nil {
				log.Errorf("delete ethernet interface error: %s", err)
				rsp.SetError(err)
				return
			}
		}
	}

	// 如果关联到edge，创建 EdgeDevice 关系（Host 类型）
	// 注意：一个 Edge 只能有一个 Host 类型的 Device，所以需要先删除旧的关系
	if device.EdgeID > 0 {
		hostType := model.EdgeDeviceRelationHost
		// 先查询是否已经存在该关系
		existingEdgeDevice, err := tx.GetEdgeDevice(device.EdgeID, deviceModel.ID, hostType)
		if err != nil {
			log.Errorf("get edge device relation error: %s", err)
			rsp.SetError(err)
			return
		}

		// 如果关系不存在，才创建
		if existingEdgeDevice == nil {
			// 先删除该 Edge 的所有旧的 Host 类型关系（因为指纹可能变化，导致设备 ID 变化）
			if err := tx.DeleteEdgeDevicesByEdgeID(device.EdgeID, &hostType); err != nil {
				log.Errorf("delete old edge device relations error: %s", err)
				rsp.SetError(err)
				return
			}
			// 创建新的关系
			edgeDevice := &model.EdgeDevice{
				EdgeID:   device.EdgeID,
				DeviceID: deviceModel.ID,
				Type:     hostType,
			}
			if err := tx.CreateEdgeDevice(edgeDevice); err != nil {
				log.Errorf("create edge device relation error: %s", err)
				rsp.SetError(err)
				return
			}
		}
		// 如果关系已存在，跳过创建
	}

	if err := tx.Commit(); err != nil {
		rsp.SetError(err)
		return
	}
	committed = true

	fb.updateEdgeHeartbeat(req.ClientID())
}
