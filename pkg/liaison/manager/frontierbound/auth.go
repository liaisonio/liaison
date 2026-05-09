package frontierbound

import (
	"fmt"

	"github.com/singchia/geminio"
)

func (fb *frontierBound) requireRegisteredEdge(req geminio.Request) (uint64, error) {
	edgeID := req.ClientID()
	if edgeID == 0 {
		return 0, fmt.Errorf("missing edge client id")
	}
	if _, err := fb.repo.GetEdge(edgeID); err != nil {
		return 0, fmt.Errorf("edge %d is not registered: %w", edgeID, err)
	}
	return edgeID, nil
}

func (fb *frontierBound) requireRequestEdge(req geminio.Request, payloadEdgeID uint64) (uint64, error) {
	edgeID, err := fb.requireRegisteredEdge(req)
	if err != nil {
		return 0, err
	}
	if payloadEdgeID != 0 && payloadEdgeID != edgeID {
		return 0, fmt.Errorf("edge id mismatch: connection edge %d, payload edge %d", edgeID, payloadEdgeID)
	}
	return edgeID, nil
}
