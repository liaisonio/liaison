package pinger

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/edge/frontierbound"
	"github.com/liaisonio/liaison/pkg/proto"
)

type Pinger interface {
	Close() error
}

type pinger struct {
	frontierBound frontierbound.FrontierBound
	edgeID        uint64
	stopCh        chan struct{}
}

func NewPinger(frontierBound frontierbound.FrontierBound) (Pinger, error) {
	edgeID, err := frontierBound.EdgeID()
	if err != nil {
		return nil, err
	}

	p := &pinger{
		frontierBound: frontierBound,
		edgeID:        edgeID,
		stopCh:        make(chan struct{}),
	}

	go p.loopPing(context.Background())
	return p, nil
}

func (p *pinger) Close() error {
	close(p.stopCh)
	return nil
}

func (p *pinger) loopPing(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 立即执行一次
	p.pingDevices(ctx)

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.pingDevices(ctx)
		}
	}
}

func (p *pinger) pingDevices(ctx context.Context) {
	// 获取需要 ping 的设备列表
	request := proto.GetEdgeDiscoveredDevicesRequest{
		EdgeID: p.edgeID,
	}
	data, err := json.Marshal(request)
	if err != nil {
		log.Errorf("marshal get edge discovered devices request error: %v", err)
		return
	}

	req := p.frontierBound.NewRequest(data)
	rsp, err := p.frontierBound.Call(ctx, "get_edge_discovered_devices", req)
	if err != nil {
		log.Errorf("call get_edge_discovered_devices error: %v", err)
		return
	}
	if rsp.Error() != nil {
		log.Errorf("get_edge_discovered_devices error: %v", rsp.Error())
		return
	}

	var response proto.GetEdgeDiscoveredDevicesResponse
	if err := json.Unmarshal(rsp.Data(), &response); err != nil {
		log.Errorf("unmarshal get edge discovered devices response error: %v", err)
		return
	}

	// 对每个设备进行 ping
	for _, device := range response.Devices {
		if p.pingIP(device.IP) {
			// ping 成功，更新心跳
			p.updateHeartbeat(ctx, device.DeviceID)
		}
	}
}

func (p *pinger) pingIP(ip string) bool {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "1000", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}

	err := cmd.Run()
	return err == nil
}

func (p *pinger) updateHeartbeat(ctx context.Context, deviceID uint64) {
	request := proto.UpdateDeviceHeartbeatRequest{
		DeviceID: deviceID,
	}
	data, err := json.Marshal(request)
	if err != nil {
		log.Errorf("marshal update device heartbeat request error: %v", err)
		return
	}

	req := p.frontierBound.NewRequest(data)
	rsp, err := p.frontierBound.Call(ctx, "update_device_heartbeat", req)
	if err != nil {
		log.Errorf("call update_device_heartbeat error: %v", err)
		return
	}
	if rsp.Error() != nil {
		log.Errorf("update_device_heartbeat error: %v", rsp.Error())
		return
	}
	log.Infof("update device heartbeat success: device_id=%d", deviceID)
}
