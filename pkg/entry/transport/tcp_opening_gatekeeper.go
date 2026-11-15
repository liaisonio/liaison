package transport

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/liaisonio/liaison/pkg/entry/transport/tcpwindow"
	"github.com/liaisonio/liaison/pkg/proto"
)

func (m *Gatekeeper) createOnDemandProxy(ctx context.Context, target *proto.Proxy) error {
	if target.AccessProtocol != "tcp" || target.ProxyPort < 1 || target.ProxyPort > 65535 {
		return tcpwindow.ErrInvalid
	}
	snapshot := *target
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing := m.proxies[target.ID]; existing != nil {
		if existing.target != nil && existing.opening != nil &&
			existing.target.ProxyPort == snapshot.ProxyPort &&
			existing.target.EdgeID == snapshot.EdgeID &&
			existing.target.ApplicationID == snapshot.ApplicationID &&
			existing.target.Dst == snapshot.Dst && existing.ctx.Err() == nil {
			return nil
		}
		return tcpwindow.ErrConflict
	}
	if _, exists := m.proxiesIdxPort[target.ProxyPort]; exists {
		return tcpwindow.ErrConflict
	}
	routeCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	var runtime *tcpOpeningRuntime
	var err error
	runtime, err = newTCPOpeningRuntime(routeCtx, func(ctx context.Context) (tcpOpeningListener, error) {
		listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", fmt.Sprintf(":%d", snapshot.ProxyPort))
		if err != nil {
			return nil, err
		}
		rp, _, err := m.startTCPListener(ctx, &snapshot, listener, runtime.registry)
		if err != nil {
			_ = listener.Close()
			return nil, err
		}
		return rp, nil
	})
	if err != nil {
		cancel()
		return err
	}
	m.proxies[target.ID] = &proxy{target: &snapshot, port: target.ProxyPort, ctx: routeCtx, cancel: cancel, done: runtime.done, opening: runtime}
	m.proxiesIdxPort[target.ProxyPort] = target.ID
	m.proxyAppMap[target.ID] = target.ApplicationID
	return nil
}

func (m *Gatekeeper) opening(id int) (*tcpOpeningRuntime, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p := m.proxies[id]
	if p == nil || p.opening == nil || p.ctx.Err() != nil {
		return nil, tcpwindow.ErrClosed
	}
	return p.opening, nil
}

func (m *Gatekeeper) GrantTCPOpening(ctx context.Context, id int, requestID string, owner int64, duration time.Duration) (tcpwindow.Lease, error) {
	r, err := m.opening(id)
	if err != nil {
		return tcpwindow.Lease{}, err
	}
	return r.grant(ctx, requestID, owner, duration)
}
func (m *Gatekeeper) GetTCPOpening(ctx context.Context, id int, requestID string, owner int64) (tcpwindow.Lease, error) {
	r, err := m.opening(id)
	if err != nil {
		return tcpwindow.Lease{}, err
	}
	return r.registry.Get(time.Now(), requestID, owner)
}
func (m *Gatekeeper) RevokeTCPOpening(ctx context.Context, id int, requestID string, owner int64) error {
	r, err := m.opening(id)
	if err != nil {
		return err
	}
	return r.revoke(ctx, requestID, owner)
}
