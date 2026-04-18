package controlplane

import (
	"context"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/proto"
)

type ControlPlane interface {
	CreateEdge(ctx context.Context, req *v1.CreateEdgeRequest) (*v1.CreateEdgeResponse, error)
	GetEdge(ctx context.Context, req *v1.GetEdgeRequest) (*v1.GetEdgeResponse, error)
	ListEdges(ctx context.Context, req *v1.ListEdgesRequest) (*v1.ListEdgesResponse, error)
	UpdateEdge(ctx context.Context, req *v1.UpdateEdgeRequest) (*v1.UpdateEdgeResponse, error)
	DeleteEdge(ctx context.Context, req *v1.DeleteEdgeRequest) (*v1.DeleteEdgeResponse, error)

	ListDevices(ctx context.Context, req *v1.ListDevicesRequest) (*v1.ListDevicesResponse, error)
	GetDevice(ctx context.Context, req *v1.GetDeviceRequest) (*v1.GetDeviceResponse, error)
	UpdateDevice(ctx context.Context, req *v1.UpdateDeviceRequest) (*v1.UpdateDeviceResponse, error)
	DeleteDevice(ctx context.Context, req *v1.DeleteDeviceRequest) (*v1.DeleteDeviceResponse, error)

	CreateApplication(ctx context.Context, req *v1.CreateApplicationRequest) (*v1.CreateApplicationResponse, error)
	ListApplications(ctx context.Context, req *v1.ListApplicationsRequest) (*v1.ListApplicationsResponse, error)
	UpdateApplication(ctx context.Context, req *v1.UpdateApplicationRequest) (*v1.UpdateApplicationResponse, error)
	DeleteApplication(ctx context.Context, req *v1.DeleteApplicationRequest) (*v1.DeleteApplicationResponse, error)

	ListProxies(ctx context.Context, req *v1.ListProxiesRequest) (*v1.ListProxiesResponse, error)
	CreateProxy(ctx context.Context, req *v1.CreateProxyRequest) (*v1.CreateProxyResponse, error)
	UpdateProxy(ctx context.Context, req *v1.UpdateProxyRequest) (*v1.UpdateProxyResponse, error)
	DeleteProxy(ctx context.Context, req *v1.DeleteProxyRequest) (*v1.DeleteProxyResponse, error)

	CreateEdgeScanApplicationTask(ctx context.Context, req *v1.CreateEdgeScanApplicationTaskRequest) (*v1.CreateEdgeScanApplicationTaskResponse, error)
	GetEdgeScanApplicationTask(ctx context.Context, req *v1.GetEdgeScanApplicationTaskRequest) (*v1.GetEdgeScanApplicationTaskResponse, error)

	ListTrafficMetrics(ctx context.Context, req *v1.ListTrafficMetricsRequest) (*v1.ListTrafficMetricsResponse, error)

	// Firewall
	GetProxyFirewall(ctx context.Context, proxyID uint) (*FirewallData, error)
	UpsertProxyFirewall(ctx context.Context, proxyID uint, cidrs []string) (*FirewallData, error)
	DeleteProxyFirewall(ctx context.Context, proxyID uint) error

	RegisterProxyManager(proxyManager proto.ProxyManager)
	RegisterFirewallManager(firewallManager proto.FirewallManager)

	// RestoreFirewallRules rehydrates the data-plane firewall allowlist from
	// persisted rules. Call after the entry layer has finished starting
	// its proxies.
	RestoreFirewallRules()
}

func NewControlPlane(conf *config.Configuration, repo repo.Repo, frontierBound frontierbound.FrontierBound) (ControlPlane, error) {
	cp := &controlPlane{
		conf:          conf,
		repo:          repo,
		frontierBound: frontierBound,
	}

	// 初始化任务检查
	go cp.checkTask()

	return cp, nil
}

type controlPlane struct {
	conf          *config.Configuration
	repo          repo.Repo
	frontierBound frontierbound.FrontierBound

	// deps
	proxyManager    proto.ProxyManager
	firewallManager proto.FirewallManager
}
