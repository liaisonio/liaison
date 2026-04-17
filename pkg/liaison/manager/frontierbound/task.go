package frontierbound

import (
	"context"
	"encoding/json"
	"errors"
	"net"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/geminio"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

type Net struct {
	Nets     []string
	Protocol string
	Port     int
}

func (fb *frontierBound) EmitScanApplications(ctx context.Context, taskID uint, edgeID uint64, net *Net) error {
	request := &proto.ScanApplicationTaskRequest{
		TaskID:   taskID,
		Nets:     net.Nets,
		Protocol: net.Protocol,
		Port:     net.Port,
	}
	data, err := json.Marshal(request)
	if err != nil {
		log.Errorf("marshal scan applications error: %s", err)
		return err
	}
	req := fb.svc.NewRequest(data)
	rsp, err := fb.svc.Call(ctx, edgeID, "scan_application", req)
	if err != nil {
		log.Errorf("emit scan applications call error: %s", err)
		return err
	}
	if rsp.Error() != nil {
		log.Errorf("emit scan applications return error: %s", rsp.Error())
		return rsp.Error()
	}
	return nil
}

func (fb *frontierBound) reportTaskScanApplication(ctx context.Context, req geminio.Request, rsp geminio.Response) {
	var task proto.ScanApplicationTaskResult
	err := json.Unmarshal(req.Data(), &task)
	if err != nil {
		rsp.SetError(err)
		return
	}

	// 获取任务
	mtask, err := fb.repo.GetTask(task.TaskID)
	if err != nil {
		rsp.SetError(err)
		return
	}
	if mtask.TaskStatus == model.TaskStatusFailed {
		rsp.SetError(errors.New("task alreadyexpired"))
		return
	}

	// 转换为model，过滤 IPv6 地址
	applications := []model.ScannedApplication{}
	for _, application := range task.ScannedApplications {
		// 忽略 IPv6 地址
		ip := net.ParseIP(application.IP)
		if ip != nil && ip.To4() == nil {
			// IPv6 地址，跳过
			continue
		}
		applications = append(applications, model.ScannedApplication{
			IP:       application.IP,
			Port:     application.Port,
			Protocol: application.Protocol,
		})
	}
	result := model.TaskScanApplicationResult{
		ScannedApplications: applications,
	}
	data, err := json.Marshal(result)
	if err != nil {
		rsp.SetError(err)
		return
	}

	// 确认状态
	switch task.Status {
	case "running":
		err = fb.repo.UpdateTaskResult(task.TaskID, model.TaskStatusRunning, data)
		if err != nil {
			rsp.SetError(err)
			return
		}
	case "completed":
		err = fb.repo.UpdateTaskResult(task.TaskID, model.TaskStatusCompleted, data)
		if err != nil {
			rsp.SetError(err)
			return
		}
	case "failed":
		err = fb.repo.UpdateTaskError(task.TaskID, task.Error)
		if err != nil {
			rsp.SetError(err)
			return
		}
	}
}

func (fb *frontierBound) pullTaskScanApplication(ctx context.Context, req geminio.Request, rsp geminio.Response) {
	var request proto.PullTaskScanApplicationRequest
	err := json.Unmarshal(req.Data(), &request)
	if err != nil {
		rsp.SetError(err)
		return
	}

	// 获取任务
	task, err := fb.repo.GetTaskByEdgeID(request.EdgeID)
	if err != nil {
		rsp.SetError(err)
		return
	}
	params := model.TaskScanApplicationParams{}
	err = json.Unmarshal(task.TaskParams, &params)
	if err != nil {
		rsp.SetError(err)
		return
	}

	// 返回任务
	response := proto.PullTaskScanApplicationResponse{
		TaskID:   task.ID,
		Nets:     params.Nets,
		Port:     params.Port,
		Protocol: params.Protocol,
	}
	data, err := json.Marshal(response)
	if err != nil {
		rsp.SetError(err)
		return
	}
	rsp.SetData(data)
}
