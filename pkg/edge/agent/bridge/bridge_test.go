package bridge

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

type fakeAgent struct {
	events   chan rpc.Message
	done     chan struct{}
	once     sync.Once
	approved int
	mode     string
	answers  map[string][]string
}

func (f *fakeAgent) SetPermissionMode(mode string) error               { f.mode = mode; return nil }
func (f *fakeAgent) ApproveCommand(context.Context, rpc.Message) error { f.approved++; return nil }
func (f *fakeAgent) AnswerInput(_ context.Context, _ rpc.Message, answers map[string][]string) error {
	f.answers = answers
	return nil
}

func (f *fakeAgent) CheckAuthentication(context.Context) (bool, error) { return true, nil }
func (f *fakeAgent) NewThread(context.Context) (agentruntime.Thread, error) {
	return agentruntime.Thread{ID: "thread-owned"}, nil
}
func (f *fakeAgent) ResumeThread(_ context.Context, id string) (agentruntime.Thread, error) {
	return agentruntime.Thread{ID: id}, nil
}
func (f *fakeAgent) Send(context.Context, string, string) (string, error) { return "turn-owned", nil }
func (f *fakeAgent) Interrupt(context.Context, string, string) error      { return nil }
func (f *fakeAgent) RejectRequest(context.Context, rpc.Message) error     { return nil }
func (f *fakeAgent) Events() <-chan rpc.Message                           { return f.events }
func (f *fakeAgent) Done() <-chan struct{}                                { return f.done }
func (f *fakeAgent) Close() error                                         { f.once.Do(func() { close(f.done); close(f.events) }); return nil }

type fakeAdapter struct{ agent *fakeAgent }

func (fakeAdapter) Kind() string { return "fake" }
func (f fakeAdapter) Launch(ctx context.Context, _ discovery.Installation, _ string) (agentruntime.Session, error) {
	return f.agent, nil
}
func setup(t *testing.T) (*Bridge, *fakeAgent, proto.EdgeAgentRequest) {
	t.Helper()
	agent := &fakeAgent{events: make(chan rpc.Message, 8), done: make(chan struct{})}
	dir := t.TempDir()
	i := discovery.Installation{Agent: "fake", Path: "/installed/agent", ResolvedPath: "/installed/agent"}
	b, err := newBridge(context.Background(), fakeAdapter{agent}, func(context.Context) (discovery.Result, discovery.Environment, error) {
		return discovery.Result{Installations: []discovery.Installation{i}}, discovery.Environment{OS: "darwin", AccountID: "501", Home: dir}, nil
	}, filepath.Join(dir, "agent-bindings.json"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, b.Close()) })
	req := proto.EdgeAgentRequest{Action: "start", InstallationID: installationID(i), Project: dir}
	return b, agent, req
}
func call(b *Bridge, actor string, req proto.EdgeAgentRequest) proto.EdgeAgentResult {
	return b.Handle(context.Background(), proto.EdgeAgentRPCRequest{Version: 1, ActorID: actor, Request: req})
}
func TestOwnerIsolationAndLiveTranscript(t *testing.T) {
	b, a, req := setup(t)
	r := call(b, "alice", req)
	require.Equal(t, "ok", r.Status)
	require.Len(t, r.SessionID, 32)
	for _, action := range []string{"poll", "send", "interrupt", "stop"} {
		x := proto.EdgeAgentRequest{Action: action, SessionID: r.SessionID}
		if action == "send" {
			x.Text = "hello"
		}
		require.Equal(t, "not_found", call(b, "bob", x).Status)
	}
	r = call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: r.SessionID, Text: "hello"})
	require.True(t, r.Running)
	require.Equal(t, "busy", call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: r.SessionID, Text: "duplicate"}).Status)
	a.events <- rpc.Message{Method: "item/agentMessage/delta", Params: json.RawMessage(`{"threadId":"foreign","turnId":"turn-owned","delta":"secret"}`)}
	a.events <- rpc.Message{Method: "item/agentMessage/delta", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn-owned","delta":"Hello"}`)}
	a.events <- rpc.Message{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-owned","turn":{"id":"turn-owned","status":"completed"}}`)}
	require.Eventually(t, func() bool {
		return !call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID}).Running
	}, time.Second, time.Millisecond)
	r = call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID})
	require.Equal(t, "Hello", r.Messages[1].Text)
	r = call(b, "alice", proto.EdgeAgentRequest{Action: "stop", SessionID: r.SessionID})
	require.True(t, r.Closed)
	select {
	case <-a.done:
	default:
		t.Fatal("owned process not closed")
	}
}
func TestLeaseAndOutputBounds(t *testing.T) {
	t.Run("lease", func(t *testing.T) {
		b, a, req := setup(t)
		r := call(b, "alice", req)
		b.mu.Lock()
		s := b.sessions[r.SessionID]
		b.mu.Unlock()
		s.mu.Lock()
		s.touched = time.Now().Add(-2 * time.Minute)
		s.mu.Unlock()
		select {
		case <-a.done:
		case <-time.After(2 * time.Second):
			t.Fatal("lease did not close session")
		}
	})
	t.Run("output", func(t *testing.T) {
		b, a, req := setup(t)
		r := call(b, "alice", req)
		call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: r.SessionID, Text: "hello"})
		data, err := json.Marshal(map[string]string{"threadId": "thread-owned", "turnId": "turn-owned", "delta": strings.Repeat("a", maxOutput)})
		require.NoError(t, err)
		a.events <- rpc.Message{Method: "item/agentMessage/delta", Params: data}
		require.Eventually(t, func() bool {
			return call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID}).Truncated
		}, time.Second, time.Millisecond)
		snapshot := call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID})
		require.Equal(t, "ok", snapshot.Status)
		require.True(t, snapshot.Running)
		require.False(t, snapshot.Closed)
		require.LessOrEqual(t, len(snapshot.Messages[1].Text), 64<<10)
	})
}

func TestEntryBindingAndSessionDetails(t *testing.T) {
	b, _, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	r := call(b, "alice", req)
	require.Equal(t, "ok", r.Status)
	require.Equal(t, "thread-owned", r.ThreadID)
	require.NotEmpty(t, r.Project)
	require.NotEmpty(t, r.StartedAt)
	for _, id := range []string{"", strings.Repeat("b", 32)} {
		require.Equal(t, "not_found", call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID, AccessID: id}).Status)
	}
	require.Equal(t, "ok", call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID, AccessID: req.AccessID}).Status)
	require.Equal(t, "invalid_request", call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: r.SessionID, AccessID: req.AccessID, Text: "hello", SkillID: strings.Repeat("c", 32)}).Status)
}
func TestDiscoveryAndLaunchRejectInvalidInputs(t *testing.T) {
	b, _, req := setup(t)
	r := call(b, "alice", proto.EdgeAgentRequest{Action: "discover"})
	require.Len(t, r.Installations, 1)
	req.InstallationID = strings.Repeat("0", 32)
	require.Equal(t, "not_found", call(b, "alice", req).Status)
	req.Project = "relative"
	require.Equal(t, "invalid_request", call(b, "alice", req).Status)
	require.Equal(t, "invalid_request", call(b, "", proto.EdgeAgentRequest{Action: "discover"}).Status)
}
