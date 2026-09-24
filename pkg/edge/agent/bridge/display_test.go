package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestDisplayBoundsDoNotCloseNativeSession(t *testing.T) {
	s := &session{running: true}
	for i := 0; i < 150; i++ {
		s.messages = append(s.messages, proto.EdgeAgentMessage{Role: "assistant", Text: strings.Repeat("中文<", 15000)})
		s.boundDisplayLocked()
	}
	require.True(t, s.running)
	require.False(t, s.closed)
	require.True(t, s.truncated)
	require.LessOrEqual(t, s.bytes, maxOutput)
	for _, message := range s.messages {
		require.True(t, utf8.ValidString(message.Text))
	}
	out := boundedSnapshot(proto.EdgeAgentResult{Messages: append([]proto.EdgeAgentMessage{}, s.messages...)})
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	require.LessOrEqual(t, len(raw), 480<<10)
	many := proto.EdgeAgentResult{}
	for i := 0; i < 96; i++ {
		many.Messages = append(many.Messages, proto.EdgeAgentMessage{Role: "assistant", Text: strings.Repeat("<", 1024)})
	}
	raw, err = json.Marshal(boundedSnapshot(many))
	require.NoError(t, err)
	require.LessOrEqual(t, len(raw), 480<<10)
}

func TestResetDisplayRequiresExactArchiveRevision(t *testing.T) {
	b, _, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	out := call(b, "alice", req)
	s := b.sessions[out.SessionID]
	s.mu.Lock()
	s.messages = []proto.EdgeAgentMessage{{Role: "user", Text: "Original title"}}
	s.changedLocked()
	s.mu.Unlock()
	out = call(b, "alice", proto.EdgeAgentRequest{Action: "poll", AccessID: req.AccessID, SessionID: out.SessionID})
	send := proto.EdgeAgentRPCRequest{Version: 1, ActorID: "alice", ResetRevision: out.Revision + 1, Request: proto.EdgeAgentRequest{Action: "send", AccessID: req.AccessID, SessionID: out.SessionID, Text: "Next round"}}
	require.Equal(t, "busy", b.Handle(context.Background(), send).Status)
	send.ResetRevision = out.Revision
	next := b.Handle(context.Background(), send)
	require.Equal(t, "ok", next.Status)
	require.EqualValues(t, 1, next.Window)
	require.True(t, next.HistoryWindowing)
	require.Equal(t, "Original title", next.Title)
	require.Equal(t, "Next round", next.Messages[0].Text)
	require.Equal(t, out.ThreadID, next.ThreadID)
}
