package frontierbound

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/liaisonio/liaison/pkg/proto"
)

func (fb *frontierBound) WebIDE(ctx context.Context, id uint64, req proto.WebIDERequest) (proto.WebIDEResult, error) {
	if !req.Valid() {
		return proto.WebIDEResult{}, errors.New("invalid IDE request")
	}
	data, err := json.Marshal(req)
	if err != nil {
		return proto.WebIDEResult{}, err
	}
	rsp, err := fb.svc.Call(ctx, id, proto.RPCWebIDE, fb.svc.NewRequest(data))
	if err != nil {
		return proto.WebIDEResult{}, err
	}
	if rsp.Error() != nil {
		return proto.WebIDEResult{}, rsp.Error()
	}
	if len(rsp.Data()) > 512<<10 {
		return proto.WebIDEResult{}, errors.New("IDE response too large")
	}
	var result proto.WebIDEResult
	err = json.Unmarshal(rsp.Data(), &result)
	if err == nil && result.Version != 1 {
		err = errors.New("unsupported IDE protocol")
	}
	return result, err
}
