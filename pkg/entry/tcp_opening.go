package entry

import (
	"context"
	"github.com/liaisonio/liaison/pkg/entry/transport/tcpwindow"
	"time"
)

func (m *unifiedProxyManager) GrantTCPOpening(ctx context.Context, id int, requestID string, owner int64, duration time.Duration) (tcpwindow.Lease, error) {
	return m.gatekeeper.GrantTCPOpening(ctx, id, requestID, owner, duration)
}
func (m *unifiedProxyManager) GetTCPOpening(ctx context.Context, id int, requestID string, owner int64) (tcpwindow.Lease, error) {
	return m.gatekeeper.GetTCPOpening(ctx, id, requestID, owner)
}
func (m *unifiedProxyManager) RevokeTCPOpening(ctx context.Context, id int, requestID string, owner int64) error {
	return m.gatekeeper.RevokeTCPOpening(ctx, id, requestID, owner)
}
