package controlplane

import (
	"context"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type agentRepo struct {
	*installationRepo
	owner uint
}

type entryAgentRepo struct {
	*agentRepo
	entry *model.AgentAccess
}

func (r *entryAgentRepo) GetUserByID(uint) (*model.User, error)              { return nil, gorm.ErrRecordNotFound }
func (r *entryAgentRepo) CreateManagementAudit(*model.ManagementAudit) error { return nil }

func (r *entryAgentRepo) GetAgentAccess(_ context.Context, owner uint, id string) (*model.AgentAccess, error) {
	if r.entry == nil || r.entry.OwnerID != owner || r.entry.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return r.entry, nil
}
func TestAgentAccessRecheckedBeforeEveryRPC(t *testing.T) {
	id := strings.Repeat("a", 32)
	r := &entryAgentRepo{agentRepo: &agentRepo{installationRepo: &installationRepo{edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}, owner: 2}, entry: &model.AgentAccess{ID: id, OwnerID: 2, EdgeID: 7, InstallationID: strings.Repeat("b", 32), Project: "/project"}}
	f := &agentFrontier{}
	cp := &controlPlane{repo: r, frontierBound: f, authorizeFeature: func(context.Context, string) error { return nil }}
	ctx := context.WithValue(context.Background(), "user_id", uint(2))
	for _, action := range []string{"poll", "send", "interrupt", "stop"} {
		req := proto.EdgeAgentRequest{EdgeID: 7, AccessID: id, SessionID: strings.Repeat("s", 32), Action: action}
		if action == "send" {
			req.Text = "hello"
		}
		r.entry.OwnerID = 3
		_, err := cp.EdgeAgent(ctx, req)
		require.Error(t, err)
		require.Zero(t, f.calls)
		r.entry.OwnerID = 2
		r.entry.EdgeID = 8
		_, err = cp.EdgeAgent(ctx, req)
		require.Error(t, err)
		require.Zero(t, f.calls)
		r.entry.EdgeID = 7
	}
	_, err := cp.EdgeAgent(ctx, proto.EdgeAgentRequest{Action: "start", EdgeID: 7, AccessID: id, InstallationID: r.entry.InstallationID, Project: "/other"})
	require.Error(t, err)
	require.Zero(t, f.calls)
	_, err = cp.EdgeAgent(ctx, proto.EdgeAgentRequest{Action: "start", EdgeID: 7, AccessID: id, InstallationID: r.entry.InstallationID, Project: "/project"})
	require.NoError(t, err)
	require.Equal(t, 1, f.calls)
	r.entry = nil
	_, err = cp.EdgeAgent(ctx, proto.EdgeAgentRequest{Action: "poll", EdgeID: 7, AccessID: id, SessionID: strings.Repeat("s", 32)})
	require.Error(t, err)
	require.Equal(t, 1, f.calls)
}

func (r *agentRepo) ListIAMResourceRelations(string, uint64) ([]*model.IAMResourceRelation, error) {
	return []*model.IAMResourceRelation{{Relation: model.IAMRelationOwner, SubjectType: model.IAMSubjectUser, SubjectID: r.owner}}, nil
}

type agentFrontier struct {
	frontierbound.FrontierBound
	calls       int
	actor       string
	afterCall   func()
	projectRoot string
}

func (f *agentFrontier) EdgeAgent(_ context.Context, _ uint64, r proto.EdgeAgentRPCRequest) (proto.EdgeAgentResult, error) {
	f.calls++
	f.actor = r.ActorID
	f.projectRoot = r.ProjectRoot
	if f.afterCall != nil {
		f.afterCall()
	}
	return proto.EdgeAgentResult{Version: 1, Status: "ok"}, nil
}
func TestEdgeAgentRechecksOwnerAndFeature(t *testing.T) {
	r := &agentRepo{installationRepo: &installationRepo{visible: []uint64{7}, edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}, owner: 2}
	f := &agentFrontier{}
	cp := &controlPlane{repo: r, frontierBound: f, authorizeFeature: func(context.Context, string) error { return nil }}
	ctx := context.WithValue(context.Background(), "user_id", uint(2))
	_, err := cp.EdgeAgent(ctx, proto.EdgeAgentRequest{EdgeID: 7, Action: "discover"})
	require.NoError(t, err)
	require.Equal(t, "2", f.actor)
	for _, action := range []string{"poll", "send", "interrupt", "stop"} {
		r.owner = 3
		req := proto.EdgeAgentRequest{EdgeID: 7, Action: action, SessionID: "01234567890123456789012345678901"}
		if action == "send" {
			req.Text = "hello"
		}
		_, err = cp.EdgeAgent(ctx, req)
		require.Error(t, err)
		require.Equal(t, 1, f.calls)
	}
	r.owner = 2
	cp.authorizeFeature = func(context.Context, string) error { return iam.ErrForbidden }
	_, err = cp.EdgeAgent(ctx, proto.EdgeAgentRequest{EdgeID: 7, Action: "discover"})
	require.ErrorIs(t, err, iam.ErrForbidden)
	require.Equal(t, 1, f.calls)
	_, err = cp.EdgeAgent(context.Background(), proto.EdgeAgentRequest{EdgeID: 7, Action: "discover"})
	require.ErrorIs(t, err, iam.ErrForbidden)
}

func TestWatchRechecksRevocationAfterWaiting(t *testing.T) {
	id := strings.Repeat("a", 32)
	r := &entryAgentRepo{agentRepo: &agentRepo{installationRepo: &installationRepo{edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}, owner: 2}, entry: &model.AgentAccess{ID: id, OwnerID: 2, EdgeID: 7, Project: "/saved-project"}}
	f := &agentFrontier{}
	cp := &controlPlane{repo: r, frontierBound: f, authorizeFeature: func(context.Context, string) error { return nil }}
	ctx := context.WithValue(context.Background(), "user_id", uint(2))
	req := proto.EdgeAgentRequest{Action: "watch", EdgeID: 7, AccessID: id, SessionID: strings.Repeat("s", 32), Cursor: "1"}
	_, err := cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "/saved-project", f.projectRoot)
	f.afterCall = func() { r.owner = 3 }
	_, err = cp.EdgeAgent(ctx, req)
	require.ErrorIs(t, err, iam.ErrForbidden)
	r.owner = 2
	f.afterCall = func() { r.entry = nil }
	_, err = cp.EdgeAgent(ctx, req)
	require.Error(t, err)
}
