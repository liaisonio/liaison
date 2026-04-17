package frontierbound

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/geminio"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

// 获取 Edge 发现的设备列表（用于 ping）
func (fb *frontierBound) getEdgeDiscoveredDevices(ctx context.Context, req geminio.Request, rsp geminio.Response) {
	var request proto.GetEdgeDiscoveredDevicesRequest
	if err := json.Unmarshal(req.Data(), &request); err != nil {
		rsp.SetError(err)
		return
	}

	// 通过 EdgeDevice 关系表获取 Edge 发现的设备（类型为 Discovered）
	discoveredType := model.EdgeDeviceRelationDiscovered
	edgeDevices, err := fb.repo.GetEdgeDevicesByEdgeID(request.EdgeID, &discoveredType)
	if err != nil {
		log.Errorf("get edge discovered devices error: %s", err)
		rsp.SetError(err)
		return
	}

	// 获取每个设备的 IP 地址（取第一个 IPv4 地址）
	devices := make([]proto.DiscoveredDevice, 0, len(edgeDevices))
	for _, edgeDevice := range edgeDevices {
		device, err := fb.repo.GetDeviceByID(edgeDevice.DeviceID)
		if err != nil {
			log.Errorf("get device by id error: %s, device_id: %d", err, edgeDevice.DeviceID)
			continue
		}

		// 获取设备的网络接口
		interfaces, err := fb.repo.GetEthernetInterfacesByDeviceID(device.ID)
		if err != nil {
			log.Errorf("get ethernet interfaces error: %s, device_id: %d", err, device.ID)
			continue
		}

		// 找到第一个 IPv4 地址
		ip := ""
		for _, iface := range interfaces {
			// 跳过 IPv6 地址（包含冒号）
			if iface.IP != "" && !strings.Contains(iface.IP, ":") {
				ip = iface.IP
				break
			}
		}

		if ip != "" {
			devices = append(devices, proto.DiscoveredDevice{
				DeviceID: uint64(device.ID),
				IP:       ip,
			})
		}
	}

	response := proto.GetEdgeDiscoveredDevicesResponse{
		Devices: devices,
	}
	data, err := json.Marshal(response)
	if err != nil {
		rsp.SetError(err)
		return
	}
	rsp.SetData(data)
}

// 更新设备心跳
func (fb *frontierBound) updateDeviceHeartbeat(ctx context.Context, req geminio.Request, rsp geminio.Response) {
	var request proto.UpdateDeviceHeartbeatRequest
	if err := json.Unmarshal(req.Data(), &request); err != nil {
		rsp.SetError(err)
		return
	}

	err := fb.repo.UpdateDeviceHeartbeat(uint(request.DeviceID))
	if err != nil {
		log.Errorf("update device heartbeat error: %s, device_id: %d", err, request.DeviceID)
		rsp.SetError(err)
		return
	}
}
