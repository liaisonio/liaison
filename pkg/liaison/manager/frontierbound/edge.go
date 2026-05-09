package frontierbound

import (
	"context"

	"github.com/singchia/geminio"
)

// 上报edge，更改初始化到Running状态
func (fb *frontierBound) reportEdge(ctx context.Context, req geminio.Request, rsp geminio.Response) {
	if _, err := fb.requireRegisteredEdge(req); err != nil {
		rsp.SetError(err)
	}
}
