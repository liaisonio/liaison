package bridge

import (
	"encoding/json"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestPlanAndToolMetadataScopedToTurn(t *testing.T) {
	b, a, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	started := call(b, "alice", req)
	poll := proto.EdgeAgentRequest{Action: "poll", AccessID: req.AccessID, SessionID: started.SessionID}
	call(b, "alice", proto.EdgeAgentRequest{Action: "send", AccessID: req.AccessID, SessionID: started.SessionID, Text: "Inspect"})
	emit := func(method, params string) { a.events <- rpc.Message{Method: method, Params: json.RawMessage(params)} }
	emit("turn/plan/updated", `{"threadId":"foreign","turnId":"turn-owned","plan":[{"step":"wrong owner","status":"pending"}]}`)
	emit("turn/plan/updated", `{"threadId":"thread-owned","turnId":"foreign","plan":[{"step":"wrong turn","status":"pending"}]}`)
	emit("turn/plan/updated", `{"threadId":"thread-owned","turnId":"turn-owned","explanation":"Checking the project","plan":[{"step":"Inspect","status":"completed"},{"step":"Verify","status":"inProgress"}]}`)
	emit("item/completed", `{"threadId":"thread-owned","turnId":"turn-owned","item":{"id":"search","type":"webSearch","query":"SQLite WAL"}}`)
	emit("item/completed", `{"threadId":"thread-owned","turnId":"turn-owned","item":{"id":"tool","type":"mcpToolCall","server":"docs","tool":"search","arguments":{"secret":"do-not-store"},"result":"do-not-store"}}`)
	emit("item/completed", `{"threadId":"thread-owned","turnId":"turn-owned","item":{"id":"proposed","type":"plan","text":"## Plan\n\nRun tests"}}`)
	require.Eventually(t, func() bool { return len(call(b, "alice", poll).Activities) == 4 }, time.Second, time.Millisecond)
	out := call(b, "alice", poll)
	require.Len(t, out.Activities[0].Plan, 2)
	require.Equal(t, "inProgress", out.Activities[0].Plan[1].Status)
	require.Equal(t, "SQLite WAL", out.Activities[1].Label)
	require.Equal(t, "docs / search", out.Activities[2].Label)
	require.Contains(t, out.Activities[3].Output, "Run tests")
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "do-not-store")
	require.NotContains(t, string(raw), "wrong")
	emit("turn/plan/updated", `{"threadId":"thread-owned","turnId":"turn-owned","plan":[{"step":"Inspect","status":"completed"},{"step":"Verify","status":"completed"}]}`)
	require.Eventually(t, func() bool { return call(b, "alice", poll).Activities[0].Status == "completed" }, time.Second, time.Millisecond)
}

func TestPlanBudgetsAndURLMetadata(t *testing.T) {
	s := &session{access: strings.Repeat("a", 32)}
	steps := make([]proto.AgentPlanStep, 100)
	for i := range steps {
		steps[i] = proto.AgentPlanStep{Step: strings.Repeat("长路径", 1000), Status: "pending"}
	}
	s.recordPlanLocked("turn", "", steps)
	require.True(t, s.activities[0].Truncated)
	require.LessOrEqual(t, len(s.activities[0].Plan), 32)
	require.LessOrEqual(t, activitySize(s.activities[0]), maxActivityText)
	s.recordActivityLocked("url", "webSearch", "", 0, true)
	item := nativeActivity{ID: "url", Type: "webSearch"}
	item.Action.URL = "https://user:password@example.com/docs?key=secret#fragment"
	s.recordActivityDetailsLocked(item)
	require.Equal(t, "https://example.com/docs", s.activities[1].Label)
	preview := &session{}
	preview.recordPlanLocked("turn", "private", steps)
	require.Empty(t, preview.activities)
}
