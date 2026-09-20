package frontierbound

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/liaisonio/liaison/pkg/proto"
)

func (fb *frontierBound) UninstallEdge(ctx context.Context, id uint64, command proto.UninstallCommand) error {
	data, err := json.Marshal(command)
	if err != nil {
		return err
	}
	rsp, err := fb.svc.Call(ctx, id, proto.RPCUninstall, fb.svc.NewRequest(data))
	if err != nil {
		return err
	}
	return rsp.Error()
}

func (fb *frontierBound) EdgeInstallationStatus(ctx context.Context, id uint64) (proto.InstallationStatus, error) {
	rsp, err := fb.svc.Call(ctx, id, proto.RPCInstallationStatus, fb.svc.NewRequest([]byte(`{}`)))
	if err != nil {
		return proto.InstallationStatus{}, err
	}
	if rsp.Error() != nil {
		return proto.InstallationStatus{}, rsp.Error()
	}
	if len(rsp.Data()) > 16*1024 {
		return proto.InstallationStatus{}, fmt.Errorf("installation status exceeds limit")
	}
	var status proto.InstallationStatus
	if err = json.Unmarshal(rsp.Data(), &status); err != nil {
		return status, err
	}
	return status, nil
}
