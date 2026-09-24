package bridge

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestApprovalScopeAndOneShot(t *testing.T) {
	b, a, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	r := call(b, "alice", req)
	s := b.sessions[r.SessionID]
	capable := a
	s.mu.Lock()
	s.running = true
	s.turn = "turn"
	s.mu.Unlock()
	event := rpc.Message{ID: json.RawMessage(`1`), Method: "item/commandExecution/requestApproval", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn","command":"ls /tmp","cwd":"/tmp","reason":"inspect"}`)}
	require.True(t, s.queueApproval(event))
	require.True(t, s.queueApproval(event))
	require.Len(t, s.snapshot().Approvals, 1)
	q := proto.EdgeAgentRequest{Action: "approve", AccessID: req.AccessID, SessionID: r.SessionID, ApprovalID: s.snapshot().Approvals[0].ID, Decision: "accept"}
	require.Equal(t, "not_found", call(b, "bob", q).Status)
	foreign := q
	foreign.AccessID = strings.Repeat("b", 32)
	require.Equal(t, "not_found", call(b, "alice", foreign).Status)
	require.Zero(t, capable.approved)
	require.Equal(t, "ok", call(b, "alice", q).Status)
	require.Equal(t, 1, capable.approved)
	require.Empty(t, s.snapshot().Approvals)
	require.Equal(t, "invalid_request", call(b, "alice", q).Status)
	require.Equal(t, 1, capable.approved)
	event.Params = json.RawMessage(`{"threadId":"foreign","turnId":"turn","command":"ls"}`)
	require.False(t, s.queueApproval(event))
	event.Params = json.RawMessage(`{"threadId":"thread-owned","turnId":"old-turn","command":"ls"}`)
	require.False(t, s.queueApproval(event))
	mode := proto.EdgeAgentRequest{Action: "permissions", AccessID: req.AccessID, SessionID: r.SessionID, PermissionMode: "workspace-write"}
	require.Equal(t, "busy", call(b, "alice", mode).Status)
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	require.Equal(t, "workspace-write", call(b, "alice", mode).PermissionMode)
	require.Equal(t, "workspace-write", capable.mode)
}

func TestManagedLongTurnSurvivesReaper(t *testing.T) {
	b, _, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	r := call(b, "alice", req)
	call(b, "alice", proto.EdgeAgentRequest{Action: "send", AccessID: req.AccessID, SessionID: r.SessionID, Text: "long task"})
	s := b.sessions[r.SessionID]
	s.mu.Lock()
	s.turnStarted = time.Now().Add(-10 * time.Minute)
	s.created = time.Now().Add(-48 * time.Hour)
	s.runtimeStarted = s.created
	s.touched = s.created
	s.updated = s.created
	s.mu.Unlock()
	// Exercise the actual one-second reaper, not just the lease predicate.
	require.Never(t, func() bool { return s.snapshot().Closed }, 1500*time.Millisecond, 20*time.Millisecond)
	s.mu.Lock()
	s.running = false
	expired, _ := s.expirationLocked(time.Now())
	s.mu.Unlock()
	require.True(t, expired, "idle process cleanup remains enabled")
}

func TestApprovalRequestValidation(t *testing.T) {
	q := proto.EdgeAgentRequest{Action: "approve", SessionID: strings.Repeat("a", 32), ApprovalID: strings.Repeat("b", 32), Decision: "accept"}
	require.True(t, q.Valid())
	q.Decision = "acceptForSession"
	require.False(t, q.Valid())
	q.Decision = "accept"
	q.Action = "poll"
	require.False(t, q.Valid())
	q = proto.EdgeAgentRequest{Action: "permissions", SessionID: strings.Repeat("a", 32), PermissionMode: "danger-full-access"}
	require.False(t, q.Valid())
}
