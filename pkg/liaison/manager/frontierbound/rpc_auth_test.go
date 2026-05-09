package frontierbound

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
)

type fakeRequest struct {
	clientID uint64
	data     []byte
	timeout  time.Duration
}

func (r *fakeRequest) ID() uint64                       { return 1 }
func (r *fakeRequest) StreamID() uint64                 { return 0 }
func (r *fakeRequest) ClientID() uint64                 { return r.clientID }
func (r *fakeRequest) Method() string                   { return "" }
func (r *fakeRequest) Timeout() time.Duration           { return r.timeout }
func (r *fakeRequest) Data() []byte                     { return r.data }
func (r *fakeRequest) Custom() []byte                   { return nil }
func (r *fakeRequest) SetTimeout(timeout time.Duration) { r.timeout = timeout }
func (r *fakeRequest) SetCustom([]byte)                 {}
func (r *fakeRequest) SetClientID(clientID uint64)      { r.clientID = clientID }
func (r *fakeRequest) SetStreamID(uint64)               {}

type fakeResponse struct {
	err      error
	data     []byte
	clientID uint64
	streamID uint64
	custom   []byte
}

func (r *fakeResponse) ID() uint64                  { return 1 }
func (r *fakeResponse) StreamID() uint64            { return r.streamID }
func (r *fakeResponse) ClientID() uint64            { return r.clientID }
func (r *fakeResponse) Method() string              { return "" }
func (r *fakeResponse) Data() []byte                { return r.data }
func (r *fakeResponse) Error() error                { return r.err }
func (r *fakeResponse) Custom() []byte              { return r.custom }
func (r *fakeResponse) SetData(data []byte)         { r.data = data }
func (r *fakeResponse) SetError(err error)          { r.err = err }
func (r *fakeResponse) SetCustom(custom []byte)     { r.custom = custom }
func (r *fakeResponse) SetClientID(clientID uint64) { r.clientID = clientID }
func (r *fakeResponse) SetStreamID(streamID uint64) { r.streamID = streamID }

func createFrontierboundTestEdge(t *testing.T, r repo.Repo, name string) *model.Edge {
	t.Helper()
	edge := &model.Edge{
		Name:   name,
		Status: model.EdgeStatusRunning,
		Online: model.EdgeOnlineStatusOnline,
	}
	if err := r.CreateEdge(edge); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	return edge
}

func newFakeRequest(t *testing.T, edgeID uint64, payload any) *fakeRequest {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return &fakeRequest{clientID: edgeID, data: data}
}

func TestReportDeviceRejectsDeletedEdge(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()
	edge := createFrontierboundTestEdge(t, r, "edge-1")
	if err := r.DeleteEdge(uint64(edge.ID)); err != nil {
		t.Fatalf("delete edge: %v", err)
	}

	fb := &frontierBound{repo: r}
	req := newFakeRequest(t, uint64(edge.ID), proto.Device{
		Fingerprint: "device-fingerprint",
		HostName:    "device-1",
		EdgeID:      uint64(edge.ID),
	})
	rsp := &fakeResponse{}
	fb.reportDevice(context.Background(), req, rsp)

	if rsp.Error() == nil {
		t.Fatalf("expected deleted edge report to be rejected")
	}
	if _, err := r.GetDeviceByFingerprint("device-fingerprint"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("device should not be created after deleted edge report, err=%v", err)
	}
}

func TestReportDeviceBindsHostRelationToConnectionEdge(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()
	edge := createFrontierboundTestEdge(t, r, "edge-1")

	fb := &frontierBound{repo: r}
	req := newFakeRequest(t, uint64(edge.ID), proto.Device{
		Fingerprint: "device-fingerprint",
		HostName:    "device-1",
	})
	rsp := &fakeResponse{}
	fb.reportDevice(context.Background(), req, rsp)
	if rsp.Error() != nil {
		t.Fatalf("report device: %v", rsp.Error())
	}

	device, err := r.GetDeviceByFingerprint("device-fingerprint")
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	relation, err := r.GetEdgeDevice(uint64(edge.ID), device.ID, model.EdgeDeviceRelationHost)
	if err != nil {
		t.Fatalf("get edge device relation: %v", err)
	}
	if relation == nil {
		t.Fatalf("expected host relation for connection edge")
	}
}

func TestReportDeviceRejectsPayloadEdgeMismatch(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()
	edge1 := createFrontierboundTestEdge(t, r, "edge-1")
	edge2 := createFrontierboundTestEdge(t, r, "edge-2")

	fb := &frontierBound{repo: r}
	req := newFakeRequest(t, uint64(edge1.ID), proto.Device{
		Fingerprint: "device-fingerprint",
		HostName:    "device-1",
		EdgeID:      uint64(edge2.ID),
	})
	rsp := &fakeResponse{}
	fb.reportDevice(context.Background(), req, rsp)

	if rsp.Error() == nil {
		t.Fatalf("expected mismatched edge id to be rejected")
	}
}

func TestPullTaskScanApplicationRejectsPayloadEdgeMismatch(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()
	edge1 := createFrontierboundTestEdge(t, r, "edge-1")
	edge2 := createFrontierboundTestEdge(t, r, "edge-2")

	fb := &frontierBound{repo: r}
	req := newFakeRequest(t, uint64(edge1.ID), proto.PullTaskScanApplicationRequest{
		EdgeID: uint64(edge2.ID),
	})
	rsp := &fakeResponse{}
	fb.pullTaskScanApplication(context.Background(), req, rsp)

	if rsp.Error() == nil {
		t.Fatalf("expected mismatched task pull to be rejected")
	}
}

func TestUpdateDeviceHeartbeatRequiresDiscoveredRelation(t *testing.T) {
	r := newTestRepo(t)
	defer r.Close()
	edge := createFrontierboundTestEdge(t, r, "edge-1")
	device := &model.Device{
		Fingerprint: "device-fingerprint",
		Name:        "device-1",
		HostName:    "device-1",
	}
	if err := r.CreateDevice(device); err != nil {
		t.Fatalf("create device: %v", err)
	}

	fb := &frontierBound{repo: r}
	req := newFakeRequest(t, uint64(edge.ID), proto.UpdateDeviceHeartbeatRequest{
		DeviceID: uint64(device.ID),
	})
	rsp := &fakeResponse{}
	fb.updateDeviceHeartbeat(context.Background(), req, rsp)

	if rsp.Error() == nil {
		t.Fatalf("expected heartbeat for undiscovered device to be rejected")
	}
}

func TestKickEdgeCallsFrontierControlPlane(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/edges/42" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fb := &frontierBound{
		controlPlaneURL: server.URL,
		httpClient:      server.Client(),
		edgeConnAddr:    map[uint64]string{42: "127.0.0.1:12345"},
	}
	if err := fb.KickEdge(context.Background(), 42); err != nil {
		t.Fatalf("kick edge: %v", err)
	}
	if !called {
		t.Fatalf("expected controlplane request")
	}
	if _, ok := fb.edgeConnAddr[42]; ok {
		t.Fatalf("expected remembered edge connection to be cleared")
	}
}
