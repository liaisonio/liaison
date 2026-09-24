package controlplane

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"path/filepath"
	"strings"
	"testing"
)

type historyRepo struct {
	repo.Repo
	online bool
	owner  uint
}

func (r *historyRepo) GetEdge(uint64) (*model.Edge, error) {
	e := &model.Edge{Status: model.EdgeStatusRunning}
	if r.online {
		e.Online = model.EdgeOnlineStatusOnline
	}
	return e, nil
}
func (r *historyRepo) ListIAMResourceRelations(string, uint64) ([]*model.IAMResourceRelation, error) {
	return []*model.IAMResourceRelation{{Relation: model.IAMRelationOwner, SubjectType: model.IAMSubjectUser, SubjectID: r.owner}}, nil
}
func (r *historyRepo) GetUserByID(uint) (*model.User, error)              { return nil, gorm.ErrRecordNotFound }
func (r *historyRepo) CreateManagementAudit(*model.ManagementAudit) error { return nil }

type historyFrontier struct {
	frontierbound.FrontierBound
	out   proto.EdgeAgentResult
	calls int
	last  proto.EdgeAgentRPCRequest
}

func (f *historyFrontier) EdgeAgent(_ context.Context, _ uint64, req proto.EdgeAgentRPCRequest) (proto.EdgeAgentResult, error) {
	f.calls++
	f.last = req
	return f.out, nil
}

func TestHistoryOfflineRestartEncryptionAndBackgroundSync(t *testing.T) {
	d, err := dao.NewDao(&config.Configuration{Manager: config.Manager{DB: filepath.Join(t.TempDir(), "history.db")}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })
	r := &historyRepo{Repo: d, owner: 1}
	f := &historyFrontier{}
	allowed := true
	auth := func(context.Context, string) error {
		if !allowed {
			return errors.New("denied")
		}
		return nil
	}
	key, err := newHistoryCipher("test-only-history-secret")
	require.NoError(t, err)
	cp := &controlPlane{repo: r, frontierBound: f, historyCipher: key, authorizeFeature: auth}
	ctx := context.WithValue(context.Background(), "user_id", uint(1))
	access := strings.Repeat("a", 32)
	id := strings.Repeat("b", 32)
	require.NoError(t, d.SaveAgentAccess(ctx, &model.AgentAccess{ID: access, OwnerID: 1, EdgeID: 7, Project: "/project"}, true))
	scope := &model.EdgeAgentHistory{OwnerID: 1, AccessID: access, EdgeID: 7, SessionID: id}
	snapshot := proto.EdgeAgentResult{Version: 1, Status: "ok", SessionID: id, ThreadID: "native-thread", Project: "/project", Model: "local-model", Revision: 1, Messages: []proto.EdgeAgentMessage{{Role: "user", Text: "Private content"}}}
	snapshot.Approvals = []proto.AgentApproval{{ID: strings.Repeat("c", 32), Command: "private pending command"}}
	snapshot.PermissionsAvailable = true
	snapshot.InputRequests = []proto.AgentInputRequest{{ID: strings.Repeat("d", 32)}}
	snapshot.Activities = []proto.AgentActivity{{ID: "tool", Kind: "commandExecution", Status: "completed", Command: "pwd", Output: "/project", MessageIndex: 0}, {ID: "diff", Kind: "fileChange", Status: "completed", MessageIndex: 0, Changes: []proto.AgentFileChange{{Path: "app.go", Kind: "update", Diff: "-old\n+new"}}}}
	require.NoError(t, cp.saveHistory(ctx, scope, snapshot))
	stored, err := d.GetEdgeAgentHistory(ctx, scope)
	require.NoError(t, err)
	require.NotContains(t, string(stored.Payload), "Private content")
	require.NoError(t, cp.archiveTranscript(ctx, scope, snapshot))
	page, err := d.PreviousEdgeAgentHistoryPage(ctx, scope, 1)
	require.NoError(t, err)
	require.NotContains(t, string(page.Payload), "Private content")
	pageResult, err := cp.EdgeAgent(ctx, proto.EdgeAgentRequest{Action: "transcript", AccessID: access, EdgeID: 7, SessionID: id, HistoryBefore: "1"})
	require.NoError(t, err)
	require.Equal(t, snapshot.Messages, pageResult.Messages)
	require.Empty(t, pageResult.InputRequests)
	require.Empty(t, pageResult.Approvals)
	_, err = cp.EdgeAgent(context.WithValue(ctx, "user_id", uint(2)), proto.EdgeAgentRequest{Action: "transcript", AccessID: access, EdgeID: 7, SessionID: id, HistoryBefore: "1"})
	require.Error(t, err)
	// Recreate the control plane: only the repository and stable key survive.
	cp = &controlPlane{repo: r, frontierBound: f, historyCipher: key, authorizeFeature: auth}
	req := proto.EdgeAgentRequest{Action: "poll", AccessID: access, EdgeID: 7, SessionID: id}
	out, err := cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.True(t, out.Archived)
	require.True(t, out.Closed)
	require.Equal(t, "Private content", out.Messages[0].Text)
	require.Empty(t, out.Approvals, "pending native approvals must not survive in archived history")
	require.False(t, out.PermissionsAvailable)
	require.Empty(t, out.InputRequests, "pending questions must never be replayed from history")
	require.Equal(t, "/project", out.Activities[0].Output)
	require.Equal(t, "-old\n+new", out.Activities[1].Changes[0].Diff)
	require.Zero(t, f.calls)
	allowed = false
	_, err = cp.EdgeAgent(ctx, req)
	require.Error(t, err)
	allowed = true
	r.owner = 2
	_, err = cp.EdgeAgent(ctx, req)
	require.Error(t, err)
	r.owner = 1
	r.online = true
	f.out = snapshot
	f.out.Revision = 2
	f.out.Messages = append(f.out.Messages, proto.EdgeAgentMessage{Role: "assistant", Text: "Background answer"})
	cp.syncAgentHistory(context.Background(), *scope)
	r.online = false
	out, err = cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.Len(t, out.Messages, 2)
	r.online = true
	req.Action = "resume"
	f.out = snapshot
	f.out.Messages = append(f.out.Messages, proto.EdgeAgentMessage{Role: "assistant", Text: "Background answer"})
	f.out.Revision = 3
	f.out.Closed = false
	out, err = cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "ok", out.Status)
	require.NotNil(t, f.last.Resume)
	require.Equal(t, id, f.last.Resume.SessionID)
	require.Len(t, f.last.Resume.Messages, 2)
	require.Equal(t, uint64(2), f.last.Resume.Revision)
	r.online = false
	req.Action = "rename"
	req.Title = "Saved name"
	out, err = cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "Saved name", out.Title)
	req.Title = ""
	req.Action = "sessions"
	req.SessionID = ""
	out, err = cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.Len(t, out.Sessions, 1)
	require.Equal(t, "Saved name", out.Sessions[0].Title)
	req.SessionID = id
	req.Action = "delete"
	out, err = cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "ok", out.Status)
	snapshot.Revision = 100
	require.NoError(t, cp.saveHistory(ctx, scope, snapshot))
	req.Action = "poll"
	out, err = cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "not_found", out.Status)
	wrong := *stored
	wrong.EdgeID = 8
	_, err = cp.historySnapshot(&wrong)
	require.Error(t, err)
}
