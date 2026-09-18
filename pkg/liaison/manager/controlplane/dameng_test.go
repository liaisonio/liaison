package controlplane

import (
	"context"
	"github.com/liaisonio/liaison/pkg/dameng"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDamengBuildCapabilityIsEnforced(t *testing.T) {
	require.Equal(t, dameng.Available(), isAllowedApplicationType("dameng"))
	require.Equal(t, dameng.Available(), isWebDataProtocol("dameng"))
	require.Equal(t, 5236, getDefaultPortByApplicationType("dameng"))
	err := validateAccessProtocol(model.AccessProtocolWeb, &model.Application{ApplicationType: model.ApplicationTypeDameng})
	if dameng.Available() {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
	}
}

func TestDamengTargetAndCredentialIsolation(t *testing.T) {
	if !dameng.Available() {
		t.Skip("requires optional driver build")
	}
	cp, r := newTestControlPlane(t)
	t.Cleanup(func() { r.Close() })
	_, app := createTestEdgeApplication(t, r)
	app.ApplicationType = model.ApplicationTypeDameng
	require.NoError(t, r.UpdateApplication(app))
	proxy := &model.Proxy{Name: "dameng-fixture", ApplicationID: app.ID, Status: model.ProxyStatusRunning, AccessProtocol: model.AccessProtocolWeb}
	require.NoError(t, r.CreateProxy(proxy))
	grantTestResourceToUsers(t, r, resourceAccess, proxy.ID, 1, 2)
	first := context.WithValue(context.Background(), "user_id", uint(1))
	second := context.WithValue(context.Background(), "user_id", uint(2))
	outsider := context.WithValue(context.Background(), "user_id", uint(3))
	require.NoError(t, cp.SaveWebDataCredential(first, proxy.ID, "dameng", "fixture", "", "", "cipher-fixture", "nonce-fixture"))
	target, err := cp.GetWebDataTarget(first, proxy.ID)
	require.NoError(t, err)
	require.Len(t, target.Credentials, 1)
	target, err = cp.GetWebDataTarget(second, proxy.ID)
	require.NoError(t, err)
	require.Empty(t, target.Credentials)
	_, err = cp.GetWebDataCredentialSecret(second, proxy.ID, "dameng", "fixture", "", "")
	require.Error(t, err)
	_, err = cp.GetWebDataTarget(outsider, proxy.ID)
	require.Error(t, err)
}
