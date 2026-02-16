package frontierbound

import (
	"context"
	"errors"
	"math/rand"
	"net"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/frontier/api/dataplane/v1/service"
	"github.com/singchia/liaison/pkg/liaison/config"
	"github.com/singchia/liaison/pkg/liaison/repo"
	"github.com/singchia/liaison/pkg/utils"
)

type FrontierBound interface {
	EmitScanApplications(ctx context.Context, taskID uint, edgeID uint64, net *Net) error
	Close() error
}

type frontierBound struct {
	repo             repo.Repo
	svc              service.Service
	trafficCollector interface {
		RecordTraffic(proxyID, applicationID uint, bytesIn, bytesOut int64)
	}
}

func NewFrontierBound(conf *config.Configuration, repo repo.Repo, trafficCollector interface {
	RecordTraffic(proxyID, applicationID uint, bytesIn, bytesOut int64)
}) (FrontierBound, error) {
	dial := conf.Frontier.Dial
	if len(dial.Addrs) == 0 {
		return nil, errors.New("dial addr is empty")
	}
	fb := &frontierBound{
		repo:             repo,
		trafficCollector: trafficCollector,
	}

	dialer := func() (net.Conn, error) {
		return utils.Dial(&dial, rand.Intn(len(dial.Addrs)))
	}
	svc, err := service.NewService(dialer, service.OptionServiceLog(log.DefaultLog), service.OptionServiceBufferSize(512, 512))
	if err != nil {
		log.Errorf("new service error: %s", err)
		return nil, err
	}
	// 注册frontier回调函数
	err = svc.RegisterGetEdgeID(context.Background(), fb.getID)
	if err != nil {
		log.Errorf("register get edge id error: %s", err)
		return nil, err
	}
	err = svc.RegisterEdgeOnline(context.Background(), fb.online)
	if err != nil {
		log.Errorf("register edge online error: %s", err)
		return nil, err
	}
	err = svc.RegisterEdgeOffline(context.Background(), fb.offline)
	if err != nil {
		log.Errorf("register edge offline error: %s", err)
		return nil, err
	}
	// 注册liaison函数
	err = svc.Register(context.Background(), "report_device", fb.reportDevice)
	if err != nil {
		log.Errorf("register report device error: %s", err)
		return nil, err
	}
	err = svc.Register(context.Background(), "report_device_usage", fb.reportDeviceUsage)
	if err != nil {
		log.Errorf("register report device usage error: %s", err)
		return nil, err
	}
	err = svc.Register(context.Background(), "report_edge", fb.reportEdge)
	if err != nil {
		log.Errorf("register report edge error: %s", err)
		return nil, err
	}
	err = svc.Register(context.Background(), "report_task_scan_application", fb.reportTaskScanApplication)
	if err != nil {
		log.Errorf("register report task scan application error: %s", err)
		return nil, err
	}
	err = svc.Register(context.Background(), "pull_task_scan_application", fb.pullTaskScanApplication)
	if err != nil {
		log.Errorf("register pull task scan application error: %s", err)
		return nil, err
	}
	err = svc.Register(context.Background(), "get_edge_discovered_devices", fb.getEdgeDiscoveredDevices)
	if err != nil {
		log.Errorf("register get edge discovered devices error: %s", err)
		return nil, err
	}
	err = svc.Register(context.Background(), "update_device_heartbeat", fb.updateDeviceHeartbeat)
	if err != nil {
		log.Errorf("register update device heartbeat error: %s", err)
		return nil, err
	}
	// 流量统计已移到entry端，不再需要edge端上报

	fb.svc = svc
	return fb, nil
}

func (fb *frontierBound) Close() error {
	return fb.svc.Close()
}
