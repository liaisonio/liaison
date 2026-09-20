package controlplane

import (
	"context"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

type edgeInstallationReader interface {
	EdgeInstallationStatus(context.Context, uint64) (proto.InstallationStatus, error)
}

func (cp *controlPlane) GetEdgeInstallation(ctx context.Context, id uint64) (proto.InstallationStatus, error) {
	empty := proto.InstallationStatus{Version: 1, Reason: "installation_unverified"}
	if _, ok := actorUserID(ctx); !ok {
		return empty, iam.ErrForbidden
	}
	if cp.authorizeFeature == nil {
		return empty, iam.ErrForbidden
	}
	if err := cp.authorizeFeature(ctx, iam.FeatureConnectorUninstall); err != nil {
		return empty, err
	}
	if id == 0 {
		return empty, badRequest("EDGE_ID_REQUIRED", "连接器 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceConnector, id); err != nil {
		return empty, err
	}
	edge, err := cp.repo.GetEdge(id)
	if err != nil {
		return empty, mapRecordNotFound(err, "EDGE_NOT_FOUND", "连接器不存在")
	}
	if edge.Online != model.EdgeOnlineStatusOnline || edge.Status != model.EdgeStatusRunning {
		empty.Reason = "connector_offline"
		return empty, nil
	}
	reader, ok := cp.frontierBound.(edgeInstallationReader)
	if !ok {
		empty.Reason = "edge_upgrade_required"
		return empty, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	status, err := reader.EdgeInstallationStatus(ctx, id)
	if err != nil {
		empty.Reason = "preflight_unavailable"
		return empty, nil
	}
	if status.Version != 1 {
		empty.Reason = "edge_upgrade_required"
		return empty, nil
	}
	if status.OwnershipVerified && !status.CanUninstall {
		status.Reason = "uninstall_executor_unavailable"
	} else if !status.OwnershipVerified {
		status.CanUninstall = false
		status.Reason = "installation_unverified"
	}
	return status, nil
}
