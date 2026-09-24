package frontierbound

import (
	"context"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

func TestDevicePollRefreshesOnlyAuthenticatedHostWithoutUsage(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()
	edge := createFrontierboundTestEdge(t, r, "edge")
	other := createFrontierboundTestEdge(t, r, "other")
	old := time.Now().Add(-5 * time.Minute)
	devices := []*model.Device{}
	for i, name := range []string{"host", "discovered", "other-host"} {
		d := &model.Device{Name: name, Fingerprint: name, HeartbeatAt: old, Online: model.DeviceOnlineStatusOffline, CPUUsage: 42, MemoryUsage: 31, DiskUsage: 20}
		if err := r.CreateDevice(d); err != nil {
			t.Fatal(err)
		}
		edgeID, kind := uint64(edge.ID), model.EdgeDeviceRelationHost
		if i == 1 {
			kind = model.EdgeDeviceRelationDiscovered
		}
		if i == 2 {
			edgeID = uint64(other.ID)
		}
		if err := r.CreateEdgeDevice(&model.EdgeDevice{EdgeID: edgeID, DeviceID: d.ID, Type: kind}); err != nil {
			t.Fatal(err)
		}
		devices = append(devices, d)
	}
	fb := &frontierBound{repo: r}
	// A forged edge ID must not refresh either device.
	bad := &fakeResponse{}
	fb.getEdgeDiscoveredDevices(context.Background(), newFakeRequest(t, uint64(edge.ID), proto.GetEdgeDiscoveredDevicesRequest{EdgeID: uint64(other.ID)}), bad)
	if bad.Error() == nil {
		t.Fatal("accepted forged edge ID")
	}
	before, err := r.GetDeviceByID(devices[0].ID)
	if err != nil || before.HeartbeatAt.After(old.Add(time.Second)) {
		t.Fatalf("forged request updated host: %v", err)
	}
	rsp := &fakeResponse{}
	fb.getEdgeDiscoveredDevices(context.Background(), newFakeRequest(t, uint64(edge.ID), proto.GetEdgeDiscoveredDevicesRequest{EdgeID: uint64(edge.ID)}), rsp)
	if rsp.Error() != nil {
		t.Fatal(rsp.Error())
	}
	for i, d := range devices {
		got, err := r.GetDeviceByID(d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if got.Online != model.DeviceOnlineStatusOnline || time.Since(got.HeartbeatAt) > time.Second {
				t.Fatal("host heartbeat not refreshed")
			}
		} else if got.HeartbeatAt.After(old.Add(time.Second)) {
			t.Fatalf("refreshed unrelated device %s", d.Name)
		}
		if got.CPUUsage != 42 || got.MemoryUsage != 31 || got.DiskUsage != 20 {
			t.Fatal("heartbeat overwrote resource metrics")
		}
	}
}
