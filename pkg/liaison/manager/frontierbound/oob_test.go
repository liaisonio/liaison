package frontierbound

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

func newTestRepo(t *testing.T) repo.Repo {
	t.Helper()
	conf := &config.Configuration{}
	conf.Manager.DB = t.TempDir() + "/liaison.db"

	r, err := repo.NewRepo(conf)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	return r
}

func TestUpdateEdgeHeartbeatMarksRunningEdgeOnline(t *testing.T) {
	r := newTestRepo(t)
	edge := &model.Edge{
		Name:   "edge-1",
		Status: model.EdgeStatusRunning,
		Online: model.EdgeOnlineStatusOffline,
	}
	if err := r.CreateEdge(edge); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	fb := &frontierBound{repo: r}
	fb.updateEdgeHeartbeat(uint64(edge.ID))

	updated, err := r.GetEdge(uint64(edge.ID))
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if updated.Online != model.EdgeOnlineStatusOnline {
		t.Fatalf("online = %d, want %d", updated.Online, model.EdgeOnlineStatusOnline)
	}
	if updated.HeartbeatAt.IsZero() {
		t.Fatalf("heartbeat_at was not updated")
	}
}

func TestUpdateEdgeHeartbeatMarksStoppedEdgeOnline(t *testing.T) {
	r := newTestRepo(t)
	edge := &model.Edge{
		Name:   "edge-1",
		Status: model.EdgeStatusStopped,
		Online: model.EdgeOnlineStatusOffline,
	}
	if err := r.CreateEdge(edge); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	fb := &frontierBound{repo: r}
	fb.updateEdgeHeartbeat(uint64(edge.ID))

	updated, err := r.GetEdge(uint64(edge.ID))
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if updated.Online != model.EdgeOnlineStatusOnline {
		t.Fatalf("online = %d, want %d", updated.Online, model.EdgeOnlineStatusOnline)
	}
	if updated.HeartbeatAt.IsZero() {
		t.Fatalf("heartbeat_at was not updated")
	}
}

func TestStoppedEdgeCanResolveID(t *testing.T) {
	r := newTestRepo(t)
	edge := &model.Edge{
		Name:   "edge-1",
		Status: model.EdgeStatusStopped,
		Online: model.EdgeOnlineStatusOffline,
	}
	if err := r.CreateEdge(edge); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	accessKey := &model.AccessKey{
		EdgeID:    edge.ID,
		AccessKey: "access-key",
		SecretKey: "secret-key",
	}
	if err := r.CreateAccessKey(accessKey); err != nil {
		t.Fatalf("create access key: %v", err)
	}

	meta, err := json.Marshal(proto.Meta{
		AccessKey: accessKey.AccessKey,
		SecretKey: accessKey.SecretKey,
	})
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	fb := &frontierBound{repo: r}
	id, err := fb.getID(meta)
	if err != nil {
		t.Fatalf("get id: %v", err)
	}
	if id != uint64(edge.ID) {
		t.Fatalf("edge id = %d, want %d", id, edge.ID)
	}

	if err := fb.online(id, meta, &net.TCPAddr{}); err != nil {
		t.Fatalf("online: %v", err)
	}
	updated, err := r.GetEdge(id)
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if updated.Status != model.EdgeStatusStopped {
		t.Fatalf("status = %d, want stopped", updated.Status)
	}
	if updated.Online != model.EdgeOnlineStatusOnline {
		t.Fatalf("online = %d, want online", updated.Online)
	}
}

func TestOfflineIgnoresStaleConnection(t *testing.T) {
	r := newTestRepo(t)
	edge := &model.Edge{
		Name:   "edge-1",
		Status: model.EdgeStatusRunning,
		Online: model.EdgeOnlineStatusOffline,
	}
	if err := r.CreateEdge(edge); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	fb := &frontierBound{repo: r}
	current := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10002}
	stale := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	if err := fb.online(uint64(edge.ID), nil, current); err != nil {
		t.Fatalf("online: %v", err)
	}
	if err := fb.offline(uint64(edge.ID), nil, stale); err != nil {
		t.Fatalf("stale offline: %v", err)
	}
	updated, err := r.GetEdge(uint64(edge.ID))
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if updated.Online != model.EdgeOnlineStatusOnline {
		t.Fatalf("stale offline changed online to %d", updated.Online)
	}

	if err := fb.offline(uint64(edge.ID), nil, current); err != nil {
		t.Fatalf("current offline: %v", err)
	}
	updated, err = r.GetEdge(uint64(edge.ID))
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if updated.Online != model.EdgeOnlineStatusOffline {
		t.Fatalf("current offline left online = %d, want offline", updated.Online)
	}
}
