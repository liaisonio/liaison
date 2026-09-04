package controlplane

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func (cp *controlPlane) CreateEdge(ctx context.Context, req *v1.CreateEdgeRequest) (*v1.CreateEdgeResponse, error) {
	name, err := normalizeResourceName(req.Name, "Connector")
	if err != nil {
		return nil, err
	}
	// 在事务中创建edge和ak/sk
	tx := cp.repo.Begin()

	edge := &model.Edge{
		Name:        name,
		Description: req.Description,
		Status:      model.EdgeStatusRunning, // 默认状态为运行中
		Online:      model.EdgeOnlineStatusOffline,
	}

	err = tx.CreateEdge(edge)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := claimResource(ctx, tx, resourceConnector, uint64(edge.ID)); err != nil {
		tx.Rollback()
		return nil, err
	}

	// 生成 AK/SK
	accessKey, secretKey := generateAccessKeyPair()

	err = tx.CreateAccessKey(&model.AccessKey{
		EdgeID:    edge.ID,
		AccessKey: accessKey,
		SecretKey: secretKey,
	})
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// 提交事务
	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	// 生成安装命令
	serverURL := cp.conf.Manager.ServerURL
	httpAddr := "" // HTTP下载地址（host:port）
	edgeAddr := "" // Edge连接地址（host:port）

	if serverURL == "" {
		// 如果没有配置，从 Listen 地址生成
		listen := cp.conf.Manager.Listen
		if listen.TLS.Enable {
			serverURL = fmt.Sprintf("https://%s", listen.Addr)
		} else {
			serverURL = fmt.Sprintf("http://%s", listen.Addr)
		}
		httpAddr = listen.Addr
	} else {
		// 从 serverURL 中提取地址（移除 http:// 或 https:// 前缀）
		if strings.HasPrefix(serverURL, "https://") {
			httpAddr = strings.TrimPrefix(serverURL, "https://")
		} else if strings.HasPrefix(serverURL, "http://") {
			httpAddr = strings.TrimPrefix(serverURL, "http://")
		} else {
			httpAddr = serverURL
		}
	}

	// Edge连接地址（使用FrontierEdgePort端口）
	edgePort := cp.conf.Manager.FrontierEdgePort
	if edgePort == 0 {
		edgePort = 30012
	}
	// 从httpAddr中提取host（如果包含端口，去掉端口）
	httpHost := httpAddr
	if strings.Contains(httpAddr, ":") {
		httpHost = strings.Split(httpAddr, ":")[0]
	}
	edgeAddr = fmt.Sprintf("%s:%d", httpHost, edgePort)

	installCommand := fmt.Sprintf("curl -k -sSL %s/install.sh | bash -s -- --access-key=%s --secret-key=%s --server-http-addr=%s --server-edge-addr=%s",
		serverURL, accessKey, secretKey, httpAddr, edgeAddr)

	// 返回响应
	return &v1.CreateEdgeResponse{
		Code:    200,
		Message: "success",
		Data: &v1.AccessKey{
			AccessKey: accessKey,
			SecretKey: secretKey,
			Command:   installCommand,
		},
	}, nil
}

func (cp *controlPlane) GetEdge(ctx context.Context, req *v1.GetEdgeRequest) (*v1.GetEdgeResponse, error) {
	if req.Id == 0 {
		return nil, badRequest("EDGE_ID_REQUIRED", "连接器 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceConnector, req.Id); err != nil {
		return nil, err
	}
	edge, err := cp.repo.GetEdge(req.Id)
	if err != nil {
		return nil, mapRecordNotFound(err, "EDGE_NOT_FOUND", "连接器不存在")
	}
	// 通过 EdgeDevice 关系表获取关联的 Device（Host 类型）
	hostType := model.EdgeDeviceRelationHost
	edgeDevices, err := cp.repo.GetEdgeDevicesByEdgeID(req.Id, &hostType)
	if err == nil && len(edgeDevices) > 0 {
		device, err := cp.repo.GetDeviceByID(edgeDevices[0].DeviceID)
		if err == nil && device != nil {
			edge.Device = device
		}
	}
	return &v1.GetEdgeResponse{
		Code:    200,
		Message: "success",
		Data: &v1.Edge{
			Id:          uint64(edge.ID),
			Name:        edge.Name,
			Description: edge.Description,
			Status:      int32(edge.Status),
			Online:      int32(edge.Online),
			CreatedAt:   edge.CreatedAt.Format(time.DateTime),
			UpdatedAt:   edge.UpdatedAt.Format(time.DateTime),
		},
	}, nil
}

func (cp *controlPlane) ListEdges(ctx context.Context, req *v1.ListEdgesRequest) (*v1.ListEdgesResponse, error) {
	var (
		edgeIDs []uint64
		err     error
	)
	// 如果指定了设备名，先查找设备，然后通过 EdgeDevice 关系表查找关联的 Edge
	if req.DeviceName != "" {
		devices, err := cp.repo.ListDevices(&dao.ListDevicesQuery{
			Query: dao.Query{
				Order: "id",
				Desc:  true,
			},
			Name: req.DeviceName,
		})
		if err != nil {
			return nil, err
		}
		// 如果指定了设备名但搜不到设备，直接返回空结果
		if len(devices) == 0 {
			return &v1.ListEdgesResponse{
				Code:    200,
				Message: "success",
				Data: &v1.Edges{
					Total: 0,
					Edges: []*v1.Edge{},
				},
			}, nil
		}
		// 通过 EdgeDevice 关系表查找关联的 Edge（Host 类型）
		hostType := model.EdgeDeviceRelationHost
		for _, device := range devices {
			edgeDevices, err := cp.repo.GetEdgeDevicesByDeviceID(device.ID, &hostType)
			if err != nil {
				return nil, err
			}
			for _, edgeDevice := range edgeDevices {
				edgeIDs = append(edgeIDs, edgeDevice.EdgeID)
			}
		}
		// 如果没有找到关联的 Edge，直接返回空结果
		if len(edgeIDs) == 0 {
			return &v1.ListEdgesResponse{
				Code:    200,
				Message: "success",
				Data: &v1.Edges{
					Total: 0,
					Edges: []*v1.Edge{},
				},
			}, nil
		}
	}

	query := &dao.ListEdgesQuery{
		Query: dao.Query{
			Page:     int(req.Page),
			PageSize: int(req.PageSize),
			Order:    "id",
			Desc:     true,
		},
	}
	visibleIDs, scoped, err := visibleResourceIDs(ctx, cp.repo, resourceConnector)
	if err != nil {
		return nil, err
	}
	if scoped {
		query.ScopeApplied = true
		if len(edgeIDs) > 0 {
			query.EdgeIDs = intersectResourceIDs(edgeIDs, visibleIDs)
		} else {
			query.EdgeIDs = visibleIDs
		}
	}
	// 如果通过设备名找到了 Edge IDs，使用这些 IDs 过滤
	if !scoped && len(edgeIDs) > 0 {
		query.EdgeIDs = edgeIDs
	}
	if req.Name != "" {
		query.Name = req.Name
	}
	// 搜索Edges
	edges, err := cp.repo.ListEdges(query)
	if err != nil {
		return nil, err
	}

	// 通过 EdgeDevice 关系表获取所有 Edge 关联的 Device（Host 类型）
	edgeIDList := make([]uint64, 0, len(edges))
	for _, edge := range edges {
		edgeIDList = append(edgeIDList, uint64(edge.ID))
	}

	// 批量查询 EdgeDevice 关系
	hostType := model.EdgeDeviceRelationHost
	deviceIDSet := make(map[uint]bool)
	edgeDeviceMap := make(map[uint64]uint) // edgeID -> deviceID
	for _, edgeID := range edgeIDList {
		edgeDevices, err := cp.repo.GetEdgeDevicesByEdgeID(edgeID, &hostType)
		if err != nil {
			return nil, err
		}
		for _, edgeDevice := range edgeDevices {
			edgeDeviceMap[edgeID] = edgeDevice.DeviceID
			if !deviceIDSet[edgeDevice.DeviceID] {
				deviceIDSet[edgeDevice.DeviceID] = true
			}
		}
	}

	// 查询所有相关的 Device
	var devices []*model.Device
	if len(deviceIDSet) > 0 {
		deviceIDs := make([]uint, 0, len(deviceIDSet))
		for deviceID := range deviceIDSet {
			deviceIDs = append(deviceIDs, deviceID)
		}
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
	}

	// 创建设备映射，通过 ID 匹配
	deviceMap := make(map[uint]*model.Device)
	for _, device := range devices {
		deviceMap[device.ID] = device
	}
	// 关联设备到 edge
	for _, edge := range edges {
		if deviceID, ok := edgeDeviceMap[uint64(edge.ID)]; ok {
			if device, ok := deviceMap[deviceID]; ok {
				edge.Device = device
			}
		}
	}

	count, err := cp.repo.CountEdges(query)
	if err != nil {
		return nil, err
	}
	return &v1.ListEdgesResponse{
		Code:    200,
		Message: "success",
		Data: &v1.Edges{
			Total: int32(count),
			Edges: transformEdges(edges),
		},
	}, nil
}

func (cp *controlPlane) UpdateEdge(ctx context.Context, req *v1.UpdateEdgeRequest) (*v1.UpdateEdgeResponse, error) {
	if req.Id == 0 {
		return nil, badRequest("EDGE_ID_REQUIRED", "连接器 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceConnector, req.Id); err != nil {
		return nil, err
	}
	edge, err := cp.repo.GetEdge(req.Id)
	if err != nil {
		return nil, mapRecordNotFound(err, "EDGE_NOT_FOUND", "连接器不存在")
	}
	oldStatus := edge.Status
	if req.Name != "" {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			return nil, badRequest("EDGE_NAME_REQUIRED", "连接器名称不能为空")
		}
		edge.Name = name
	}
	if req.Description != "" {
		edge.Description = req.Description
	}
	if req.Status != 0 {
		switch model.EdgeStatus(req.Status) {
		case model.EdgeStatusRunning, model.EdgeStatusStopped:
			edge.Status = model.EdgeStatus(req.Status)
		default:
			return nil, badRequest("EDGE_STATUS_INVALID", "连接器运行状态无效")
		}
	}
	statusChanged := oldStatus != edge.Status
	if statusChanged && edge.Status == model.EdgeStatusStopped {
		if err := cp.stopEdgeProxies(uint64(edge.ID)); err != nil {
			return nil, err
		}
		if err := cp.failActiveEdgeTasks(uint64(edge.ID), "edge stopped"); err != nil {
			return nil, err
		}
	}
	err = cp.repo.UpdateEdge(edge)
	if err != nil {
		return nil, err
	}
	if statusChanged && edge.Status == model.EdgeStatusRunning {
		if err := cp.startEdgeProxies(uint64(edge.ID)); err != nil {
			edge.Status = oldStatus
			_ = cp.repo.UpdateEdge(edge)
			return nil, err
		}
	}
	// 重新获取更新后的 edge 以返回完整数据
	updatedEdge, err := cp.repo.GetEdge(req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.UpdateEdgeResponse{
		Code:    200,
		Message: "success",
		Data:    transformEdge(updatedEdge),
	}, nil
}

func (cp *controlPlane) DeleteEdge(ctx context.Context, req *v1.DeleteEdgeRequest) (*v1.DeleteEdgeResponse, error) {
	if req.Id == 0 {
		return nil, badRequest("EDGE_ID_REQUIRED", "连接器 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceConnector, req.Id); err != nil {
		return nil, err
	}
	if _, err := cp.repo.GetEdge(req.Id); err != nil {
		return nil, mapRecordNotFound(err, "EDGE_NOT_FOUND", "连接器不存在")
	}
	if err := cp.deleteEdgeCascade(req.Id, newLifecycleDeleteTracker()); err != nil {
		return nil, err
	}
	if err := cp.repo.DeleteIAMResourceRelations(resourceConnector, req.Id); err != nil {
		return nil, err
	}
	return &v1.DeleteEdgeResponse{
		Code:    200,
		Message: "success",
	}, nil
}

func (cp *controlPlane) CreateEdgeScanApplicationTask(ctx context.Context, req *v1.CreateEdgeScanApplicationTaskRequest) (*v1.CreateEdgeScanApplicationTaskResponse, error) {
	if req.EdgeId == 0 {
		return nil, badRequest("EDGE_ID_REQUIRED", "连接器 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceConnector, req.EdgeId); err != nil {
		return nil, err
	}
	if req.Port < 0 || req.Port > 65535 {
		return nil, badRequest("SCAN_PORT_INVALID", "扫描端口必须在 1-65535 之间，或留空扫描常用端口")
	}
	req.Protocol = strings.ToLower(strings.TrimSpace(req.Protocol))
	if req.Protocol == "" {
		req.Protocol = "tcp"
	}
	if req.Protocol != "tcp" && req.Protocol != "udp" {
		return nil, badRequest("SCAN_PROTOCOL_INVALID", "扫描协议仅支持 tcp 或 udp")
	}
	// 获取edge
	edge, err := cp.repo.GetEdge(req.EdgeId)
	if err != nil {
		log.Errorf("get edge error: %s", err)
		return nil, mapRecordNotFound(err, "EDGE_NOT_FOUND", "连接器不存在")
	}
	if edge.Status == model.EdgeStatusStopped {
		log.Errorf("edge is stopped")
		return nil, preconditionFailed("EDGE_STOPPED", "连接器已禁用，无法扫描应用")
	}
	if edge.Online != model.EdgeOnlineStatusOnline {
		log.Errorf("edge is not online")
		return nil, preconditionFailed("EDGE_OFFLINE", "连接器离线，无法扫描应用")
	}

	// 获取设备（通过 EdgeDevice 关系表，Host 类型）
	hostType := model.EdgeDeviceRelationHost
	edgeDevices, err := cp.repo.GetEdgeDevicesByEdgeID(req.EdgeId, &hostType)
	if err != nil || len(edgeDevices) == 0 {
		log.Errorf("get edge device relation error: %s", err)
		return nil, conflict("EDGE_DEVICE_MISSING", "连接器未关联设备，无法扫描应用")
	}
	device, err := cp.repo.GetDeviceByID(edgeDevices[0].DeviceID)
	if err != nil {
		log.Errorf("get device error: %s", err)
		return nil, mapRecordNotFound(err, "DEVICE_NOT_FOUND", "连接器关联设备不存在")
	}
	nets := []string{}
	for _, iface := range device.Interfaces {
		// 获取IP地址所在网段
		//ip := net.ParseIP(iface.IP)
		//if ip == nil {
		//	continue
		//}
		//nets = append(nets, fmt.Sprintf("%s/%s", ip.Mask(net.IPMask(iface.Netmask)), iface.Netmask))
		nets = append(nets, iface.IP)
	}

	// 创建任务
	params := model.TaskScanApplicationParams{
		Nets:     nets,
		Port:     int(req.Port),
		Protocol: req.Protocol,
	}
	data, err := json.Marshal(params)
	if err != nil {
		log.Errorf("marshal params error: %s", err)
		return nil, err
	}
	expiration := 10 * time.Minute
	task := &model.Task{
		EdgeID:      req.EdgeId,
		TaskType:    model.TaskTypeScan,
		TaskSubType: model.TaskSubTypeScanApplication,
		TaskStatus:  model.TaskStatusPending,
		ExpiredAt:   time.Now().Add(expiration), // 10分钟过期
		TaskParams:  data,
		TaskResult:  []byte(`{"scanned_applications":[]}`), // 初始化为空结果
		Error:       "",                                    // 初始化为空错误
	}
	err = cp.repo.CreateTask(task)
	if err != nil {
		log.Errorf("create task error: %s", err)
		return nil, err
	}
	go func() {
		time.Sleep(expiration)
		task, err := cp.repo.GetTask(task.ID)
		if err != nil {
			log.Errorf("get task error: %s", err)
			return
		}
		switch task.TaskStatus {
		case model.TaskStatusPending:
			err = cp.repo.UpdateTaskError(task.ID, "task expired")
			if err != nil {
				log.Errorf("update task error: %s", err)
			}
		case model.TaskStatusRunning:
			err = cp.repo.UpdateTaskError(task.ID, "task expired")
			if err != nil {
				log.Errorf("update task error: %s", err)
			}
		}
	}()

	// 下发扫描任务
	err = cp.frontierBound.EmitScanApplications(context.Background(), task.ID, req.EdgeId, &frontierbound.Net{
		Nets:     nets,
		Protocol: req.Protocol,
		Port:     int(req.Port),
	})
	if err != nil {
		err = cp.repo.UpdateTaskError(task.ID, err.Error())
		if err != nil {
			log.Errorf("update task error: %s", err)
		}
		return nil, err
	}
	return &v1.CreateEdgeScanApplicationTaskResponse{
		Code:    200,
		Message: "success",
		Data: &v1.EdgeScanApplicationTaskCreated{
			TaskId: uint64(task.ID),
		},
	}, nil
}

func (cp *controlPlane) GetEdgeScanApplicationTask(ctx context.Context, req *v1.GetEdgeScanApplicationTaskRequest) (*v1.GetEdgeScanApplicationTaskResponse, error) {
	if req.EdgeId == 0 {
		return nil, badRequest("EDGE_ID_REQUIRED", "连接器 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceConnector, req.EdgeId); err != nil {
		return nil, err
	}
	var task *model.Task
	if req.TaskId != 0 {
		var err error
		task, err = cp.repo.GetTask(uint(req.TaskId))
		if err != nil {
			return nil, mapRecordNotFound(err, "SCAN_TASK_NOT_FOUND", "扫描任务不存在")
		}
		if task.EdgeID != req.EdgeId || task.TaskType != model.TaskTypeScan || task.TaskSubType != model.TaskSubTypeScanApplication {
			return nil, notFound("SCAN_TASK_NOT_FOUND", "扫描任务不存在", nil)
		}
	} else {
		tasks, err := cp.repo.ListTasks(&dao.ListTasksQuery{
			EdgeID:      uint(req.EdgeId),
			TaskType:    model.TaskTypeScan,
			TaskSubType: model.TaskSubTypeScanApplication,
			Query: dao.Query{
				Page:     1,
				PageSize: 1,
				Order:    "id",
				Desc:     true,
			},
		})
		if err != nil {
			log.Errorf("list tasks error: %s", err)
			return nil, err
		}
		if len(tasks) != 0 {
			task = tasks[0]
		}
	}
	if task == nil {
		return &v1.GetEdgeScanApplicationTaskResponse{
			Code:    200,
			Message: "success",
			Data:    nil,
		}, nil
	}

	// result
	result := model.TaskScanApplicationResult{}
	err := json.Unmarshal(task.TaskResult, &result)
	if err != nil {
		log.Errorf("unmarshal task result error: %s", err)
		return nil, err
	}
	// 根据端口号推断应用类型
	detectApplicationTypeByPort := func(port int) string {
		portToType := map[int]string{
			22:    "ssh",
			80:    "http",
			443:   "http",
			3389:  "rdp",
			5900:  "vnc",
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

	applications := []string{}
	for _, application := range result.ScannedApplications {
		// 如果 protocol 是 http，直接使用 http
		appType := application.Protocol
		if appType == "" || appType == "tcp" {
			// 根据端口推断应用类型
			appType = detectApplicationTypeByPort(application.Port)
		}
		applications = append(applications, fmt.Sprintf("%s:%d:%s", application.IP, application.Port, appType))
	}

	// 返回
	return &v1.GetEdgeScanApplicationTaskResponse{
		Code:    200,
		Message: "success",
		Data: &v1.EdgeScanApplicationTask{
			Id:           uint64(task.ID),
			EdgeId:       uint64(task.EdgeID),
			TaskStatus:   task.TaskStatus.String(),
			CreatedAt:    task.CreatedAt.Format(time.DateTime),
			UpdatedAt:    task.UpdatedAt.Format(time.DateTime),
			Applications: applications,
			Error:        task.Error,
		},
	}, nil
}

func transformEdges(edges []*model.Edge) []*v1.Edge {
	edgesV1 := make([]*v1.Edge, len(edges))
	for i, edge := range edges {
		edgesV1[i] = transformEdge(edge)
	}
	return edgesV1
}

func transformEdge(edge *model.Edge) *v1.Edge {
	edgeV1 := &v1.Edge{
		Id:          uint64(edge.ID),
		Name:        edge.Name,
		Description: edge.Description,
		Status:      int32(edge.Status),
		Online:      int32(edge.Online),
		CreatedAt:   edge.CreatedAt.Format(time.DateTime),
		UpdatedAt:   edge.UpdatedAt.Format(time.DateTime),
	}
	// 填充设备信息
	if edge.Device != nil {
		edgeV1.Device = transformDevice(edge.Device)
	}
	return edgeV1
}

// generateAccessKey 生成 Access Key
// 格式: 时间戳 + 随机字符串
func generateAccessKey() string {
	// 获取时间戳
	timestamp := time.Now().UnixNano()

	// 生成随机字节
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)

	// 组合并编码
	data := fmt.Sprintf("%d%s", timestamp, hex.EncodeToString(randomBytes))
	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(data))

	// 限制长度
	if len(encoded) > 20 {
		encoded = encoded[:20]
	}

	return encoded
}

// generateSecretKey 生成 Secret Key
// 格式: 32字节随机数据，Base64编码
func generateSecretKey() string {
	randomBytes := make([]byte, 32)
	rand.Read(randomBytes)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(randomBytes)
}

// generateAccessKeyPair 生成 Access Key 和 Secret Key 对
func generateAccessKeyPair() (accessKey, secretKey string) {
	return generateAccessKey(), generateSecretKey()
}
