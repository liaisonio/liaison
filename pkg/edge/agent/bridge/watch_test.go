package bridge

import (
	"context"
	"encoding/json"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
	"time"
)

func TestWatchWakesOnRealEventsAndSeparatesMessages(t *testing.T) {
	b, a, req := setup(t)
	r := call(b, "alice", req)
	sent := call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: r.SessionID, Text: "hello"})
	watch := proto.EdgeAgentRequest{Action: "watch", SessionID: r.SessionID, Cursor: strconv.FormatUint(sent.Revision, 10)}
	require.Equal(t, "not_found", call(b, "bob", watch).Status)
	done := make(chan proto.EdgeAgentResult, 1)
	go func() { done <- call(b, "alice", watch) }()
	select {
	case <-done:
		t.Fatal("watch returned without a change")
	case <-time.After(20 * time.Millisecond):
	}
	started := time.Now()
	a.events <- rpc.Message{Method: "item/agentMessage/delta", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn-owned","itemId":"intro","delta":"Checking."}`)}
	select {
	case update := <-done:
		require.Greater(t, update.Revision, sent.Revision)
		require.Less(t, time.Since(started), 500*time.Millisecond)
	case <-time.After(time.Second):
		t.Fatal("watch was not woken")
	}
	a.events <- rpc.Message{Method: "item/completed", Params: json.RawMessage(`{"threadId":"foreign","turnId":"turn-owned","item":{"id":"foreign","type":"commandExecution","status":"completed"}}`)}
	a.events <- rpc.Message{Method: "item/completed", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn-owned","item":{"id":"tool","type":"commandExecution","status":"completed","command":"secret-token","aggregatedOutput":"secret-output","durationMs":45}}`)}
	a.events <- rpc.Message{Method: "item/agentMessage/delta", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn-owned","itemId":"final","delta":"Done."}`)}
	a.events <- rpc.Message{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-owned","turn":{"id":"turn-owned","status":"completed"}}`)}
	require.Eventually(t, func() bool {
		return !call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID}).Running
	}, time.Second, time.Millisecond)
	final := call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID})
	require.Len(t, final.Messages, 3)
	require.Equal(t, "Done.", final.Messages[2].Text)
	require.Len(t, final.Activities, 1)
	encoded, err := json.Marshal(final)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret")
	require.NotContains(t, string(encoded), "foreign")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	watch.Cursor = strconv.FormatUint(final.Revision, 10)
	require.Equal(t, "unavailable", b.Handle(ctx, proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", Request: watch}).Status)
}

func TestActivityProjectionBounds(t *testing.T) {
	s := &session{}
	for i := 0; i < 200; i++ {
		s.recordActivityLocked(strconv.Itoa(i), "commandExecution", "inProgress", 0, false)
	}
	require.Len(t, s.activities, 128)
	s.recordActivityLocked("0", "commandExecution", "declined", -1, true)
	require.Equal(t, "declined", s.activities[len(s.activities)-1].Status)
	require.Zero(t, s.activities[len(s.activities)-1].DurationMS)
	require.True(t, s.truncated)
	s.recordActivityLocked("unknown", "arbitrary-raw-data", "completed", 0, true)
	require.Len(t, s.activities, 128)
}
