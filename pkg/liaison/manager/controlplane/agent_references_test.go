package controlplane

import (
	"context"
	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAgentReferencesRespectOwnershipAndDeletion(t *testing.T) {
	cp, r := newTestControlPlane(t)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	admin, a, b := seedResourceScopeUsers(t, r)
	_, err := cp.CreateEdge(a, &v1.CreateEdgeRequest{Name: "selected edge"})
	require.NoError(t, err)
	edges, err := cp.ListEdges(a, &v1.ListEdgesRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, edges.Data.Edges, 1)
	device := &model.Device{Name: "selected device", Fingerprint: "refs-device"}
	require.NoError(t, r.CreateDevice(device))
	require.NoError(t, claimResource(a, r, resourceDevice, uint64(device.ID)))
	app := &model.Application{Name: "selected app", DeviceID: device.ID, IP: "127.0.0.1", Port: 8080, EdgeIDs: model.UintSlice{}}
	require.NoError(t, r.CreateApplication(app))
	require.NoError(t, claimResource(a, r, resourceApplication, uint64(app.ID)))
	for _, ref := range []struct {
		kind string
		id   uint64
		name string
	}{{"connector", edges.Data.Edges[0].Id, "selected edge"}, {"device", uint64(device.ID), "selected device"}, {"application", uint64(app.ID), "selected app"}} {
		t.Run(ref.kind, func(t *testing.T) {
			for _, actor := range []context.Context{a, admin} {
				id, _ := actorUserID(actor)
				name, err := cp.ResolveAgentResource(context.Background(), id, ref.kind, ref.id)
				require.NoError(t, err)
				require.Equal(t, ref.name, name)
			}
			id, _ := actorUserID(b)
			_, err := cp.ResolveAgentResource(admin, id, ref.kind, ref.id)
			require.Error(t, err) // ambient admin context cannot override actor
			_, err = cp.ResolveAgentResource(admin, 0, ref.kind, ref.id)
			require.Error(t, err)
		})
	}
	require.NoError(t, r.DeleteApplication(app.ID))
	id, _ := actorUserID(a)
	_, err = cp.ResolveAgentResource(a, id, "application", uint64(app.ID))
	require.Error(t, err)
}
