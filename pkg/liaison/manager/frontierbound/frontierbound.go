package frontierbound

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/utils"
	"github.com/singchia/frontier/api/dataplane/v1/service"
	"github.com/singchia/geminio"
)

type FrontierBound interface {
	EmitScanApplications(ctx context.Context, taskID uint, edgeID uint64, net *Net) error
	KickEdge(ctx context.Context, edgeID uint64) error
	Close() error
}

type frontierBound struct {
	repo             repo.Repo
	svc              service.Service
	edgeConnMu       sync.Mutex
	edgeConnAddr     map[uint64]string
	controlPlaneURL  string
	httpClient       *http.Client
	registerMu       sync.Mutex
	cancel           context.CancelFunc
	trafficCollector interface {
		RecordTraffic(proxyID, applicationID uint, bytesIn, bytesOut int64)
	}
}

const (
	registrationTimeout         = 10 * time.Second
	registrationRepairDelay     = 10 * time.Second
	registrationRefreshInterval = 5 * time.Minute
)

func NewFrontierBound(conf *config.Configuration, repo repo.Repo, trafficCollector interface {
	RecordTraffic(proxyID, applicationID uint, bytesIn, bytesOut int64)
}) (FrontierBound, error) {
	dial := conf.Frontier.Dial
	if len(dial.Addrs) == 0 {
		return nil, errors.New("dial addr is empty")
	}
	fb := &frontierBound{
		repo:             repo,
		edgeConnAddr:     map[uint64]string{},
		controlPlaneURL:  conf.Frontier.ControlPlaneURL,
		httpClient:       &http.Client{Timeout: 5 * time.Second},
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
	fb.svc = svc
	ctx, cancel := context.WithCancel(context.Background())
	fb.cancel = cancel
	registerCtx, cancelRegister := context.WithTimeout(context.Background(), registrationTimeout)
	defer cancelRegister()
	if err := fb.registerRPCs(registerCtx); err != nil {
		cancel()
		_ = svc.Close()
		return nil, err
	}
	go fb.refreshRegistrations(ctx)
	return fb, nil
}

func (fb *frontierBound) Close() error {
	if fb.cancel != nil {
		fb.cancel()
	}
	return fb.svc.Close()
}

func (fb *frontierBound) refreshRegistrations(ctx context.Context) {
	timer := time.NewTimer(registrationRepairDelay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			registerCtx, cancelRegister := context.WithTimeout(ctx, registrationTimeout)
			err := fb.registerRPCs(registerCtx)
			cancelRegister()
			if err != nil {
				log.Warnf("refresh frontier RPC registrations failed: %s", err)
			}
			timer.Reset(registrationRefreshInterval)
		}
	}
}

func (fb *frontierBound) registerRPCs(ctx context.Context) error {
	fb.registerMu.Lock()
	defer fb.registerMu.Unlock()

	// 注册 frontier 回调函数。
	if err := fb.svc.RegisterGetEdgeID(ctx, fb.getID); err != nil {
		return fmt.Errorf("register get edge id: %w", err)
	}
	if err := fb.svc.RegisterEdgeOnline(ctx, fb.online); err != nil {
		return fmt.Errorf("register edge online: %w", err)
	}
	if err := fb.svc.RegisterEdgeOffline(ctx, fb.offline); err != nil {
		return fmt.Errorf("register edge offline: %w", err)
	}

	registrations := []struct {
		name string
		rpc  func(context.Context, geminio.Request, geminio.Response)
	}{
		{name: "report_device", rpc: fb.reportDevice},
		{name: "report_device_usage", rpc: fb.reportDeviceUsage},
		{name: "report_edge", rpc: fb.reportEdge},
		{name: "report_task_scan_application", rpc: fb.reportTaskScanApplication},
		{name: "pull_task_scan_application", rpc: fb.pullTaskScanApplication},
		{name: "get_edge_discovered_devices", rpc: fb.getEdgeDiscoveredDevices},
		{name: "update_device_heartbeat", rpc: fb.updateDeviceHeartbeat},
	}
	for _, registration := range registrations {
		if err := fb.svc.Register(ctx, registration.name, registration.rpc); err != nil {
			return fmt.Errorf("register %s: %w", registration.name, err)
		}
	}
	return nil
}
