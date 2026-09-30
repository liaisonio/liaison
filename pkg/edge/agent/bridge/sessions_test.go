package bridge

import (
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestSavedSessionListIsolationAndRetention(t *testing.T) {
	b, _, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	started := call(b, "alice", req)
	require.Equal(t, "ok", started.Status)
	list := proto.EdgeAgentRequest{Action: "sessions", AccessID: req.AccessID}
	require.Empty(t, call(b, "bob", list).Sessions)
	other := list
	other.AccessID = strings.Repeat("b", 32)
	require.Empty(t, call(b, "alice", other).Sessions)
	mine := call(b, "alice", list)
	require.True(t, mine.SessionsAvailable)
	require.Len(t, mine.Sessions, 1)
	require.Equal(t, started.SessionID, mine.Sessions[0].SessionID)
	call(b, "alice", proto.EdgeAgentRequest{Action: "send", AccessID: req.AccessID, SessionID: started.SessionID, Text: "Explain this project"})
	mine = call(b, "alice", list)
	require.True(t, mine.Sessions[0].Running)
	require.Equal(t, "Explain this project", mine.Sessions[0].Title)
	ended := call(b, "alice", proto.EdgeAgentRequest{Action: "stop", AccessID: req.AccessID, SessionID: started.SessionID})
	require.True(t, ended.Closed)
	require.True(t, call(b, "alice", list).Sessions[0].Closed)
	history := call(b, "alice", proto.EdgeAgentRequest{Action: "poll", AccessID: req.AccessID, SessionID: started.SessionID})
	require.Equal(t, "Explain this project", history.Messages[0].Text)
}

func TestManagedSessionExpiration(t *testing.T) {
	now := time.Now()
	s := &session{access: strings.Repeat("a", 32), created: now.Add(-time.Hour), touched: now.Add(-2 * time.Minute), updated: now.Add(-2 * time.Minute)}
	expire, forget := s.expirationLocked(now)
	require.False(t, expire, "Leaving a page must not end a managed session after 90s")
	require.False(t, forget)
	s.touched = now.Add(-31 * time.Minute)
	s.updated = s.touched
	expire, _ = s.expirationLocked(now)
	require.True(t, expire)
	s.running = true
	expire, _ = s.expirationLocked(now)
	require.False(t, expire)
	s.closed = true
	s.closedAt = now
	expire, forget = s.expirationLocked(now)
	require.False(t, expire)
	require.False(t, forget)
	_, forget = s.expirationLocked(now.Add(25 * time.Hour))
	require.True(t, forget)
}

func TestSessionListAttentionAndReplyTokens(t *testing.T) {
	b, _, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	started := call(b, "alice", req)
	b.mu.Lock()
	s := b.sessions[started.SessionID]
	b.mu.Unlock()
	require.NotNil(t, s)
	s.mu.Lock()
	s.running = true
	s.approvals = []pendingApproval{{}}
	s.messages = []proto.EdgeAgentMessage{{Role: "assistant", Text: "reply"}}
	s.mu.Unlock()
	list := proto.EdgeAgentRequest{Action: "sessions", AccessID: req.AccessID}
	first := call(b, "alice", list).Sessions[0]
	require.Equal(t, "approval", first.Attention)
	require.NotEmpty(t, first.ReplyToken)
	s.mu.Lock()
	s.approvals = nil
	s.inputs = []pendingInput{{}}
	s.mu.Unlock()
	next := call(b, "alice", list).Sessions[0]
	require.Equal(t, "input", next.Attention)
	require.Equal(t, first.ReplyToken, next.ReplyToken)
	s.mu.Lock()
	s.inputs = nil
	s.status = "turn_failed"
	s.mu.Unlock()
	require.Equal(t, "failed", call(b, "alice", list).Sessions[0].Attention)
	require.Empty(t, call(b, "bob", list).Sessions)
}

func TestHistorySnapshotDoesNotRenewIdleLease(t *testing.T) {
	b, _, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	started := call(b, "alice", req)
	require.Equal(t, "ok", started.Status)
	b.mu.Lock()
	s := b.sessions[started.SessionID]
	b.mu.Unlock()
	s.mu.Lock()
	before := time.Now().Add(-20 * time.Minute)
	s.touched = before
	s.mu.Unlock()
	snapshot := call(b, "alice", proto.EdgeAgentRequest{Action: "snapshot", AccessID: req.AccessID, SessionID: started.SessionID})
	require.Equal(t, started.SessionID, snapshot.SessionID)
	s.mu.Lock()
	defer s.mu.Unlock()
	require.Equal(t, before, s.touched)
}
