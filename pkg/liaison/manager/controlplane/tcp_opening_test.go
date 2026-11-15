package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/entry/transport/tcpwindow"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

type openingFakeManager struct {
	*fakeProxyManager
	registry *tcpwindow.Registry
	calls    int
}

func (m *openingFakeManager) GrantTCPOpening(_ context.Context, _ int, id string, owner int64, duration time.Duration) (tcpwindow.Lease, error) {
	m.calls++
	return m.registry.Grant(time.Now(), id, owner, duration)
}
func (m *openingFakeManager) GetTCPOpening(_ context.Context, _ int, id string, owner int64) (tcpwindow.Lease, error) {
	return m.registry.Get(time.Now(), id, owner)
}
func (m *openingFakeManager) RevokeTCPOpening(_ context.Context, _ int, id string, owner int64) error {
	return m.registry.Revoke(id, owner)
}

func TestTCPOpeningPolicy(t *testing.T) {
	p, err := normalizeTCPOpeningPolicy(nil, model.AccessProtocolTCP)
	require.NoError(t, err)
	require.Equal(t, "continuous", p.Mode)
	require.EqualValues(t, 10, p.DurationSeconds)
	for _, duration := range []int32{-1, 1, 4, 301} {
		_, err := normalizeTCPOpeningPolicy(&v1.TCPOpeningPolicy{Mode: "on_demand", DurationSeconds: duration}, model.AccessProtocolTCP)
		require.Error(t, err)
	}
	_, err = normalizeTCPOpeningPolicy(&v1.TCPOpeningPolicy{Mode: "on_demand"}, model.AccessProtocolHTTP)
	require.Error(t, err)
}

func TestTCPOpeningAuthorizationReplayAndAudit(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	registry, err := tcpwindow.New(256)
	require.NoError(t, err)
	runtime := &openingFakeManager{fakeProxyManager: newFakeProxyManager(), registry: registry}
	cp.proxyManager = runtime
	p := &model.Proxy{Name: "Test", Status: model.ProxyStatusRunning, AccessProtocol: model.AccessProtocolTCP, TCPOpeningMode: "on_demand", TCPOpeningDurationSeconds: 10}
	require.NoError(t, r.CreateProxy(p))
	grantTestResourceToUsers(t, r, resourceAccess, p.ID, 10, 11)
	ctx := context.WithValue(context.Background(), "user_id", uint(10))
	requestID := "test-opening-00001"
	_, err = cp.TCPOpening(context.Background(), p.ID, requestID, "open")
	require.Error(t, err)
	_, err = cp.TCPOpening(context.WithValue(ctx, "user_id", uint(12)), p.ID, requestID, "open")
	require.Error(t, err)
	require.Zero(t, runtime.calls)
	first, err := cp.TCPOpening(ctx, p.ID, requestID, "open")
	require.NoError(t, err)
	require.Equal(t, "open", first.State)
	retry, err := cp.TCPOpening(ctx, p.ID, requestID, "open")
	require.NoError(t, err)
	require.Equal(t, first.ExpiresAt, retry.ExpiresAt)
	other, err := cp.TCPOpening(context.WithValue(ctx, "user_id", uint(11)), p.ID, requestID, "get")
	require.NoError(t, err)
	require.Equal(t, "closed", other.State)
	closed, err := cp.TCPOpening(ctx, p.ID, requestID, "revoke")
	require.NoError(t, err)
	require.Equal(t, "closed", closed.State)
	retry, err = cp.TCPOpening(ctx, p.ID, requestID, "open")
	require.NoError(t, err)
	require.Equal(t, "closed", retry.State)
	p.Status = model.ProxyStatusStopped
	require.NoError(t, r.UpdateProxy(p))
	_, err = cp.TCPOpening(ctx, p.ID, "test-opening-00002", "open")
	require.Error(t, err)
}

func TestTCPOpeningPolicyPersistsAndRebuilds(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	cp.proxyManager = newFakeProxyManager()
	cp.firewallManager = newFakeFirewallManager()
	created, err := cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{ApplicationId: uint64(application.ID), Name: "Timed TCP", AccessProtocol: "tcp", TcpOpeningPolicy: &v1.TCPOpeningPolicy{Mode: "on_demand", DurationSeconds: 5}})
	require.NoError(t, err)
	require.Equal(t, "on_demand", created.Data.TcpOpeningPolicy.Mode)
	require.NotZero(t, created.Data.Port)
	updated, err := cp.UpdateProxy(context.Background(), &v1.UpdateProxyRequest{Id: created.Data.Id, TcpOpeningPolicy: &v1.TCPOpeningPolicy{Mode: "continuous", DurationSeconds: 10}})
	require.NoError(t, err)
	require.Equal(t, "continuous", updated.Data.TcpOpeningPolicy.Mode)
	stored, err := r.GetProxyByID(uint(created.Data.Id))
	require.NoError(t, err)
	require.Equal(t, "continuous", stored.TCPOpeningMode)
	require.Equal(t, 10, stored.TCPOpeningDurationSeconds)
}

func TestTCPOpeningRateLimitBounded(t *testing.T) {
	var limiter tcpOpeningRateLimit
	now := time.Now()
	for i := 0; i < 30; i++ {
		require.True(t, limiter.allow(1, now))
	}
	require.False(t, limiter.allow(1, now))
	require.True(t, limiter.allow(1, now.Add(time.Minute)))
}

type failedOpeningAuditRepo struct{ repo.Repo }

func (r failedOpeningAuditRepo) CreateManagementAudit(*model.ManagementAudit) error {
	return errors.New("audit unavailable")
}

func TestTCPOpeningAuditFailureDoesNotGrant(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	registry, err := tcpwindow.New(256)
	require.NoError(t, err)
	runtime := &openingFakeManager{fakeProxyManager: newFakeProxyManager(), registry: registry}
	cp.proxyManager = runtime
	p := &model.Proxy{Name: "Test", Status: model.ProxyStatusRunning, AccessProtocol: model.AccessProtocolTCP, TCPOpeningMode: "on_demand", TCPOpeningDurationSeconds: 10}
	require.NoError(t, r.CreateProxy(p))
	grantTestResourceToUsers(t, r, resourceAccess, p.ID, 10)
	cp.repo = failedOpeningAuditRepo{r}
	_, err = cp.TCPOpening(context.WithValue(context.Background(), "user_id", uint(10)), p.ID, "test-opening-00001", "open")
	require.Error(t, err)
	require.Zero(t, runtime.calls)
}
