package controlplane

import (
	"context"
	"testing"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

func TestHTTPEntryDefaultsAndLegacyUpdate(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, app := createTestEdgeApplication(t, r)
	app.ApplicationType = model.ApplicationTypeHTTP
	require.NoError(t, r.UpdateApplication(app))
	pm := newFakeProxyManager()
	cp.RegisterProxyManager(pm)
	created, err := cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{ApplicationId: uint64(app.ID), AccessProtocol: "http"})
	require.NoError(t, err)
	require.Equal(t, "path", created.Data.HttpEntryMode)
	require.Zero(t, created.Data.Port)
	require.Contains(t, created.Data.AccessUrl, "/access/")
	require.Contains(t, created.Data.AccessUrl, "/web/")
	saved, err := r.GetProxyByID(uint(created.Data.Id))
	require.NoError(t, err)
	require.True(t, isWebOnlyProxy(saved, app))
	updated, err := cp.UpdateProxy(context.Background(), &v1.UpdateProxyRequest{Id: created.Data.Id, Name: "Renamed"})
	require.NoError(t, err)
	require.Equal(t, "path", updated.Data.HttpEntryMode)
	legacy := &model.Proxy{Name: "Legacy", ApplicationID: app.ID, AccessProtocol: model.AccessProtocolHTTP, Port: 12345, Status: model.ProxyStatusStopped}
	require.NoError(t, r.CreateProxy(legacy))
	updated, err = cp.UpdateProxy(context.Background(), &v1.UpdateProxyRequest{Id: uint64(legacy.ID), Name: "Legacy renamed"})
	require.NoError(t, err)
	require.Equal(t, "port", updated.Data.HttpEntryMode)
	require.EqualValues(t, 12345, updated.Data.Port)
	_, err = cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{ApplicationId: uint64(app.ID), AccessProtocol: "http", HttpEntryMode: "domain"})
	require.Error(t, err)
	_, err = cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{ApplicationId: uint64(app.ID), AccessProtocol: "tcp", HttpEntryMode: "path"})
	require.Error(t, err)
	updated, err = cp.UpdateProxy(context.Background(), &v1.UpdateProxyRequest{Id: uint64(legacy.ID), HttpEntryMode: "path"})
	require.NoError(t, err)
	require.Zero(t, updated.Data.Port)
}
