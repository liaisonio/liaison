package controlplane

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	v1 "github.com/singchia/liaison/api/v1"
	"github.com/singchia/liaison/pkg/liaison/repo/dao"
	"github.com/singchia/liaison/pkg/liaison/repo/model"
	"github.com/singchia/liaison/pkg/proto"
)

func (cp *controlPlane) RegisterProxyManager(proxyManager proto.ProxyManager) {
	cp.proxyManager = proxyManager
}

func (cp *controlPlane) CreateProxy(_ context.Context, req *v1.CreateProxyRequest) (*v1.CreateProxyResponse, error) {
	// 查看是否有冲突
	// 获取application
	application, err := cp.repo.GetApplicationByID(uint(req.ApplicationId))
	if err != nil {
		log.Warnf("application %d not found", req.ApplicationId)
		return nil, err
	}

	// 如果端口为空或0，设置为0让系统自动分配
	requestedPort := int(req.Port)
	if requestedPort == 0 {
		requestedPort = 0 // 明确设置为0，让系统自动分配
	}

	// 创建Proxy持久化（如果端口为0，先保存0，后续会更新）
	proxy := &model.Proxy{
		Name:          req.Name,
		Status:        model.ProxyStatusRunning,
		Description:   req.Description,
		Port:          requestedPort,
		ApplicationID: uint(req.ApplicationId),
	}
	err = cp.repo.CreateProxy(proxy)
	if err != nil {
		log.Warnf("failed to create proxy: %s", err)
		return nil, err
	}

	// 创建Proxy（如果端口为0，系统会自动分配）
	// 如果是 HTTP 应用，默认使用 HTTPS
	useHTTPS := false
	if application.ApplicationType == model.ApplicationTypeHTTP {
		useHTTPS = true
	}

	protoproxy := &proto.Proxy{
		ID:              int(proxy.ID),
		Name:            proxy.Name,
		ProxyPort:       requestedPort, // 如果为0，系统会自动分配
		EdgeID:          uint64(application.EdgeIDs[0]),
		ApplicationID:   application.ID,
		Dst:             fmt.Sprintf("%s:%d", application.IP, application.Port),
		ApplicationType: string(application.ApplicationType),
		UseHTTPS:        useHTTPS,
	}
	err = cp.proxyManager.CreateProxy(context.Background(), protoproxy)
	if err != nil {
		log.Errorf("failed to create proxy listener: %s", err)
		// 如果创建失败，删除数据库记录
		_ = cp.repo.DeleteProxy(proxy.ID)
		return nil, err
	}

	// 如果端口为0，系统已经分配了端口，更新数据库中的端口
	if requestedPort == 0 {
		actualPort := protoproxy.ProxyPort
		if actualPort > 0 {
			proxy.Port = actualPort
			err = cp.repo.UpdateProxy(proxy)
			if err != nil {
				log.Errorf("failed to update proxy port: %s", err)
				// 即使更新失败，代理已经创建成功，继续返回成功
			} else {
				log.Infof("proxy %d: updated port to %d (system allocated)", proxy.ID, actualPort)
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
	ids := make([]uint, len(proxies))
	for i, proxy := range proxies {
		ids[i] = proxy.ApplicationID
	}
	// list applications
	applications, err := cp.repo.ListApplications(&dao.ListApplicationsQuery{
		Query: dao.Query{
			Page:     int(req.Page),
			PageSize: int(req.PageSize),
			Order:    "id",
			Desc:     true,
		},
		IDs: ids,
	})
	if err != nil {
		return nil, err
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

// 更新代理，不允许更新代理端口
func (cp *controlPlane) UpdateProxy(_ context.Context, req *v1.UpdateProxyRequest) (*v1.UpdateProxyResponse, error) {
	proxy, err := cp.repo.GetProxyByID(uint(req.Id))
	if err != nil {
		return nil, err
	}

	// 保存旧状态
	oldStatus := proxy.Status

	// 更新名称
	if req.Name != "" {
		proxy.Name = req.Name
	}

	// 更新描述
	if req.Description != "" {
		proxy.Description = req.Description
	}

	// 更新状态
	if req.Status != "" {
		switch req.Status {
		case "running":
			proxy.Status = model.ProxyStatusRunning
		case "stopped":
			proxy.Status = model.ProxyStatusStopped
		default:
			log.Warnf("unknown proxy status: %s", req.Status)
		}
	}

	// 如果状态发生变化，需要调用 ProxyManager
	if oldStatus != proxy.Status {
		// 获取 application 信息（用于启动代理时）
		application, err := cp.repo.GetApplicationByID(proxy.ApplicationID)
		if err != nil {
			log.Warnf("application %d not found", proxy.ApplicationID)
			return nil, err
		}

		if proxy.Status == model.ProxyStatusStopped {
			// 停止代理：调用 DeleteProxy（这会停止代理但不会删除数据库记录）
			err = cp.proxyManager.DeleteProxy(context.Background(), int(proxy.ID))
			if err != nil {
				log.Errorf("failed to stop proxy: %s", err)
				return nil, err
			}
		} else if proxy.Status == model.ProxyStatusRunning {
			// 启动代理：调用 CreateProxy
			// 如果是 HTTP 应用，默认使用 HTTPS
			useHTTPS := false
			if application.ApplicationType == model.ApplicationTypeHTTP {
				useHTTPS = true
			}
			err = cp.proxyManager.CreateProxy(context.Background(), &proto.Proxy{
				ID:              int(proxy.ID),
				Name:            proxy.Name,
				ProxyPort:       proxy.Port,
				EdgeID:          uint64(application.EdgeIDs[0]),
				Dst:             fmt.Sprintf("%s:%d", application.IP, application.Port),
				ApplicationType: string(application.ApplicationType),
				UseHTTPS:        useHTTPS,
			})
			if err != nil {
				log.Errorf("failed to start proxy: %s", err)
				return nil, err
			}
		}
	}

	// 更新数据库
	err = cp.repo.UpdateProxy(proxy)
	if err != nil {
		return nil, err
	}

	// 重新获取更新后的 proxy 以返回完整数据
	updatedProxy, err := cp.repo.GetProxyByID(uint(req.Id))
	if err != nil {
		return nil, err
	}

	return &v1.UpdateProxyResponse{
		Code:    200,
		Message: "success",
		Data:    cp.transformProxy(updatedProxy),
	}, nil
}

func (cp *controlPlane) DeleteProxy(_ context.Context, req *v1.DeleteProxyRequest) (*v1.DeleteProxyResponse, error) {
	err := cp.repo.DeleteProxy(uint(req.Id))
	if err != nil {
		return nil, err
	}
	// 删除正在工作的代理
	err = cp.proxyManager.DeleteProxy(context.Background(), int(req.Id))
	if err != nil {
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

	// 将 ProxyStatus 转换为字符串
	var status string
	switch proxy.Status {
	case model.ProxyStatusRunning:
		status = "running"
	case model.ProxyStatusStopped:
		status = "stopped"
	default:
		status = "unknown"
	}

	// 生成访问地址
	var accessURL string
	if proxy.Application != nil && proxy.Port > 0 {
		serverURL := cp.conf.Manager.ServerURL
		if serverURL != "" {
			if proxy.Application.ApplicationType == model.ApplicationTypeHTTP {
				// HTTP: server_url + 端口
				accessURL = fmt.Sprintf("%s:%d", serverURL, proxy.Port)
			} else {
				// TCP: server_url去掉https://，加上端口
				url := serverURL
				if strings.HasPrefix(url, "https://") {
					url = strings.TrimPrefix(url, "https://")
				} else if strings.HasPrefix(url, "http://") {
					url = strings.TrimPrefix(url, "http://")
				}
				accessURL = fmt.Sprintf("%s:%d", url, proxy.Port)
			}
		}
	}

	return &v1.Proxy{
		Id:          uint64(proxy.ID),
		Name:        proxy.Name,
		Port:        int32(proxy.Port),
		Status:      status,
		Application: application,
		Description: proxy.Description,
		CreatedAt:   proxy.CreatedAt.Format(time.DateTime),
		UpdatedAt:   proxy.UpdatedAt.Format(time.DateTime),
		AccessUrl:   accessURL,
	}
}
