package controlplane

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

type probeFrontier struct {
	frontierbound.FrontierBound
	calls  int
	reply  proto.TCPProbeResult
	target proto.TCPProbeRequest
}

func (f *probeFrontier) ProbeApplication(ctx context.Context, id uint64, target proto.TCPProbeRequest) (proto.TCPProbeResult, error) {
	f.calls++
	f.target = target
	return f.reply, nil
}

type savedProbeRepo struct{ *installationRepo }

func (r *savedProbeRepo) GetApplicationByID(id uint) (*model.Application, error) {
	return &model.Application{Model: gorm.Model{ID: id}, EdgeIDs: model.UintSlice{7}, IP: "saved.internal", Port: 3306}, nil
}
func TestApplicationProbeUsesSavedTarget(t *testing.T) {
	r := &savedProbeRepo{&installationRepo{visible: []uint64{7, 9}, edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}}
	f := &probeFrontier{reply: proto.TCPProbeResult{Version: 1, Status: "reachable"}}
	cp := &controlPlane{repo: r, frontierBound: f, probeSlots: make(chan struct{}, 4)}
	ctx := context.WithValue(context.Background(), "user_id", uint(2))
	_, err := cp.ProbeApplication(ctx, ApplicationProbeRequest{ApplicationID: 9})
	require.NoError(t, err)
	require.Equal(t, proto.TCPProbeRequest{Host: "saved.internal", Port: 3306}, f.target)
	_, err = cp.ProbeApplication(ctx, ApplicationProbeRequest{ApplicationID: 9, Host: "override.internal"})
	require.Error(t, err)
	require.Equal(t, 1, f.calls)
	r.visible = []uint64{7}
	_, err = cp.ProbeApplication(ctx, ApplicationProbeRequest{ApplicationID: 9})
	require.Error(t, err)
	require.Equal(t, 1, f.calls)
}
func TestApplicationProbeAuthorizationAndResult(t *testing.T) {
	for _, name := range []string{"reachable", "anonymous", "invisible", "offline", "invalid", "unknown version", "invalid status", "busy"} {
		t.Run(name, func(t *testing.T) {
			r := &installationRepo{visible: []uint64{7}, edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}
			f := &probeFrontier{reply: proto.TCPProbeResult{Version: 1, Status: "reachable"}}
			cp := &controlPlane{repo: r, frontierBound: f, probeSlots: make(chan struct{}, 4)}
			ctx := context.WithValue(context.Background(), "user_id", uint(2))
			req := ApplicationProbeRequest{EdgeID: 7, Host: "localhost", Port: 443}
			expected := "reachable"
			switch name {
			case "anonymous":
				ctx = context.Background()
			case "invisible":
				r.visible = nil
			case "offline":
				r.edge.Online = model.EdgeOnlineStatusOffline
				expected = "connector_offline"
			case "invalid":
				req.Host = "http://private"
			case "unknown version":
				f.reply.Version = 99
				expected = "edge_upgrade_required"
			case "invalid status":
				f.reply.Status = "secret detail"
				expected = "probe_unavailable"
			case "busy":
				for i := 0; i < 4; i++ {
					cp.probeLimiter.Allow()
				}
				expected = "busy"
			}
			got, err := cp.ProbeApplication(ctx, req)
			if name == "anonymous" || name == "invisible" || name == "invalid" {
				require.Error(t, err)
				require.Zero(t, f.calls)
				return
			}
			require.NoError(t, err)
			require.Equal(t, expected, got.Status)
			if name == "offline" || name == "busy" {
				require.Zero(t, f.calls)
			}
		})
	}
}
