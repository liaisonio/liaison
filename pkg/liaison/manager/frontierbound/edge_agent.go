package frontierbound

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio/application"
)

func (fb *frontierBound) EdgeAgent(ctx context.Context, id uint64, req proto.EdgeAgentRPCRequest) (proto.EdgeAgentResult, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	rsp, err := fb.svc.Call(ctx, id, proto.RPCEdgeAgent, fb.svc.NewRequest(data))
	missing := func(err error) bool {
		return err != nil && (errors.Is(err, application.ErrRemoteRPCUnregistered) || err.Error() == application.ErrRemoteRPCUnregistered.Error() || err.Error() == "no such rpc: "+proto.RPCEdgeAgent)
	}
	if missing(err) {
		return proto.EdgeAgentResult{Version: 1, Status: "upgrade_required"}, nil
	}
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	if missing(rsp.Error()) {
		return proto.EdgeAgentResult{Version: 1, Status: "upgrade_required"}, nil
	}
	if rsp.Error() != nil {
		return proto.EdgeAgentResult{}, rsp.Error()
	}
	if len(rsp.Data()) > 512<<10 {
		return proto.EdgeAgentResult{}, errors.New("agent response too large")
	}
	var result proto.EdgeAgentResult
	err = json.Unmarshal(rsp.Data(), &result)
	return result, err
}
