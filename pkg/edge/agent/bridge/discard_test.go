package bridge

import (
	"strings"
	"sync"
	"testing"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestDiscardEmptyOwnedSession(t *testing.T) {
	for _, state := range []string{"empty", "messages", "running", "activities"} {
		t.Run(state, func(t *testing.T) {
			b, _, req := setup(t)
			req.AccessID = strings.Repeat("a", 32)
			initial := call(b, "alice", req)
			r := proto.EdgeAgentRequest{Action: "discard", SessionID: initial.SessionID, AccessID: req.AccessID}
			require.True(t, r.Valid())
			require.Equal(t, "not_found", call(b, "bob", r).Status)
			foreign := r
			foreign.AccessID = strings.Repeat("b", 32)
			require.Equal(t, "not_found", call(b, "alice", foreign).Status)
			b.mu.Lock()
			s := b.sessions[initial.SessionID]
			b.mu.Unlock()
			s.mu.Lock()
			switch state {
			case "messages":
				s.messages = []proto.EdgeAgentMessage{{Role: "user", Text: "Keep this"}}
			case "running":
				s.running = true
			case "activities":
				s.activities = []proto.AgentActivity{{ID: "activity"}}
			}
			s.mu.Unlock()
			got := call(b, "alice", r)
			r.Action = "poll"
			if state == "empty" {
				require.Equal(t, "ok", got.Status)
				require.Equal(t, "not_found", call(b, "alice", r).Status)
			} else {
				require.Equal(t, "busy", got.Status)
				require.False(t, call(b, "alice", r).Closed)
			}
		})
	}
}

func TestDiscardRacingFirstMessageNeverDeletesContent(t *testing.T) {
	for i := 0; i < 30; i++ {
		b, _, req := setup(t)
		req.AccessID = strings.Repeat("a", 32)
		initial := call(b, "alice", req)
		var discarded, sent proto.EdgeAgentResult
		var wg sync.WaitGroup
		wg.Add(2)
		gate := make(chan struct{})
		go func() {
			defer wg.Done()
			<-gate
			discarded = call(b, "alice", proto.EdgeAgentRequest{Action: "discard", SessionID: initial.SessionID, AccessID: req.AccessID})
		}()
		go func() {
			defer wg.Done()
			<-gate
			sent = call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: initial.SessionID, AccessID: req.AccessID, Text: "Do not delete"})
		}()
		close(gate)
		wg.Wait()
		if discarded.Status == "ok" {
			require.False(t, sent.Running)
			require.Empty(t, sent.Messages)
		} else {
			require.Equal(t, "busy", discarded.Status)
			require.True(t, sent.Running)
			poll := call(b, "alice", proto.EdgeAgentRequest{Action: "poll", SessionID: initial.SessionID, AccessID: req.AccessID})
			require.Equal(t, "Do not delete", poll.Messages[0].Text)
		}
	}
}
