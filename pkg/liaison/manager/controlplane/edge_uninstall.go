package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
)

var uninstallIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type edgeUninstaller interface {
	UninstallEdge(context.Context, uint64, proto.UninstallCommand) error
}

func randomUninstallID(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func (cp *controlPlane) requireUninstall(ctx context.Context, edge uint64) error {
	if _, ok := actorUserID(ctx); !ok || cp.authorizeFeature == nil {
		return iam.ErrForbidden
	}
	if err := cp.authorizeFeature(ctx, iam.FeatureConnectorUninstall); err != nil {
		return err
	}
	return requireVisibleResource(ctx, cp.repo, resourceConnector, edge)
}

func (cp *controlPlane) CreateEdgeUninstall(ctx context.Context, id uint64, name, instance string) (*model.EdgeUninstallTask, error) {
	if err := cp.requireUninstall(ctx, id); err != nil {
		return nil, err
	}
	edge, err := cp.repo.GetEdge(id)
	if err != nil {
		return nil, mapRecordNotFound(err, "EDGE_NOT_FOUND", "连接器不存在")
	}
	if name != edge.Name || name == "" {
		return nil, badRequest("CONFIRMATION_REQUIRED", "请确认连接器名称")
	}
	if task, err := cp.repo.GetEdgeUninstallTask(ctx, id, ""); err == nil {
		return task, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	status, err := cp.GetEdgeInstallation(ctx, id)
	if err != nil {
		return nil, err
	}
	if !status.CanUninstall || !uninstallIDPattern.MatchString(status.RuntimeID) || status.InstanceID == "" || status.InstanceID != instance {
		return nil, badRequest("UNINSTALL_UNAVAILABLE", "当前安装不可远程卸载")
	}
	sender, ok := cp.frontierBound.(edgeUninstaller)
	if !ok {
		return nil, badRequest("UNINSTALL_UNAVAILABLE", "当前安装不可远程卸载")
	}
	endpoint, err := url.Parse(cp.conf.Manager.ServerURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || strings.Trim(endpoint.Path, "/") != "" {
		return nil, badRequest("CALLBACK_UNAVAILABLE", "请配置有效的 HTTPS 服务地址")
	}
	jobID, err := randomUninstallID(16)
	if err != nil {
		return nil, err
	}
	token, err := randomUninstallID(32)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(token))
	actor, _ := actorUserID(ctx)
	task := &model.EdgeUninstallTask{ID: jobID, EdgeID: id, ActiveEdge: &id, CreatedBy: actor, InstanceID: instance, Status: "accepted", TokenHash: hex.EncodeToString(sum[:]), ExpiresAt: time.Now().Add(5 * time.Minute)}
	created, err := cp.repo.CreateEdgeUninstallTask(ctx, task)
	if err != nil {
		return nil, err
	}
	if !created {
		return cp.repo.GetEdgeUninstallTask(ctx, id, "")
	}
	// The task is itself a durable operation record. Audit failure prevents dispatch.
	if err := cp.RecordManagementAudit(ctx, &ManagementAudit{UserID: actor, Module: "connectors", Action: "uninstall", Resource: jobID, Method: "POST", Success: true, StatusCode: 202}); err != nil {
		if stateErr := cp.repo.SetEdgeUninstallResult(ctx, jobID, "failed", "audit_unavailable"); stateErr != nil {
			return nil, stateErr
		}
		return nil, err
	}
	endpoint.Path = "/api/v1/edge-uninstall-results/" + jobID
	command := proto.UninstallCommand{RuntimeID: status.RuntimeID, TaskID: jobID, InstanceID: instance, ExpiresAt: task.ExpiresAt.Unix(), CallbackURL: endpoint.String(), CallbackToken: token}
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err = sender.UninstallEdge(callCtx, id, command); err != nil {
		// Delivery can fail after the helper was launched. Never retry automatically
		// and never translate a broken connection into successful uninstallation.
		latest, getErr := cp.repo.GetEdgeUninstallTask(ctx, id, jobID)
		if getErr != nil {
			return nil, getErr
		}
		if latest.Status == "accepted" {
			if err = cp.repo.SetEdgeUninstallResult(ctx, jobID, "unknown", "delivery_unconfirmed"); err != nil {
				return nil, err
			}
		}
	}
	return cp.repo.GetEdgeUninstallTask(ctx, id, jobID)
}

func (cp *controlPlane) GetEdgeUninstall(ctx context.Context, edge uint64, id string) (*model.EdgeUninstallTask, error) {
	if err := cp.requireUninstall(ctx, edge); err != nil {
		return nil, err
	}
	if id == "active" {
		id = ""
	} else if !uninstallIDPattern.MatchString(id) {
		return nil, badRequest("TASK_ID_INVALID", "任务 ID 无效")
	}
	task, err := cp.repo.GetEdgeUninstallTask(ctx, edge, id)
	if id == "" && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, mapRecordNotFound(err, "TASK_NOT_FOUND", "任务不存在")
	}
	if time.Now().After(task.ExpiresAt) && task.Status != "completed" && task.Status != "failed" {
		task.Status = "unknown"
		task.Reason = "result_unconfirmed"
	}
	return task, nil
}

func (cp *controlPlane) ReportEdgeUninstall(ctx context.Context, id, token, status string) error {
	if !uninstallIDPattern.MatchString(id) || len(token) != 64 {
		return iam.ErrForbidden
	}
	if status != "running" && status != "completed" && status != "failed" {
		return badRequest("TASK_STATUS_INVALID", "任务状态无效")
	}
	task, err := cp.repo.GetEdgeUninstallTaskByID(ctx, id)
	if err != nil {
		return iam.ErrForbidden
	}
	sum := sha256.Sum256([]byte(token))
	expected, err := hex.DecodeString(task.TokenHash)
	if err != nil || subtle.ConstantTimeCompare(sum[:], expected) != 1 || time.Now().After(task.ExpiresAt) {
		return iam.ErrForbidden
	}
	reason := ""
	if status == "failed" {
		reason = "local_uninstall_failed"
	}
	return cp.repo.SetEdgeUninstallResult(ctx, id, status, reason)
}
