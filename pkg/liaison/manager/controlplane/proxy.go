package controlplane

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

func (cp *controlPlane) RegisterProxyManager(proxyManager proto.ProxyManager) {
	cp.proxyManager = proxyManager
}

func (cp *controlPlane) RegisterFirewallManager(firewallManager proto.FirewallManager) {
	cp.firewallManager = firewallManager
}

func (cp *controlPlane) CreateProxy(_ context.Context, req *v1.CreateProxyRequest) (*v1.CreateProxyResponse, error) {
	name, err := normalizeResourceName(req.Name, "Access")
	if err != nil {
		return nil, err
	}
	if req.ApplicationId == 0 {
		return nil, badRequest("APPLICATION_ID_REQUIRED", "关联应用不能为空")
	}
	if req.Port < 0 || req.Port > 65535 {
		return nil, badRequest("PROXY_PORT_INVALID", "公网端口必须在 1-65535 之间；SSH/RDP/VNC 可留空仅通过网页访问")
	}
	application, err := cp.repo.GetApplicationByID(uint(req.ApplicationId))
	if err != nil {
		log.Warnf("application %d not found", req.ApplicationId)
		return nil, mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "关联应用不存在")
	}
	edge, err := cp.validateProxyApplication(application)
	if err != nil {
		return nil, err
	}

	accessProtocol, err := normalizeAccessProtocol(req.AccessProtocol, application, int(req.Port), req.ExposePublicPort)
	if err != nil {
		return nil, err
	}
	requestedPort, err := cp.resolveCreateProxyPort(accessProtocol, int(req.Port))
	if err != nil {
		return nil, err
	}
	if requestedPort > 0 {
		if err := cp.ensureProxyPortAvailable(requestedPort, 0); err != nil {
			return nil, err
		}
	}

	proxy := &model.Proxy{
		Name:           name,
		Status:         model.ProxyStatusRunning,
		Description:    req.Description,
		Port:           requestedPort,
		ApplicationID:  uint(req.ApplicationId),
		AccessProtocol: accessProtocol,
	}
	err = cp.repo.CreateProxy(proxy)
	if err != nil {
		log.Warnf("failed to create proxy: %s", err)
		return nil, err
	}
	proxy.Application = application

	if edge.Status == model.EdgeStatusRunning {
		err = cp.startProxyRuntime(proxy, application)
		if err != nil {
			log.Errorf("failed to create proxy listener: %s", err)
			_ = cp.repo.DeleteProxy(proxy.ID)
			return nil, err
		}
		if requestedPort == 0 && proxy.Port > 0 {
			if err = cp.repo.UpdateProxy(proxy); err != nil {
				_ = cp.stopProxyRuntime(proxy)
				_ = cp.repo.DeleteProxy(proxy.ID)
				return nil, err
			}
		}
	}

	return &v1.CreateProxyResponse{
		Code:    200,
		Message: "success",
		Data:    cp.transformProxy(proxy),
	}, nil
}

func (cp *controlPlane) ListProxies(_ context.Context, req *v1.ListProxiesRequest) (*v1.ListProxiesResponse, error) {
	// list proxies
	query := dao.ListProxiesQuery{
		Query: dao.Query{
			Page:     int(req.Page),
			PageSize: int(req.PageSize),
			Order:    "id",
			Desc:     true,
		},
	}
	if req.Name != "" {
		query.Name = req.Name
	}
	proxies, err := cp.repo.ListProxies(&query)
	if err != nil {
		return nil, err
	}
	count, err := cp.repo.CountProxies(&query)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(proxies))
	seenIDs := make(map[uint]bool)
	for _, proxy := range proxies {
		if !seenIDs[proxy.ApplicationID] {
			ids = append(ids, proxy.ApplicationID)
			seenIDs[proxy.ApplicationID] = true
		}
	}
	var applications []*model.Application
	if len(ids) > 0 {
		applications, err = cp.repo.ListApplications(&dao.ListApplicationsQuery{
			Query: dao.Query{
				Order: "id",
				Desc:  true,
			},
			IDs: ids,
		})
		if err != nil {
			return nil, err
		}
	}
	// add applications to proxies
	// 创建一个 map 来快速查找 application
	appMap := make(map[uint]*model.Application)
	for _, app := range applications {
		appMap[app.ID] = app
	}

	for i := range proxies {
		if app, exists := appMap[proxies[i].ApplicationID]; exists {
			proxies[i].Application = app
		} else {
			log.Warnf("application %d not found", proxies[i].ApplicationID)
		}
	}

	return &v1.ListProxiesResponse{
		Code:    200,
		Message: "success",
		Data: &v1.Proxies{
			Total:   int32(count),
			Proxies: cp.transformProxies(proxies),
		},
	}, nil
}

func (cp *controlPlane) UpdateProxy(_ context.Context, req *v1.UpdateProxyRequest) (*v1.UpdateProxyResponse, error) {
	if req.Id == 0 {
		return nil, badRequest("PROXY_ID_REQUIRED", "访问 ID 不能为空")
	}
	if req.Port < 0 || req.Port > 65535 {
		return nil, badRequest("PROXY_PORT_INVALID", "公网端口必须在 1-65535 之间")
	}
	proxy, err := cp.repo.GetProxyByID(uint(req.Id))
	if err != nil {
		return nil, mapRecordNotFound(err, "PROXY_NOT_FOUND", "访问不存在")
	}
	application, applicationErr := cp.repo.GetApplicationByID(proxy.ApplicationID)
	if applicationErr != nil {
		return nil, mapRecordNotFound(applicationErr, "APPLICATION_NOT_FOUND", "关联应用不存在")
	}

	oldProxy := *proxy
	if req.AccessProtocol != "" {
		protocol, protocolErr := normalizeAccessProtocol(req.AccessProtocol, application, int(req.Port), req.GetExposePublicPort())
		if protocolErr != nil {
			return nil, protocolErr
		}
		proxy.AccessProtocol = protocol
	} else if req.ExposePublicPort != nil && isWebOnlyCapableApplicationType(application.ApplicationType) {
		if req.GetExposePublicPort() {
			proxy.AccessProtocol = model.AccessProtocolTCP
		} else if application.ApplicationType == model.ApplicationTypeSSH {
			proxy.AccessProtocol = model.AccessProtocolWebSSH
		} else {
			proxy.AccessProtocol = model.AccessProtocolWeb
		}
	}

	if req.Name != "" {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			return nil, badRequest("PROXY_NAME_REQUIRED", "访问名称不能为空")
		}
		proxy.Name = name
	}
	if req.Description != "" {
		proxy.Description = req.Description
	}
	if req.ExposePublicPort != nil {
		port, err := cp.resolveUpdateProxyPort(effectiveAccessProtocol(proxy, application), proxy, int(req.Port), req.GetExposePublicPort())
		if err != nil {
			return nil, err
		}
		proxy.Port = port
	} else if req.AccessProtocol != "" && !accessProtocolRequiresPublicPort(effectiveAccessProtocol(proxy, application)) {
		proxy.Port = 0
	} else if req.AccessProtocol != "" && req.Port > 0 && int(req.Port) != proxy.Port {
		if err := cp.ensureProxyPortAvailable(int(req.Port), proxy.ID); err != nil {
			return nil, err
		}
		proxy.Port = int(req.Port)
	} else if req.AccessProtocol != "" && proxy.Port == 0 {
		port, allocateErr := cp.allocateAvailableProxyPort(proxy.ID)
		if allocateErr != nil {
			return nil, allocateErr
		}
		proxy.Port = port
	} else if req.Port > 0 && int(req.Port) != proxy.Port {
		if err := cp.ensureProxyPortAvailable(int(req.Port), proxy.ID); err != nil {
			return nil, err
		}
		proxy.Port = int(req.Port)
	}
	if req.Status != "" {
		switch req.Status {
		case "running":
			proxy.Status = model.ProxyStatusRunning
		case "stopped":
			proxy.Status = model.ProxyStatusStopped
		default:
			return nil, badRequest("PROXY_STATUS_INVALID", "访问状态无效")
		}
	}

	statusChanged := oldProxy.Status != proxy.Status
	portChanged := oldProxy.Port != proxy.Port
	protocolChanged := oldProxy.AccessProtocol != proxy.AccessProtocol
	runtimeChanged := statusChanged || portChanged || protocolChanged

	oldRuntimeEligible := false
	if oldProxy.Status == model.ProxyStatusRunning {
		oldRuntimeEligible, _ = cp.proxyRuntimeEligible(&oldProxy, application)
	}
	newRuntimeEligible := false
	if proxy.Status == model.ProxyStatusRunning {
		newRuntimeEligible, err = cp.proxyRuntimeEligible(proxy, application)
		if err != nil {
			return nil, err
		}
	}

	if runtimeChanged && oldProxy.Status == model.ProxyStatusRunning {
		if err := cp.stopProxyRuntime(&oldProxy); err != nil {
			log.Errorf("failed to stop proxy: %s", err)
			return nil, err
		}
	}

	startedNewRuntime := false
	if runtimeChanged && newRuntimeEligible {
		if err := cp.startProxyRuntime(proxy, application); err != nil {
			if oldRuntimeEligible {
				_ = cp.startProxyRuntime(&oldProxy, application)
			}
			log.Errorf("failed to start proxy: %s", err)
			return nil, err
		}
		startedNewRuntime = true
	}

	err = cp.repo.UpdateProxy(proxy)
	if err != nil {
		if startedNewRuntime {
			_ = cp.stopProxyRuntime(proxy)
		}
		if runtimeChanged && oldRuntimeEligible {
			_ = cp.startProxyRuntime(&oldProxy, application)
		}
		return nil, err
	}

	// 重新获取更新后的 proxy 以返回完整数据
	updatedProxy, err := cp.repo.GetProxyByID(uint(req.Id))
	if err != nil {
		return nil, err
	}
	if application == nil {
		application, _ = cp.repo.GetApplicationByID(updatedProxy.ApplicationID)
	}
	updatedProxy.Application = application

	return &v1.UpdateProxyResponse{
		Code:    200,
		Message: "success",
		Data:    cp.transformProxy(updatedProxy),
	}, nil
}

func (cp *controlPlane) DeleteProxy(_ context.Context, req *v1.DeleteProxyRequest) (*v1.DeleteProxyResponse, error) {
	if req.Id == 0 {
		return nil, badRequest("PROXY_ID_REQUIRED", "访问 ID 不能为空")
	}
	if _, err := cp.repo.GetProxyByID(uint(req.Id)); err != nil {
		return nil, mapRecordNotFound(err, "PROXY_NOT_FOUND", "访问不存在")
	}
	if err := cp.deleteProxyCascade(uint(req.Id), newLifecycleDeleteTracker()); err != nil {
		return nil, err
	}
	return &v1.DeleteProxyResponse{
		Code:    200,
		Message: "success",
	}, nil
}

func (cp *controlPlane) transformProxies(proxies []*model.Proxy) []*v1.Proxy {
	proxiesV1 := make([]*v1.Proxy, len(proxies))
	for i, proxy := range proxies {
		proxiesV1[i] = cp.transformProxy(proxy)
	}
	return proxiesV1
}

func (cp *controlPlane) transformProxy(proxy *model.Proxy) *v1.Proxy {
	var application *v1.Application
	if proxy.Application != nil {
		application = transformApplication(proxy.Application)
	}

	status := proxyStatusString(proxy.Status)
	effectiveStatus, effectiveStatusMessage := cp.proxyEffectiveStatus(proxy, proxy.Application)

	// 生成访问地址 —— server_url 形如 https://<host>[:<manager_port>]，
	// 这里只需要 host 部分,再拼 entry 自己的端口。
	var accessURL string
	if proxy.Application != nil && proxy.Port > 0 {
		serverURL := cp.conf.Manager.ServerURL
		if serverURL != "" {
			host := serverURL
			if u, err := url.Parse(serverURL); err == nil && u.Host != "" {
				host = u.Hostname() // 去掉 scheme + manager 端口
			} else {
				// 兜底:手动剥 scheme,再剥结尾 :port(只剥末尾,不影响 IPv6)
				host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
				if i := strings.LastIndex(host, ":"); i > strings.LastIndex(host, "]") {
					host = host[:i]
				}
			}
			if proxy.Application.ApplicationType == model.ApplicationTypeHTTP {
				accessURL = fmt.Sprintf("https://%s:%d", host, proxy.Port)
			} else {
				accessURL = fmt.Sprintf("%s:%d", host, proxy.Port)
			}
		}
	}

	return &v1.Proxy{
		Id:                     uint64(proxy.ID),
		Name:                   proxy.Name,
		Port:                   int32(proxy.Port),
		Status:                 status,
		Application:            application,
		Description:            proxy.Description,
		CreatedAt:              proxy.CreatedAt.Format(time.DateTime),
		UpdatedAt:              proxy.UpdatedAt.Format(time.DateTime),
		AccessUrl:              accessURL,
		EffectiveStatus:        effectiveStatus,
		EffectiveStatusMessage: effectiveStatusMessage,
		ExposePublicPort:       proxy.Port > 0,
		AccessProtocol:         string(effectiveAccessProtocol(proxy, proxy.Application)),
	}
}

func (cp *controlPlane) resolveCreateProxyPort(protocol model.AccessProtocol, requestedPort int) (int, error) {
	if requestedPort < 0 || requestedPort > 65535 {
		return 0, badRequest("PROXY_PORT_INVALID", "公网端口必须在 1-65535 之间")
	}
	if !accessProtocolRequiresPublicPort(protocol) {
		return 0, nil
	}
	if requestedPort > 0 {
		return requestedPort, nil
	}
	return cp.allocateAvailableProxyPort(0)
}

func (cp *controlPlane) resolveUpdateProxyPort(protocol model.AccessProtocol, proxy *model.Proxy, requestedPort int, exposePublicPort bool) (int, error) {
	if proxy == nil {
		return 0, notFound("PROXY_NOT_FOUND", "访问不存在", nil)
	}
	if requestedPort < 0 || requestedPort > 65535 {
		return 0, badRequest("PROXY_PORT_INVALID", "公网端口必须在 1-65535 之间")
	}
	if !exposePublicPort {
		if accessProtocolRequiresPublicPort(protocol) {
			return 0, badRequest("PROXY_PUBLIC_PORT_REQUIRED", "该访问协议必须开放公网端口")
		}
		return 0, nil
	}
	if requestedPort > 0 {
		if requestedPort != proxy.Port {
			if err := cp.ensureProxyPortAvailable(requestedPort, proxy.ID); err != nil {
				return 0, err
			}
		}
		return requestedPort, nil
	}
	if proxy.Port > 0 {
		return proxy.Port, nil
	}
	port, err := cp.allocateAvailableProxyPort(proxy.ID)
	if err != nil {
		return 0, err
	}
	return port, nil
}

func proxyStatusString(status model.ProxyStatus) string {
	switch status {
	case model.ProxyStatusRunning:
		return "running"
	case model.ProxyStatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}
