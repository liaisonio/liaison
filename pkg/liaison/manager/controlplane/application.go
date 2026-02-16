package controlplane

import (
	"context"
	"time"

	v1 "github.com/singchia/liaison/api/v1"
	"github.com/singchia/liaison/pkg/liaison/repo/dao"
	"github.com/singchia/liaison/pkg/liaison/repo/model"
)

// getDefaultPortByApplicationType 根据应用类型返回默认端口
func getDefaultPortByApplicationType(appType string) int {
	defaultPorts := map[string]int{
		"http":       80,
		"ssh":        22,
		"rdp":        3389,
		"mysql":      3306,
		"postgresql": 5432,
		"redis":      6379,
		"mongodb":    27017,
		"database":   3306,
	}
	if port, ok := defaultPorts[appType]; ok {
		return port
	}
	return 0
}

// detectApplicationTypeByPort 根据端口号推断应用类型
func detectApplicationTypeByPort(port int) string {
	portToType := map[int]string{
		22:    "ssh",
		80:    "http",
		443:   "http",
		3389:  "rdp",
		3306:  "mysql",
		5432:  "postgresql",
		6379:  "redis",
		27017: "mongodb",
	}
	if appType, ok := portToType[port]; ok {
		return appType
	}
	return "tcp" // 默认返回 tcp
}

func (cp *controlPlane) CreateApplication(_ context.Context, req *v1.CreateApplicationRequest) (*v1.CreateApplicationResponse, error) {
	// 验证 edge 是否存在
	_, err := cp.repo.GetEdge(req.EdgeId)
	if err != nil {
		return nil, err
	}

	// 根据应用的 IP 地址查找对应的 Device
	var deviceID uint
	if req.DeviceId != nil && *req.DeviceId > 0 {
		// 如果请求中指定了 device_id，优先使用
		deviceID = uint(*req.DeviceId)
	} else if req.Ip != "" {
		// 如果 IP 是 127.0.0.1，使用 edge 所在的 device
		if req.Ip == "127.0.0.1" || req.Ip == "::1" || req.Ip == "localhost" {
			// 获取 edge 所在的 device（通过 EdgeDevice 关系表，类型为 Host）
			hostType := model.EdgeDeviceRelationHost
			edgeDevices, err := cp.repo.GetEdgeDevicesByEdgeID(req.EdgeId, &hostType)
			if err == nil && len(edgeDevices) > 0 {
				deviceID = edgeDevices[0].DeviceID
			}
		} else {
			// 根据 IP 查找 Device
			device, err := cp.repo.GetDeviceByIP(req.Ip)
			if err == nil && device != nil {
				deviceID = uint(device.ID)
			}
			// 如果根据 IP 找不到 Device，deviceID 保持为 0
		}
	}

	// 处理应用类型和端口
	appType := req.ApplicationType
	port := int(req.Port)

	// 如果用户已经指定了应用类型，保持用户的选择，不根据端口推断
	// 只有当应用类型为空（未指定）时，才根据端口号推断应用类型
	if appType == "" && port > 0 {
		detectedType := detectApplicationTypeByPort(port)
		if detectedType != "" {
			appType = detectedType
		}
	}

	// 如果端口为空或0，根据应用类型设置默认端口
	if port == 0 && appType != "" {
		port = getDefaultPortByApplicationType(appType)
	}

	// 如果应用类型仍然为空，设置为 tcp
	if appType == "" {
		appType = "tcp"
	}

	// 注意如果edge id不在线，应用可能无法访问
	application := &model.Application{
		Name:            req.Name,
		Description:     req.Description,
		IP:              req.Ip,
		Port:            port,
		ApplicationType: model.ApplicationType(appType),
		EdgeIDs:         model.UintSlice{uint(req.EdgeId)},
		DeviceID:        deviceID,
	}
	err = cp.repo.CreateApplication(application)
	if err != nil {
		return nil, err
	}
	
	// 重新获取创建的应用，包含完整的关联数据
	createdApplication, err := cp.repo.GetApplicationByID(application.ID)
	if err != nil {
		return nil, err
	}
	
	return &v1.CreateApplicationResponse{
		Code:    200,
		Message: "success",
		Data:    transformApplication(createdApplication),
	}, nil
}

func (cp *controlPlane) ListApplications(_ context.Context, req *v1.ListApplicationsRequest) (*v1.ListApplicationsResponse, error) {
	var (
		deviceIDs       []uint
		devices         []*model.Device
		preDeviceSearch bool
		err             error
	)
	// 如果提供了设备名，先通过设备名查找设备ID和设备信息
	if req.DeviceName != nil && *req.DeviceName != "" {
		devices, err = cp.repo.ListDevices(&dao.ListDevicesQuery{
			Query: dao.Query{
				Order: "id",
				Desc:  true,
			},
			Name: *req.DeviceName,
		})
		if err != nil {
			return nil, err
		}
		if len(devices) > 0 {
			deviceIDs = make([]uint, len(devices))
			for i, device := range devices {
				deviceIDs[i] = device.ID
			}
			preDeviceSearch = true
			// 获取设备的网卡信息
			for _, device := range devices {
				interfaces, err := cp.repo.GetEthernetInterfacesByDeviceID(uint(device.ID))
				if err != nil {
					return nil, err
				}
				device.Interfaces = interfaces
			}
		} else {
			// 如果找不到设备，返回空列表
			return &v1.ListApplicationsResponse{
				Code:    200,
				Message: "success",
				Data: &v1.Applications{
					Total:        0,
					Applications: []*v1.Application{},
				},
			}, nil
		}
	} else if req.DeviceId != nil && *req.DeviceId > 0 {
		deviceIDs = []uint{uint(*req.DeviceId)}
	}

	query := &dao.ListApplicationsQuery{
		Query: dao.Query{
			Page:     int(req.Page),
			PageSize: int(req.PageSize),
			Order:    "id",
			Desc:     true,
		},
		DeviceIDs: deviceIDs,
	}
	// 应用类型筛选
	if req.ApplicationType != nil && *req.ApplicationType != "" {
		query.ApplicationType = *req.ApplicationType
	}
	// 应用名称筛选（如果提供了 application_name，需要在这里处理）
	// 注意：目前 DAO 层还没有实现 name 筛选，如果需要可以后续添加
	applications, err := cp.repo.ListApplications(query)
	if err != nil {
		return nil, err
	}

	// 如果没有提前搜索设备，则批量获取设备信息
	if !preDeviceSearch {
		deviceIDs := []uint{}
		deviceIDSet := make(map[uint]bool)
		for _, app := range applications {
			if app.DeviceID > 0 && !deviceIDSet[app.DeviceID] {
				deviceIDs = append(deviceIDs, app.DeviceID)
				deviceIDSet[app.DeviceID] = true
			}
		}
		if len(deviceIDs) > 0 {
			devices, err = cp.repo.ListDevices(&dao.ListDevicesQuery{
				Query: dao.Query{
					Order: "id",
					Desc:  true,
				},
				IDs: deviceIDs,
			})
			if err != nil {
				return nil, err
			}
			// 获取设备的网卡信息
			for _, device := range devices {
				interfaces, err := cp.repo.GetEthernetInterfacesByDeviceID(uint(device.ID))
				if err != nil {
					return nil, err
				}
				device.Interfaces = interfaces
			}
		}
	}

	// 批量获取Proxy信息
	applicationIDs := make([]uint, len(applications))
	for i, app := range applications {
		applicationIDs[i] = app.ID
	}
	proxies, err := cp.repo.ListProxies(&dao.ListProxiesQuery{
		ApplicationIDs: applicationIDs,
	})
	if err != nil {
		return nil, err
	}

	// 创建设备和Proxy映射，并关联到Application
	deviceMap := make(map[uint]*model.Device)
	for _, device := range devices {
		deviceMap[device.ID] = device
	}
	proxyMap := make(map[uint]*model.Proxy)
	for _, proxy := range proxies {
		proxyMap[proxy.ApplicationID] = proxy
	}

	// 将Device和Proxy关联到Application
	for _, app := range applications {
		if device, ok := deviceMap[app.DeviceID]; ok {
			app.Device = device
		}
		if proxy, ok := proxyMap[app.ID]; ok {
			app.Proxy = proxy
		}
	}

	countQuery := &dao.ListApplicationsQuery{
		DeviceIDs: deviceIDs,
	}
	// 应用类型筛选
	if req.ApplicationType != nil && *req.ApplicationType != "" {
		countQuery.ApplicationType = *req.ApplicationType
	}
	count, err := cp.repo.CountApplications(countQuery)
	if err != nil {
		return nil, err
	}
	return &v1.ListApplicationsResponse{
		Code:    200,
		Message: "success",
		Data: &v1.Applications{
			Total:        int32(count),
			Applications: transformApplications(applications),
		},
	}, nil
}

func (cp *controlPlane) UpdateApplication(_ context.Context, req *v1.UpdateApplicationRequest) (*v1.UpdateApplicationResponse, error) {
	application, err := cp.repo.GetApplicationByID(uint(req.Id))
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		application.Name = req.Name
	}
	if req.Description != "" {
		application.Description = req.Description
	}
	err = cp.repo.UpdateApplication(application)
	if err != nil {
		return nil, err
	}
	// 重新获取更新后的 application 以返回完整数据
	updatedApplication, err := cp.repo.GetApplicationByID(uint(req.Id))
	if err != nil {
		return nil, err
	}
	return &v1.UpdateApplicationResponse{
		Code:    200,
		Message: "success",
		Data:    transformApplication(updatedApplication),
	}, nil
}

func (cp *controlPlane) DeleteApplication(_ context.Context, req *v1.DeleteApplicationRequest) (*v1.DeleteApplicationResponse, error) {
	err := cp.repo.DeleteApplication(uint(req.Id))
	if err != nil {
		return nil, err
	}
	return &v1.DeleteApplicationResponse{
		Code:    200,
		Message: "success",
	}, nil
}

func transformApplications(applications []*model.Application) []*v1.Application {
	applicationsV1 := make([]*v1.Application, len(applications))
	for i, application := range applications {
		applicationsV1[i] = transformApplication(application)
	}
	return applicationsV1
}

func transformApplication(application *model.Application) *v1.Application {
	// 获取第一个 edge_id，因为目前一个应用只关联一个 edge
	var edgeId uint64
	if len(application.EdgeIDs) > 0 {
		edgeId = uint64(application.EdgeIDs[0])
	}

	appV1 := &v1.Application{
		Id:              uint64(application.ID),
		EdgeId:          edgeId,
		Name:            application.Name,
		Description:     application.Description,
		Ip:              application.IP,
		Port:            int32(application.Port),
		ApplicationType: string(application.ApplicationType),
		CreatedAt:       application.CreatedAt.Format(time.DateTime),
		UpdatedAt:       application.UpdatedAt.Format(time.DateTime),
	}

	// 填充设备信息
	if application.Device != nil {
		appV1.Device = transformDevice(application.Device)
	}

	// 填充Proxy信息（简化版，不包含Application以避免循环依赖）
	if application.Proxy != nil {
		// 将 ProxyStatus 转换为字符串
		var status string
		switch application.Proxy.Status {
		case model.ProxyStatusRunning:
			status = "running"
		case model.ProxyStatusStopped:
			status = "stopped"
		default:
			status = "unknown"
		}
		appV1.Proxy = &v1.Proxy{
			Id:          uint64(application.Proxy.ID),
			Name:        application.Proxy.Name,
			Port:        int32(application.Proxy.Port),
			Status:      status,
			Description: application.Proxy.Description,
			CreatedAt:   application.Proxy.CreatedAt.Format(time.DateTime),
			UpdatedAt:   application.Proxy.UpdatedAt.Format(time.DateTime),
			// Application字段留空，避免循环依赖
		}
	}

	return appV1
}
