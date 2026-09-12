package controlplane

import (
	"context"
	"net"
	"strings"
	"time"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

// getDefaultPortByApplicationType 根据应用类型返回默认端口
func getDefaultPortByApplicationType(appType string) int {
	defaultPorts := map[string]int{
		"http":          80,
		"ssh":           22,
		"rdp":           3389,
		"vnc":           5900,
		"mysql":         3306,
		"mariadb":       3306,
		"sqlserver":     1433,
		"oracle":        1521,
		"clickhouse":    9000,
		"elasticsearch": 9200,
		"opensearch":    9200,
		"postgresql":    5432,
		"redis":         6379,
		"mongodb":       27017,
		"database":      3306,
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
		5900:  "vnc",
		3306:  "mysql",
		5432:  "postgresql",
		1433:  "sqlserver",
		1521:  "oracle",
		9000:  "clickhouse",
		6379:  "redis",
		27017: "mongodb",
	}
	if appType, ok := portToType[port]; ok {
		return appType
	}
	return "tcp" // 默认返回 tcp
}

func (cp *controlPlane) CreateApplication(ctx context.Context, req *v1.CreateApplicationRequest) (*v1.CreateApplicationResponse, error) {
	name, err := normalizeResourceName(req.Name, "App")
	if err != nil {
		return nil, err
	}
	ip := strings.TrimSpace(req.Ip)
	if !isValidApplicationHost(ip) {
		return nil, badRequest("APPLICATION_IP_INVALID", "请输入合法的应用 IPv4、localhost 或主机名")
	}
	if req.EdgeId == 0 {
		return nil, badRequest("EDGE_ID_REQUIRED", "连接器不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceConnector, req.EdgeId); err != nil {
		return nil, err
	}
	// 验证 edge 是否存在
	_, err = cp.repo.GetEdge(req.EdgeId)
	if err != nil {
		return nil, mapRecordNotFound(err, "EDGE_NOT_FOUND", "连接器不存在")
	}

	// 根据应用的 IP 地址查找对应的 Device
	var deviceID uint
	if req.DeviceId != nil && *req.DeviceId > 0 {
		// 如果请求中指定了 device_id，优先使用
		deviceID = uint(*req.DeviceId)
		if err := requireVisibleResource(ctx, cp.repo, resourceDevice, uint64(deviceID)); err != nil {
			return nil, err
		}
		if _, err := cp.repo.GetDeviceByID(deviceID); err != nil {
			return nil, mapRecordNotFound(err, "DEVICE_NOT_FOUND", "设备不存在")
		}
	} else if ip != "" {
		// 如果 IP 是 127.0.0.1，使用 edge 所在的 device
		if ip == "127.0.0.1" || ip == "::1" || strings.EqualFold(ip, "localhost") {
			// 获取 edge 所在的 device（通过 EdgeDevice 关系表，类型为 Host）
			hostType := model.EdgeDeviceRelationHost
			edgeDevices, err := cp.repo.GetEdgeDevicesByEdgeID(req.EdgeId, &hostType)
			if err == nil && len(edgeDevices) > 0 {
				deviceID = edgeDevices[0].DeviceID
			}
		} else {
			// 根据 IP 查找 Device
			device, err := cp.repo.GetDeviceByIP(ip)
			if err == nil && device != nil {
				deviceID = uint(device.ID)
			} else if err != nil && !isRecordNotFound(err) {
				return nil, err
			}
			// 如果根据 IP 找不到 Device，deviceID 保持为 0
		}
	}

	// 处理应用类型和端口
	appType := strings.ToLower(strings.TrimSpace(req.ApplicationType))
	port := int(req.Port)
	if port < 0 || port > 65535 {
		return nil, badRequest("APPLICATION_PORT_INVALID", "应用端口必须在 1-65535 之间")
	}

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
	if !isAllowedApplicationType(appType) {
		return nil, badRequest("APPLICATION_TYPE_INVALID", "应用类型无效")
	}
	if port < 1 || port > 65535 {
		return nil, badRequest("APPLICATION_PORT_INVALID", "应用端口必须在 1-65535 之间")
	}

	// 注意如果edge id不在线，应用可能无法访问
	application := &model.Application{
		Name:            name,
		Description:     req.Description,
		IP:              ip,
		Port:            port,
		ApplicationType: model.ApplicationType(appType),
		EdgeIDs:         model.UintSlice{uint(req.EdgeId)},
		DeviceID:        deviceID,
	}
	err = cp.repo.CreateApplication(application)
	if err != nil {
		return nil, err
	}
	if err := claimResource(ctx, cp.repo, resourceApplication, uint64(application.ID)); err != nil {
		_ = cp.repo.DeleteApplication(application.ID)
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

func (cp *controlPlane) ListApplications(ctx context.Context, req *v1.ListApplicationsRequest) (*v1.ListApplicationsResponse, error) {
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
	visibleIDs, scoped, err := visibleResourceIDs(ctx, cp.repo, resourceApplication)
	if err != nil {
		return nil, err
	}
	if scoped {
		query.ScopeApplied = true
		query.IDs = make([]uint, len(visibleIDs))
		for i, id := range visibleIDs {
			query.IDs[i] = uint(id)
		}
	}
	// 应用类型筛选
	if req.ApplicationType != nil && *req.ApplicationType != "" {
		query.ApplicationType = *req.ApplicationType
	}
	if req.ApplicationName != nil {
		query.Name = *req.ApplicationName
	}
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
	var proxies []*model.Proxy
	if len(applicationIDs) > 0 {
		proxyQuery := &dao.ListProxiesQuery{
			ApplicationIDs: applicationIDs,
		}
		visibleProxyIDs, proxyScoped, scopeErr := visibleResourceIDs(ctx, cp.repo, resourceAccess)
		if scopeErr != nil {
			return nil, scopeErr
		}
		if proxyScoped {
			proxyQuery.ScopeApplied = true
			proxyQuery.IDs = make([]uint, len(visibleProxyIDs))
			for i, id := range visibleProxyIDs {
				proxyQuery.IDs[i] = uint(id)
			}
		}
		proxies, err = cp.repo.ListProxies(proxyQuery)
		if err != nil {
			return nil, err
		}
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
		Name:      query.Name,
		Query:     dao.Query{ScopeApplied: query.ScopeApplied},
		DeviceIDs: deviceIDs,
		IDs:       query.IDs,
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

func (cp *controlPlane) UpdateApplication(ctx context.Context, req *v1.UpdateApplicationRequest) (*v1.UpdateApplicationResponse, error) {
	if req.Id == 0 {
		return nil, badRequest("APPLICATION_ID_REQUIRED", "应用 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceApplication, req.Id); err != nil {
		return nil, err
	}
	application, err := cp.repo.GetApplicationByID(uint(req.Id))
	if err != nil {
		return nil, mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "应用不存在")
	}
	if req.Name != "" {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			return nil, badRequest("APPLICATION_NAME_REQUIRED", "应用名称不能为空")
		}
		application.Name = name
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

func (cp *controlPlane) DeleteApplication(ctx context.Context, req *v1.DeleteApplicationRequest) (*v1.DeleteApplicationResponse, error) {
	if req.Id == 0 {
		return nil, badRequest("APPLICATION_ID_REQUIRED", "应用 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceApplication, req.Id); err != nil {
		return nil, err
	}
	if _, err := cp.repo.GetApplicationByID(uint(req.Id)); err != nil {
		return nil, mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "应用不存在")
	}
	if err := cp.deleteApplicationCascade(uint(req.Id), newLifecycleDeleteTracker()); err != nil {
		return nil, err
	}
	if err := cp.repo.DeleteIAMResourceRelations(resourceApplication, req.Id); err != nil {
		return nil, err
	}
	return &v1.DeleteApplicationResponse{
		Code:    200,
		Message: "success",
	}, nil
}

func isAllowedApplicationType(appType string) bool {
	switch appType {
	case "llm", "http", "tcp", "ssh", "rdp", "vnc", "mysql", "mariadb", "sqlserver", "oracle", "clickhouse", "elasticsearch", "opensearch", "postgresql", "redis", "mongodb", "database":
		return true
	default:
		return false
	}
}

func isValidApplicationHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, "/ ") {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.To4() != nil
	}
	if looksLikeIPv4(host) {
		return false
	}
	if len(host) > 253 {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		for i, r := range label {
			valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-'
			if !valid {
				return false
			}
			if (i == 0 || i == len(label)-1) && r == '-' {
				return false
			}
		}
	}
	return true
}

func looksLikeIPv4(host string) bool {
	labels := strings.Split(host, ".")
	if len(labels) != 4 {
		return false
	}
	for _, label := range labels {
		if label == "" {
			return false
		}
		for _, r := range label {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
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
		appV1.Proxy = &v1.Proxy{
			Id:          uint64(application.Proxy.ID),
			Name:        application.Proxy.Name,
			Port:        int32(application.Proxy.Port),
			Status:      proxyStatusString(application.Proxy.Status),
			Description: application.Proxy.Description,
			CreatedAt:   application.Proxy.CreatedAt.Format(time.DateTime),
			UpdatedAt:   application.Proxy.UpdatedAt.Format(time.DateTime),
			// Application字段留空，避免循环依赖
		}
	}

	return appV1
}
