package frontierbound

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio/application"
)

func (fb *frontierBound) ProbeApplication(ctx context.Context, id uint64, target proto.TCPProbeRequest) (proto.TCPProbeResult, error) {
	data, err := json.Marshal(target)
	if err != nil {
		return proto.TCPProbeResult{}, err
	}
	response, err := fb.svc.Call(ctx, id, proto.RPCTCPProbe, fb.svc.NewRequest(data))
	if missingProbeRPC(err) {
		return proto.TCPProbeResult{Version: 1, Status: "edge_upgrade_required"}, nil
	}
	if err != nil {
		return proto.TCPProbeResult{}, err
	}
	if missingProbeRPC(response.Error()) {
		return proto.TCPProbeResult{Version: 1, Status: "edge_upgrade_required"}, nil
	}
	if response.Error() != nil {
		return proto.TCPProbeResult{}, response.Error()
	}
	if len(response.Data()) > 1024 {
		return proto.TCPProbeResult{}, errors.New("invalid probe response")
	}
	var result proto.TCPProbeResult
	err = json.Unmarshal(response.Data(), &result)
	return result, err
}

// Frontier serializes remote errors, so sentinel identity is lost in transit.
// Match only the two exact missing-method responses, never arbitrary RPC errors.
func missingProbeRPC(err error) bool {
	return err != nil && (errors.Is(err, application.ErrRemoteRPCUnregistered) ||
		err.Error() == application.ErrRemoteRPCUnregistered.Error() ||
		err.Error() == "no such rpc: "+proto.RPCTCPProbe)
}
