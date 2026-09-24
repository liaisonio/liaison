package bridge

import (
	"context"
	"encoding/json"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func (f *fakeAgent) Models(context.Context) ([]agentruntime.Model, error) {
	return []agentruntime.Model{{ID: "native-model", Name: "Native model"}}, nil
}
func (f *fakeAgent) SendModel(_ context.Context, _, _, _, model string) (string, error) {
	return "turn-" + model, nil
}

func TestSessionManagementOwnershipAndModelSelection(t *testing.T) {
	b, a, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	initial := call(b, "alice", req)
	require.Empty(t, initial.Title)
	action := func(actor, kind, model, title string) proto.EdgeAgentResult {
		return call(b, actor, proto.EdgeAgentRequest{Action: kind, AccessID: req.AccessID, SessionID: initial.SessionID, Model: model, Title: title})
	}
	for _, kind := range []string{"models", "model", "rename", "delete"} {
		model, title := "", ""
		if kind == "model" {
			model = "native-model"
		}
		if kind == "rename" {
			title = "Title"
		}
		require.Equal(t, "not_found", action("bob", kind, model, title).Status)
		foreign := proto.EdgeAgentRequest{Action: kind, AccessID: strings.Repeat("b", 32), SessionID: initial.SessionID, Model: model, Title: title}
		require.Equal(t, "not_found", call(b, "alice", foreign).Status)
	}
	require.Equal(t, "invalid_request", action("alice", "model", "native-model", "").Status)
	models := action("alice", "models", "", "")
	require.True(t, models.ModelsAvailable)
	require.Len(t, models.Models, 1)
	require.Equal(t, "invalid_request", action("alice", "model", "invented", "").Status)
	require.Equal(t, "native-model", action("alice", "model", "native-model", "").Model)
	a.events <- rpc.Message{Method: "thread/name/updated", Params: json.RawMessage(`{"threadId":"thread-owned","threadName":"Native title"}`)}
	require.Eventually(t, func() bool { return action("alice", "poll", "", "").Title == "Native title" }, time.Second, time.Millisecond)
	require.Equal(t, "Renamed", action("alice", "rename", "", "Renamed").Title)
	a.events <- rpc.Message{Method: "thread/name/updated", Params: json.RawMessage(`{"threadId":"thread-owned","threadName":"Late title"}`)}
	sent := call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: initial.SessionID, AccessID: req.AccessID, Text: "Hello"})
	require.True(t, sent.Running)
	b.mu.Lock()
	s := b.sessions[initial.SessionID]
	b.mu.Unlock()
	s.mu.Lock()
	turn := s.turn
	s.mu.Unlock()
	require.Equal(t, "turn-native-model", turn)
	require.Equal(t, "busy", action("alice", "model", "native-model", "").Status)
	require.Equal(t, "busy", action("alice", "delete", "", "").Status)
	require.True(t, action("alice", "stop", "", "").Closed)
	require.Equal(t, "Renamed", action("alice", "poll", "", "").Title)
	require.Equal(t, "ok", action("alice", "delete", "", "").Status)
	require.Equal(t, "not_found", action("alice", "poll", "", "").Status)
	require.Empty(t, call(b, "alice", proto.EdgeAgentRequest{Action: "sessions", AccessID: req.AccessID}).Sessions)
}
