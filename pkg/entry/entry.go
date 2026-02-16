package entry

import (
	"context"
	"errors"
	"fmt"

	"github.com/jumboframes/armorigo/log"
	v1 "github.com/singchia/liaison/api/v1"
	"github.com/singchia/liaison/pkg/entry/frontierbound"
	"github.com/singchia/liaison/pkg/entry/http"
	"github.com/singchia/liaison/pkg/entry/transport"
	"github.com/singchia/liaison/pkg/liaison/config"
	"github.com/singchia/liaison/pkg/liaison/manager/controlplane"
	"github.com/singchia/liaison/pkg/proto"
)

type Entry struct {
	gatekeeper   *transport.Gatekeeper
	httpServer   *http.Server
	proxyManager proto.ProxyManager
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

	// 创建统一的 ProxyManager，根据应用类型路由到不同的服务器
	proxyManager := &unifiedProxyManager{
		gatekeeper: gatekeeper,
		httpServer: httpServer,
		conf:       conf,
	}
	manager.RegisterProxyManager(proxyManager)

	entry := &Entry{
		gatekeeper:   gatekeeper,
		httpServer:   httpServer,
		proxyManager: proxyManager,
		manager:      manager,
	}

	err = entry.pullProxyConfigs()
	if err != nil {
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
	// 先尝试从 HTTP 服务器删除
	err := u.httpServer.DeleteProxy(ctx, id)
	if err == nil {
		return nil
	}
	// 如果 HTTP 服务器中没有，尝试从 gatekeeper 删除
	return u.gatekeeper.DeleteProxy(ctx, id)
}

// pullProxyConfigs 定期从manager同步Proxy配置
func (e *Entry) pullProxyConfigs() error {
	rsp, err := e.manager.ListProxies(context.Background(), &v1.ListProxiesRequest{
		Page:     -1,
		PageSize: -1,
	})
	if err != nil {
		log.Errorf("failed to list proxies: %s", err)
		return err
	}
	if rsp.Code != 200 {
		log.Errorf("failed to list proxies: %s", rsp.Message)
		return errors.New(rsp.Message)
	}
	log.Infof("list proxies: %s", rsp.Data.GetProxies())

	for _, proxy := range rsp.Data.GetProxies() {
		application := proxy.Application
		if application == nil {
			log.Warnf("proxy %d (name: %s) has no associated application, skipping", proxy.Id, proxy.Name)
			continue
		}
		dst := fmt.Sprintf("%s:%d", application.Ip, application.Port)
		
		// 判断是否是 HTTP 应用，如果是则默认使用 HTTPS
		useHTTPS := false
		if application.ApplicationType == "http" {
			useHTTPS = true
		}
		
		// 使用统一的 ProxyManager
		e.proxyManager.CreateProxy(context.Background(), &proto.Proxy{
			ID:              int(proxy.Id),
			Name:            proxy.Name,
			ProxyPort:       int(proxy.Port),
			EdgeID:          application.EdgeId,
			ApplicationID:   uint(application.Id),
			Dst:             dst,
			ApplicationType: application.ApplicationType,
			UseHTTPS:        useHTTPS,
		})
	}

	return nil
}

func (e *Entry) Close() error {
	return nil
}
