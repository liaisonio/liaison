package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	"github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

type factoryAdapter struct {
	latest **fakeAgent
}

func (factoryAdapter) Kind() string { return "fake" }
func (a factoryAdapter) Launch(context.Context, discovery.Installation, string) (runtime.Session, error) {
	agent := &fakeAgent{events: make(chan rpc.Message, 8), done: make(chan struct{})}
	*a.latest = agent
	return agent, nil
}

func TestNativeBindingSurvivesBridgeRestart(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "agent-bindings.json")
	installation := discovery.Installation{Agent: "fake", Path: "/installed/agent", ResolvedPath: "/installed/agent"}
	discover := func(context.Context) (discovery.Result, discovery.Environment, error) {
		return discovery.Result{Installations: []discovery.Installation{installation}}, discovery.Environment{OS: "darwin", AccountID: "501", Home: dir}, nil
	}
	var latest *fakeAgent
	first, err := newBridge(context.Background(), factoryAdapter{latest: &latest}, discover, store)
	require.NoError(t, err)
	access := strings.Repeat("a", 32)
	started := first.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", ProjectRoot: dir, Request: proto.EdgeAgentRequest{Action: "start", AccessID: access, InstallationID: installationID(installation), Project: dir}})
	require.Equal(t, "ok", started.Status)
	require.Equal(t, "thread-owned", started.ThreadID)
	seed := &proto.EdgeAgentResume{SessionID: started.SessionID, Revision: 7, StartedAt: started.StartedAt, Model: "native-model", Title: "Persisted", Messages: []proto.EdgeAgentMessage{{Role: "user", Text: "remember me"}}}
	request := proto.EdgeAgentRequest{Action: "resume", AccessID: access, SessionID: started.SessionID}
	seed.StartedAt = time.Now().Add(-48 * time.Hour).Format(time.RFC3339)
	ended := first.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", Request: proto.EdgeAgentRequest{Action: "stop", AccessID: access, SessionID: started.SessionID}})
	require.True(t, ended.Closed)
	reopenedLive := first.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", ProjectRoot: dir, Request: request, Resume: seed})
	require.Equal(t, "ok", reopenedLive.Status)
	require.False(t, reopenedLive.Closed)
	require.NoError(t, first.Close())
	// 同一启动路径升级软链接目标后，旧 session 的安装绑定仍可恢复。
	previousInstallationID := installationID(installation)
	installation.ResolvedPath = "/installed/versions/v2/agent"
	require.NotEqual(t, previousInstallationID, installationID(installation))

	second, err := newBridge(context.Background(), factoryAdapter{latest: &latest}, discover, store)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, second.Close()) })
	resumed := second.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", ProjectRoot: dir, Request: request, Resume: seed})
	require.Equal(t, "ok", resumed.Status)
	require.Equal(t, started.SessionID, resumed.SessionID)
	require.Equal(t, started.ThreadID, resumed.ThreadID)
	require.Equal(t, uint64(8), resumed.Revision)
	require.Equal(t, "remember me", resumed.Messages[0].Text)
	require.False(t, resumed.Closed)
	current := second.sessions[started.SessionID]
	current.mu.Lock()
	expire, forget := current.expirationLocked(time.Now())
	current.mu.Unlock()
	require.False(t, expire, "Resuming old history starts a fresh runtime lifetime")
	require.False(t, forget)

	foreign := second.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: "bob", ProjectRoot: dir, Request: request, Resume: seed})
	require.Equal(t, "resume_unavailable", foreign.Status)
	foreignAccess := request
	foreignAccess.AccessID = strings.Repeat("b", 32)
	denied := second.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", ProjectRoot: dir, Request: foreignAccess, Resume: seed})
	require.Equal(t, "resume_unavailable", denied.Status)
	// 停止只结束运行实例，不移除 owner/access/project 的持久绑定。
	stopped := second.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", Request: proto.EdgeAgentRequest{Action: "stop", AccessID: access, SessionID: started.SessionID}})
	require.True(t, stopped.Closed)
	wrongProject := second.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", ProjectRoot: t.TempDir(), Request: request, Resume: seed})
	require.Equal(t, "resume_unavailable", wrongProject.Status)
	stored, err := loadBindings(store)
	require.NoError(t, err)
	require.Contains(t, stored, bindingKey("alice", access, started.SessionID))
	// 安装身份包含 provider，另一种 Agent 不能复用相同可执行路径的绑定。
	changed := installation
	changed.Agent = "another-agent"
	require.NotEqual(t, installationID(installation), installationID(changed))
}
