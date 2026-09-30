package bridge

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

// Deliberately implements neither Codex RPC nor an auth preflight.
type normalizedAgent struct {
	updates chan agentruntime.Update
	done    chan struct{}
	once    sync.Once
}

func (a *normalizedAgent) NewThread(context.Context) (agentruntime.Thread, error) {
	return agentruntime.Thread{ID: "owned"}, nil
}
func (a *normalizedAgent) Send(context.Context, string, string) (string, error) { return "turn", nil }
func (a *normalizedAgent) Interrupt(context.Context, string, string) error      { return nil }
func (a *normalizedAgent) Done() <-chan struct{}                                { return a.done }
func (a *normalizedAgent) Updates() <-chan agentruntime.Update                  { return a.updates }
func (a *normalizedAgent) Close() error {
	a.once.Do(func() { close(a.done); close(a.updates) })
	return nil
}

type normalizedAdapter struct{ a *normalizedAgent }

func (normalizedAdapter) Kind() string { return "normalized" }
func (a normalizedAdapter) Launch(context.Context, discovery.Installation, string) (agentruntime.Session, error) {
	return a.a, nil
}

func TestNormalizedBridgeRoundtrip(t *testing.T) {
	dir := t.TempDir()
	a := &normalizedAgent{updates: make(chan agentruntime.Update, 8), done: make(chan struct{})}
	i := discovery.Installation{Agent: "normalized", Path: "/installed/agent", ResolvedPath: "/installed/agent"}
	b, err := newBridge(t.Context(), normalizedAdapter{a}, func(context.Context) (discovery.Result, discovery.Environment, error) {
		return discovery.Result{Installations: []discovery.Installation{i}}, discovery.Environment{OS: "darwin", AccountID: "501", Home: dir}, nil
	}, filepath.Join(dir, "bindings.json"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, b.Close()) })
	r := call(b, "alice", proto.EdgeAgentRequest{Action: "start", InstallationID: installationID(i), Project: dir})
	require.Equal(t, "ok", r.Status)
	id := r.SessionID
	r = call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: id, Text: "hello"})
	require.True(t, r.Running)
	require.Equal(t, "not_found", call(b, "bob", proto.EdgeAgentRequest{Action: "poll", SessionID: id}).Status)
	a.updates <- agentruntime.Update{Kind: agentruntime.MessageDelta, ThreadID: "foreign", TurnID: "turn", Text: "ignored"}
	a.updates <- agentruntime.Update{Kind: agentruntime.MessageDelta, ThreadID: "owned", TurnID: "turn", ItemID: "m", Text: "hello"}
	a.updates <- agentruntime.Update{Kind: agentruntime.TurnEnded, ThreadID: "owned", TurnID: "turn", Status: "completed"}
	require.Eventually(t, func() bool { return !call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: id}).Running }, time.Second, time.Millisecond)
	r = call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: id})
	require.Len(t, r.Messages, 2)
	require.Equal(t, "hello", r.Messages[1].Text)
	require.NotNil(t, r.TurnTiming)
	require.NotNil(t, r.TurnTiming.DispatchMS)
	require.NotNil(t, r.TurnTiming.FirstReplyMS)
	require.NotNil(t, r.TurnTiming.FinishedMS)
	require.True(t, call(b, "alice", proto.EdgeAgentRequest{Action: "stop", SessionID: id}).Closed)
}
