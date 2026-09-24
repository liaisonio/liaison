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

func TestInputAnswerScopeValidationAndSecretHistory(t *testing.T) {
	b, a, start := setup(t)
	start.AccessID = strings.Repeat("a", 32)
	r := call(b, "alice", start)
	call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: r.SessionID, AccessID: start.AccessID, Text: "choose"})
	poll := proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID, AccessID: start.AccessID}
	event := rpc.Message{ID: json.RawMessage(`5`), Method: "item/tool/requestUserInput", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn-owned","itemId":"question","isBlocking":true,"questions":[{"id":"mode","header":"Mode","question":"Which mode?","options":[{"label":"A","description":"First"},{"label":"B","description":"Second"}]},{"id":"secret","header":"Secret","question":"Secret?","isSecret":true}]}`)}
	a.events <- event
	require.Eventually(t, func() bool { return len(call(b, "alice", poll).InputRequests) == 1 }, time.Second, time.Millisecond)
	input := call(b, "alice", poll).InputRequests[0]
	q := proto.EdgeAgentRequest{Action: "answer", SessionID: r.SessionID, AccessID: start.AccessID, InputID: input.ID, Answers: []proto.AgentInputAnswer{{QuestionID: "mode", Answers: []string{"A"}}, {QuestionID: "secret", Answers: []string{"test-sensitive-value"}}}}
	require.True(t, q.Valid())
	require.Equal(t, "not_found", call(b, "bob", q).Status)
	foreign := q
	foreign.AccessID = strings.Repeat("b", 32)
	require.Equal(t, "not_found", call(b, "alice", foreign).Status)
	invalid := q
	invalid.Answers = []proto.AgentInputAnswer{{QuestionID: "mode", Answers: []string{"injected choice"}}, {QuestionID: "secret", Answers: []string{"test-sensitive-value"}}}
	require.Equal(t, "invalid_request", call(b, "alice", invalid).Status)
	require.Nil(t, a.answers)
	answered := call(b, "alice", q)
	require.Equal(t, "ok", answered.Status)
	require.Empty(t, answered.InputRequests)
	require.Equal(t, []string{"A"}, a.answers["mode"])
	require.Equal(t, []string{"test-sensitive-value"}, a.answers["secret"])
	raw, err := json.Marshal(answered)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "test-sensitive-value")
	require.Contains(t, string(raw), "[hidden]")
	require.Equal(t, "invalid_request", call(b, "alice", q).Status)
	q.InputID = ""
	require.False(t, q.Valid())
}

func TestNativeResolutionRemovesInput(t *testing.T) {
	b, a, start := setup(t)
	start.AccessID = strings.Repeat("a", 32)
	r := call(b, "alice", start)
	call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: r.SessionID, AccessID: start.AccessID, Text: "question"})
	poll := proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID, AccessID: start.AccessID}
	a.events <- rpc.Message{ID: json.RawMessage(`"input-1"`), Method: "item/tool/requestUserInput", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn-owned","itemId":"question","questions":[{"id":"q","header":"Name","question":"Name?"}]}`)}
	require.Eventually(t, func() bool { return len(call(b, "alice", poll).InputRequests) == 1 }, time.Second, time.Millisecond)
	a.events <- rpc.Message{Method: "serverRequest/resolved", Params: json.RawMessage(`{"threadId":"thread-owned","requestId":"input-1"}`)}
	require.Eventually(t, func() bool { return len(call(b, "alice", poll).InputRequests) == 0 }, time.Second, time.Millisecond)
	require.True(t, call(b, "alice", poll).Running)
}

func TestActivityDetailsStreamingAndBounds(t *testing.T) {
	b, a, start := setup(t)
	start.AccessID = strings.Repeat("a", 32)
	r := call(b, "alice", start)
	call(b, "alice", proto.EdgeAgentRequest{Action: "send", SessionID: r.SessionID, AccessID: start.AccessID, Text: "work"})
	poll := proto.EdgeAgentRequest{Action: "poll", SessionID: r.SessionID, AccessID: start.AccessID}
	send := func(method, params string) { a.events <- rpc.Message{Method: method, Params: json.RawMessage(params)} }
	send("item/started", `{"threadId":"thread-owned","turnId":"turn-owned","item":{"id":"cmd","type":"commandExecution","command":"pwd","cwd":"/project"}}`)
	send("item/commandExecution/outputDelta", `{"threadId":"foreign","turnId":"turn-owned","itemId":"cmd","delta":"foreign secret"}`)
	send("item/commandExecution/outputDelta", `{"threadId":"thread-owned","turnId":"turn-owned","itemId":"cmd","delta":"partial"}`)
	require.Eventually(t, func() bool {
		v := call(b, "alice", poll)
		return len(v.Activities) == 1 && v.Activities[0].Output == "partial"
	}, time.Second, time.Millisecond)
	send("item/completed", `{"threadId":"thread-owned","turnId":"turn-owned","item":{"id":"cmd","type":"commandExecution","command":"pwd","cwd":"/project","aggregatedOutput":"/project\n","exitCode":0,"status":"completed","durationMs":200000}}`)
	send("item/completed", `{"threadId":"thread-owned","turnId":"turn-owned","item":{"id":"file","type":"fileChange","status":"completed","changes":[{"path":"app.go","kind":{"type":"update"},"diff":"@@ -1 +1 @@\n-old\n+new"}]}}`)
	send("turn/diff/updated", `{"threadId":"thread-owned","turnId":"turn-owned","diff":"@@ -1 +1 @@\n-old\n+new"}`)
	send("turn/completed", `{"threadId":"thread-owned","turn":{"id":"turn-owned","status":"completed"}}`)
	require.Eventually(t, func() bool { return !call(b, "alice", poll).Running }, time.Second, time.Millisecond)
	v := call(b, "alice", poll)
	require.Len(t, v.Activities, 3)
	require.Equal(t, "/project\n", v.Activities[0].Output)
	require.Equal(t, 0, *v.Activities[0].ExitCode)
	require.EqualValues(t, 200000, v.Activities[0].DurationMS)
	require.Equal(t, "app.go", v.Activities[1].Changes[0].Path)
	require.Contains(t, v.Activities[2].Output, "+new")
	s := b.sessions[r.SessionID]
	s.mu.Lock()
	for i := 0; i < 20; i++ {
		id := strings.Repeat("x", i+1)
		s.recordActivityLocked(id, "commandExecution", "", 0, false)
		s.recordActivityOutputLocked(id, strings.Repeat("界", maxActivityText), false)
	}
	total := 0
	for _, activity := range s.activities {
		total += activitySize(activity)
	}
	s.mu.Unlock()
	require.LessOrEqual(t, total, maxActivityDetails)
	require.False(t, s.snapshot().Closed, "details truncation must not stop execution")
}
