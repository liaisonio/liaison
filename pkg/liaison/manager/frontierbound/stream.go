package frontierbound

import (
	"context"

	"github.com/singchia/geminio"
)

func (fb *frontierBound) OpenStream(ctx context.Context, edgeID uint64) (geminio.Stream, error) {
	return fb.svc.OpenStream(ctx, edgeID)
}
