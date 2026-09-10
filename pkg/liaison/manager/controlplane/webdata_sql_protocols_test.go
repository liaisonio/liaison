package controlplane

import (
	"context"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

func TestSQLProtocols_TargetAndCredentialIsolation(t *testing.T) {
	for _, protocol := range []string{"mariadb", "sqlserver", "oracle", "postgresql"} {
		t.Run(protocol, func(t *testing.T) {
			cp, r := newTestControlPlane(t)
			t.Cleanup(func() { r.Close() })
			_, app := createTestEdgeApplication(t, r)
			app.ApplicationType = model.ApplicationType(protocol)
			require.NoError(t, r.UpdateApplication(app))
			proxy := &model.Proxy{Name: "sql-proxy", ApplicationID: app.ID, Status: model.ProxyStatusRunning, AccessProtocol: model.AccessProtocolWeb}
			require.NoError(t, r.CreateProxy(proxy))
			require.NoError(t, validateAccessProtocol(model.AccessProtocolWeb, app))
			grantTestResourceToUsers(t, r, resourceAccess, proxy.ID, 1, 2)
			user1 := context.WithValue(context.Background(), "user_id", uint(1))
			user2 := context.WithValue(context.Background(), "user_id", uint(2))
			outsider := context.WithValue(context.Background(), "user_id", uint(3))
			require.NoError(t, cp.SaveWebDataCredential(user1, proxy.ID, protocol, "test", "app", "", "encrypted-test", "nonce-test"))
			target, err := cp.GetWebDataTarget(user1, proxy.ID)
			require.NoError(t, err)
			require.Equal(t, protocol, target.Protocol)
			require.Len(t, target.Credentials, 1)
			other, err := cp.GetWebDataTarget(user2, proxy.ID)
			require.NoError(t, err)
			require.Empty(t, other.Credentials)
			_, err = cp.GetWebDataCredentialSecret(user2, proxy.ID, protocol, "test", "app", "")
			require.Error(t, err)
			_, err = cp.GetWebDataTarget(outsider, proxy.ID)
			require.Error(t, err)
			require.True(t, isAccessAuditProtocol(protocol))
		})
	}
}
