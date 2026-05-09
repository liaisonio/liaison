package entry

import (
	"context"

	"github.com/liaisonio/liaison/pkg/entry/firewall"
	"github.com/liaisonio/liaison/pkg/entry/frontierbound"
	"github.com/liaisonio/liaison/pkg/entry/http"
	"github.com/liaisonio/liaison/pkg/entry/transport"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/proto"
)

type Entry struct {
	gatekeeper      *transport.Gatekeeper
	httpServer      *http.Server
	proxyManager    proto.ProxyManager
	firewallManager *firewall.Manager
	// liaison manager
	manager controlplane.ControlPlane
}

func NewEntry(conf *config.Configuration, manager controlplane.ControlPlane, trafficCollector interface {
	RecordTraffic(proxyID, applicationID uint, bytesIn, bytesOut int64)
}) (*Entry, error) {

	frontierBound, err := frontierbound.NewFrontierBound(conf)
	if err != nil {
		return nil, err
	}

	// 创建 TCP 端口管理器
	gatekeeper := transport.NewGatekeeper(frontierBound)
	// 设置流量统计器
	if trafficCollector != nil {
		gatekeeper.SetTrafficCollector(trafficCollector)
	}

	// 创建 HTTP 服务器
	httpServer := http.NewServer(frontierBound)
	// 设置流量统计器
	if trafficCollector != nil {
		httpServer.SetTrafficCollector(trafficCollector)
	}

	// 创建防火墙管理器（in-memory CIDR 注册表），共享给两个数据面
	firewallManager := firewall.NewManager()
	gatekeeper.SetFirewall(firewallManager)
	httpServer.SetFirewall(firewallManager)

	// 创建统一的 ProxyManager，根据应用类型路由到不同的服务器
	proxyManager := &unifiedProxyManager{
		gatekeeper: gatekeeper,
		httpServer: httpServer,
		conf:       conf,
	}
	manager.RegisterProxyManager(proxyManager)
	manager.RegisterFirewallManager(firewallManager)

	entry := &Entry{
		gatekeeper:      gatekeeper,
		httpServer:      httpServer,
		proxyManager:    proxyManager,
		firewallManager: firewallManager,
		manager:         manager,
	}

	manager.ReconcileLifecycleState()

	if err := manager.RestoreProxyListeners(); err != nil {
		return nil, err
	}

	return entry, nil
}

// unifiedProxyManager 统一的代理管理器，根据应用类型路由到不同的服务器
type unifiedProxyManager struct {
	gatekeeper *transport.Gatekeeper
	httpServer *http.Server
	conf       *config.Configuration
}

func (u *unifiedProxyManager) CreateProxy(ctx context.Context, protoproxy *proto.Proxy) error {
	// 如果是 HTTP 应用，使用 HTTP 服务器
	if protoproxy.ApplicationType == "http" {
		// 获取 TLS 证书配置
		var certFile, keyFile string
		if protoproxy.UseHTTPS && len(u.conf.Manager.Listen.TLS.Certs) > 0 {
			// 使用配置的第一个证书
			certFile = u.conf.Manager.Listen.TLS.Certs[0].Cert
			keyFile = u.conf.Manager.Listen.TLS.Certs[0].Key
		}
		return u.httpServer.CreateProxy(ctx, protoproxy, certFile, keyFile)
	}
	// 其他应用类型使用 TCP gatekeeper
	return u.gatekeeper.CreateProxy(ctx, protoproxy)
}

func (u *unifiedProxyManager) DeleteProxy(ctx context.Context, id int) error {
	// 始终两个数据面都调一次——两者对「不在本 map 里的 id」都返回 nil，所以
	// 双调是安全且必要的：老实现只要 httpServer 返回 nil 就退出，导致 TCP
	// 代理永远走不到 gatekeeper，listener 不释放；再次启用时端口冲突，
	// 「启用/关闭」都看起来失效。
	httpErr := u.httpServer.DeleteProxy(ctx, id)
	tcpErr := u.gatekeeper.DeleteProxy(ctx, id)
	if httpErr != nil {
		return httpErr
	}
	if tcpErr != nil {
		return tcpErr
	}
	return nil
}

func (e *Entry) Close() error {
	return nil
}
