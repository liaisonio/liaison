package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

type fakeProxyManager struct {
	created   map[int]*proto.Proxy
	deleted   []int
	createErr error
	deleteErr error
}

func newFakeProxyManager() *fakeProxyManager {
	return &fakeProxyManager{created: map[int]*proto.Proxy{}}
}

func (f *fakeProxyManager) CreateProxy(_ context.Context, proxy *proto.Proxy) error {
	if f.createErr != nil {
		return f.createErr
	}
	if proxy.ProxyPort == 0 {
		proxy.ProxyPort = 39001
	}
	copied := *proxy
	f.created[proxy.ID] = &copied
	return nil
}

func (f *fakeProxyManager) DeleteProxy(_ context.Context, id int) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.created, id)
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeFirewallManager struct {
	allowed map[int][]string
	revoked []int
}

func newFakeFirewallManager() *fakeFirewallManager {
	return &fakeFirewallManager{allowed: map[int][]string{}}
}

func (f *fakeFirewallManager) Allow(proxyID int, cidrs []string) error {
	f.allowed[proxyID] = append([]string(nil), cidrs...)
	return nil
}

func (f *fakeFirewallManager) Revoke(proxyID int) {
	delete(f.allowed, proxyID)
	f.revoked = append(f.revoked, proxyID)
}

func newTestControlPlane(t *testing.T) (*controlPlane, repo.Repo) {
	t.Helper()
	conf := &config.Configuration{}
	conf.Manager.DB = t.TempDir() + "/liaison.db"
	conf.Manager.ServerURL = "https://example.test"

	r, err := repo.NewRepo(conf)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	cp := &controlPlane{conf: conf, repo: r}
	return cp, r
}

func createTestEdgeApplication(t *testing.T, r repo.Repo) (*model.Edge, *model.Application) {
	t.Helper()
	edge := &model.Edge{
		Name:   "edge-1",
		Status: model.EdgeStatusRunning,
		Online: model.EdgeOnlineStatusOnline,
	}
	if err := r.CreateEdge(edge); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	application := &model.Application{
		Name:            "app-1",
		IP:              "127.0.0.1",
		Port:            8080,
		ApplicationType: model.ApplicationTypeTCP,
		EdgeIDs:         model.UintSlice{uint(edge.ID)},
	}
	if err := r.CreateApplication(application); err != nil {
		t.Fatalf("create application: %v", err)
	}
	return edge, application
}

func createTestDevice(t *testing.T, r repo.Repo, name string) *model.Device {
	t.Helper()
	device := &model.Device{
		Fingerprint: name + "-fingerprint",
		Name:        name,
		HostName:    name + ".local",
		Online:      model.DeviceOnlineStatusOnline,
		HeartbeatAt: time.Now(),
		OS:          "test-os",
	}
	if err := r.CreateDevice(device); err != nil {
		t.Fatalf("create device: %v", err)
	}
	return device
}

func TestEdgeStopKeepsProxyEnabledButStopsRuntime(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	pm := newFakeProxyManager()
	cp.RegisterProxyManager(pm)

	edge, application := createTestEdgeApplication(t, r)
	created, err := cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{
		ApplicationId: uint64(application.ID),
		Name:          "entry-1",
		Port:          32001,
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	if len(pm.created) != 1 {
		t.Fatalf("expected one running proxy, got %d", len(pm.created))
	}

	if _, err := cp.UpdateEdge(context.Background(), &v1.UpdateEdgeRequest{
		Id:     uint64(edge.ID),
		Status: int32(model.EdgeStatusStopped),
	}); err != nil {
		t.Fatalf("stop edge: %v", err)
	}
	updatedEdge, err := r.GetEdge(uint64(edge.ID))
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if updatedEdge.Online != model.EdgeOnlineStatusOnline {
		t.Fatalf("edge online changed to %d, want online state to stay independent", updatedEdge.Online)
	}

	proxy, err := r.GetProxyByID(uint(created.Data.Id))
	if err != nil {
		t.Fatalf("get proxy: %v", err)
	}
	if proxy.Status != model.ProxyStatusRunning {
		t.Fatalf("proxy status changed to %d, want running", proxy.Status)
	}
	if len(pm.created) != 0 {
		t.Fatalf("runtime proxy still running after edge stop")
	}

	listed, err := cp.ListProxies(context.Background(), &v1.ListProxiesRequest{Page: -1, PageSize: -1})
	if err != nil {
		t.Fatalf("list proxies: %v", err)
	}
	if got := listed.Data.Proxies[0].EffectiveStatus; got != proxyEffectiveStatusEdgeStopped {
		t.Fatalf("effective status = %q, want %q", got, proxyEffectiveStatusEdgeStopped)
	}
	if got := listed.Data.Proxies[0].EffectiveStatusMessage; got != "连接器已禁用" {
		t.Fatalf("effective status message = %q, want 连接器已禁用", got)
	}

	if _, err := cp.UpdateEdge(context.Background(), &v1.UpdateEdgeRequest{
		Id:     uint64(edge.ID),
		Status: int32(model.EdgeStatusRunning),
	}); err != nil {
		t.Fatalf("start edge: %v", err)
	}
	if len(pm.created) != 1 {
		t.Fatalf("runtime proxy was not restored after edge start")
	}
}

func TestStoppedProxyEffectiveStatusMessage(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()

	_, application := createTestEdgeApplication(t, r)
	proxy := &model.Proxy{
		Name:          "entry-1",
		Status:        model.ProxyStatusStopped,
		ApplicationID: application.ID,
		Port:          32001,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	listed, err := cp.ListProxies(context.Background(), &v1.ListProxiesRequest{Page: -1, PageSize: -1})
	if err != nil {
		t.Fatalf("list proxies: %v", err)
	}
	if got := listed.Data.Proxies[0].EffectiveStatus; got != proxyEffectiveStatusStopped {
		t.Fatalf("effective status = %q, want %q", got, proxyEffectiveStatusStopped)
	}
	if got := listed.Data.Proxies[0].EffectiveStatusMessage; got != "访问未启用" {
		t.Fatalf("effective status message = %q, want 访问未启用", got)
	}
}

func TestDeleteApplicationCascadesProxyAndFirewall(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	pm := newFakeProxyManager()
	fw := newFakeFirewallManager()
	cp.RegisterProxyManager(pm)
	cp.RegisterFirewallManager(fw)

	_, application := createTestEdgeApplication(t, r)
	created, err := cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{
		ApplicationId: uint64(application.ID),
		Name:          "entry-1",
		Port:          32002,
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	if _, err := cp.UpsertProxyFirewall(context.Background(), uint(created.Data.Id), []string{"203.0.113.1/32"}); err != nil {
		t.Fatalf("upsert firewall: %v", err)
	}

	if _, err := cp.DeleteApplication(context.Background(), &v1.DeleteApplicationRequest{Id: uint64(application.ID)}); err != nil {
		t.Fatalf("delete application: %v", err)
	}
	if _, err := r.GetProxyByID(uint(created.Data.Id)); err == nil {
		t.Fatalf("proxy still exists after application deletion")
	}
	if rule, err := r.GetFirewallRuleByProxyID(uint(created.Data.Id)); err != nil {
		t.Fatalf("get firewall rule: %v", err)
	} else if rule != nil {
		t.Fatalf("firewall rule still exists after application deletion")
	}
	if len(pm.created) != 0 {
		t.Fatalf("runtime proxy still running after application deletion")
	}
}

func TestDeleteEdgeCascadesOwnedDevices(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()

	edge, _ := createTestEdgeApplication(t, r)
	host := createTestDevice(t, r, "host-device")
	discovered := createTestDevice(t, r, "discovered-device")
	hostType := model.EdgeDeviceRelationHost
	discoveredType := model.EdgeDeviceRelationDiscovered
	if err := r.CreateEdgeDevice(&model.EdgeDevice{EdgeID: uint64(edge.ID), DeviceID: host.ID, Type: hostType}); err != nil {
		t.Fatalf("create host relation: %v", err)
	}
	if err := r.CreateEdgeDevice(&model.EdgeDevice{EdgeID: uint64(edge.ID), DeviceID: discovered.ID, Type: discoveredType}); err != nil {
		t.Fatalf("create discovered relation: %v", err)
	}

	if _, err := cp.DeleteEdge(context.Background(), &v1.DeleteEdgeRequest{Id: uint64(edge.ID)}); err != nil {
		t.Fatalf("delete edge: %v", err)
	}
	if _, err := r.GetDeviceByID(host.ID); err == nil {
		t.Fatalf("host device still exists after edge deletion")
	}
	if _, err := r.GetDeviceByID(discovered.ID); err == nil {
		t.Fatalf("discovered device still exists after edge deletion")
	}
	if relations, err := r.GetEdgeDevicesByEdgeID(uint64(edge.ID), nil); err != nil {
		t.Fatalf("get edge device relations: %v", err)
	} else if len(relations) != 0 {
		t.Fatalf("edge device relations still exist after edge deletion: %d", len(relations))
	}
}

func TestDeleteEdgeKeepsDevicesSharedWithAnotherEdge(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()

	edge, _ := createTestEdgeApplication(t, r)
	otherEdge := &model.Edge{
		Name:   "edge-2",
		Status: model.EdgeStatusRunning,
		Online: model.EdgeOnlineStatusOnline,
	}
	if err := r.CreateEdge(otherEdge); err != nil {
		t.Fatalf("create other edge: %v", err)
	}
	device := createTestDevice(t, r, "shared-device")
	discoveredType := model.EdgeDeviceRelationDiscovered
	if err := r.CreateEdgeDevice(&model.EdgeDevice{EdgeID: uint64(edge.ID), DeviceID: device.ID, Type: discoveredType}); err != nil {
		t.Fatalf("create first relation: %v", err)
	}
	if err := r.CreateEdgeDevice(&model.EdgeDevice{EdgeID: uint64(otherEdge.ID), DeviceID: device.ID, Type: discoveredType}); err != nil {
		t.Fatalf("create second relation: %v", err)
	}

	if _, err := cp.DeleteEdge(context.Background(), &v1.DeleteEdgeRequest{Id: uint64(edge.ID)}); err != nil {
		t.Fatalf("delete edge: %v", err)
	}
	if _, err := r.GetDeviceByID(device.ID); err != nil {
		t.Fatalf("shared device was deleted: %v", err)
	}
	relations, err := r.GetEdgeDevicesByDeviceID(device.ID, nil)
	if err != nil {
		t.Fatalf("get device relations: %v", err)
	}
	if len(relations) != 1 || relations[0].EdgeID != uint64(otherEdge.ID) {
		t.Fatalf("shared device relations = %+v, want only edge %d", relations, otherEdge.ID)
	}
}

func TestEdgeStopDoesNotPersistStoppedWhenRuntimeStopFails(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	pm := newFakeProxyManager()
	cp.RegisterProxyManager(pm)

	edge, application := createTestEdgeApplication(t, r)
	if _, err := cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{
		ApplicationId: uint64(application.ID),
		Name:          "entry-1",
		Port:          32003,
	}); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	pm.deleteErr = errors.New("listener close failed")
	if _, err := cp.UpdateEdge(context.Background(), &v1.UpdateEdgeRequest{
		Id:     uint64(edge.ID),
		Status: int32(model.EdgeStatusStopped),
	}); err == nil {
		t.Fatalf("expected stop edge error")
	}

	updatedEdge, err := r.GetEdge(uint64(edge.ID))
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if updatedEdge.Status != model.EdgeStatusRunning {
		t.Fatalf("edge status = %d, want running after failed stop", updatedEdge.Status)
	}
	if len(pm.created) != 1 {
		t.Fatalf("runtime proxy should still be tracked after failed stop")
	}
}

func TestDeleteProxyKeepsRecordWhenRuntimeStopFails(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	pm := newFakeProxyManager()
	cp.RegisterProxyManager(pm)

	_, application := createTestEdgeApplication(t, r)
	created, err := cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{
		ApplicationId: uint64(application.ID),
		Name:          "entry-1",
		Port:          32004,
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	pm.deleteErr = errors.New("listener close failed")
	if _, err := cp.DeleteProxy(context.Background(), &v1.DeleteProxyRequest{Id: created.Data.Id}); err == nil {
		t.Fatalf("expected delete proxy error")
	}
	if _, err := r.GetProxyByID(uint(created.Data.Id)); err != nil {
		t.Fatalf("proxy record should remain after failed runtime stop: %v", err)
	}
}

func TestStoppedEdgeAutoPortIsPersistedWhenEdgeStarts(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	pm := newFakeProxyManager()
	cp.RegisterProxyManager(pm)

	edge, application := createTestEdgeApplication(t, r)
	edge.Status = model.EdgeStatusStopped
	edge.Online = model.EdgeOnlineStatusOffline
	if err := r.UpdateEdge(edge); err != nil {
		t.Fatalf("stop edge in db: %v", err)
	}

	created, err := cp.CreateProxy(context.Background(), &v1.CreateProxyRequest{
		ApplicationId: uint64(application.ID),
		Name:          "entry-1",
	})
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	if created.Data.Port == 0 {
		t.Fatalf("created proxy port = 0, want reserved public port while edge is stopped")
	}
	if len(pm.created) != 0 {
		t.Fatalf("runtime should not start while edge is stopped")
	}

	if _, err := cp.UpdateEdge(context.Background(), &v1.UpdateEdgeRequest{
		Id:     uint64(edge.ID),
		Status: int32(model.EdgeStatusRunning),
	}); err != nil {
		t.Fatalf("start edge: %v", err)
	}
	proxy, err := r.GetProxyByID(uint(created.Data.Id))
	if err != nil {
		t.Fatalf("get proxy: %v", err)
	}
	if proxy.Port != int(created.Data.Port) {
		t.Fatalf("proxy port = %d, want reserved port %d", proxy.Port, created.Data.Port)
	}
	if len(pm.created) != 1 {
		t.Fatalf("runtime proxy was not started after edge start")
	}
	if got := pm.created[int(created.Data.Id)].ProxyPort; got != int(created.Data.Port) {
		t.Fatalf("runtime proxy port = %d, want reserved port %d", got, created.Data.Port)
	}
}

func TestRestoreProxyListenersPersistsAutoAllocatedPort(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	pm := newFakeProxyManager()
	cp.RegisterProxyManager(pm)

	_, application := createTestEdgeApplication(t, r)
	proxy := &model.Proxy{
		Name:          "entry-1",
		Status:        model.ProxyStatusRunning,
		ApplicationID: application.ID,
		Port:          0,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	if err := cp.RestoreProxyListeners(); err != nil {
		t.Fatalf("restore proxy listeners: %v", err)
	}
	restored, err := r.GetProxyByID(proxy.ID)
	if err != nil {
		t.Fatalf("get proxy: %v", err)
	}
	if restored.Port != 39001 {
		t.Fatalf("restored proxy port = %d, want allocated port persisted", restored.Port)
	}
	if len(pm.created) != 1 {
		t.Fatalf("runtime proxy was not restored")
	}
}

func TestRestoreProxyListenersRejectsAutoPortConflict(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	pm := newFakeProxyManager()
	cp.RegisterProxyManager(pm)

	_, application := createTestEdgeApplication(t, r)
	reserved := &model.Proxy{
		Name:          "reserved",
		Status:        model.ProxyStatusStopped,
		ApplicationID: application.ID,
		Port:          39001,
	}
	if err := r.CreateProxy(reserved); err != nil {
		t.Fatalf("create reserved proxy: %v", err)
	}
	legacy := &model.Proxy{
		Name:          "legacy",
		Status:        model.ProxyStatusRunning,
		ApplicationID: application.ID,
		Port:          0,
	}
	if err := r.CreateProxy(legacy); err != nil {
		t.Fatalf("create legacy proxy: %v", err)
	}

	if err := cp.RestoreProxyListeners(); err == nil {
		t.Fatalf("expected restore error for auto-assigned port conflict")
	}
	restored, err := r.GetProxyByID(legacy.ID)
	if err != nil {
		t.Fatalf("get legacy proxy: %v", err)
	}
	if restored.Port != 0 {
		t.Fatalf("legacy proxy port = %d, want 0 after failed restore", restored.Port)
	}
	if len(pm.created) != 0 {
		t.Fatalf("conflicting runtime proxy should be stopped")
	}
}
