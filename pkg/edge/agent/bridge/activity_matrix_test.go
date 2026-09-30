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

func TestActivityKindsAndTerminalStates(t *testing.T) {
	kinds := []string{"commandExecution", "fileChange", "webSearch", "mcpToolCall", "dynamicToolCall", "reasoning", "contextCompaction", "userInput", "turnDiff", "turnPlan", "plan", "imageView", "enteredReviewMode", "exitedReviewMode"}
	for _, kind := range kinds {
		for _, state := range []struct{ native, want string }{{"completed", "completed"}, {"failed", "failed"}, {"declined", "declined"}, {"interrupted", "ended"}, {"cancelled", "ended"}} {
			t.Run(kind+"/"+state.native, func(t *testing.T) {
				s := &session{access: strings.Repeat("a", 32)}
				s.recordActivityLocked("item", kind, "", 0, false)
				require.Equal(t, "running", s.activities[0].Status)
				s.recordActivityLocked("item", kind, state.native, -10, true)
				require.Equal(t, state.want, s.activities[0].Status)
				require.Zero(t, s.activities[0].DurationMS)
				s.recordActivityLocked("item", kind, "", 0, false)
				require.Equal(t, state.want, s.activities[0].Status, "late start never revives a finished item")
			})
		}
	}
	s := &session{}
	s.recordActivityLocked("item", "unknownNativeType", "", 0, false)
	require.Empty(t, s.activities)
}

func TestResolvedApprovalAndInterruptedTurnCannotReplay(t *testing.T) {
	b, a, req := setup(t)
	req.AccessID = strings.Repeat("a", 32)
	r := call(b, "alice", req)
	q := proto.EdgeAgentRequest{Action: "send", AccessID: req.AccessID, SessionID: r.SessionID, Text: "inspect"}
	call(b, "alice", q)
	poll := proto.EdgeAgentRequest{Action: "poll", AccessID: req.AccessID, SessionID: r.SessionID}
	approval := rpc.Message{ID: json.RawMessage(`"pending-1"`), Method: "item/commandExecution/requestApproval", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn-owned","command":"ls"}`)}
	a.events <- approval
	require.Eventually(t, func() bool { return len(call(b, "alice", poll).Approvals) == 1 }, time.Second, time.Millisecond)
	approve := proto.EdgeAgentRequest{Action: "approve", AccessID: req.AccessID, SessionID: r.SessionID, ApprovalID: call(b, "alice", poll).Approvals[0].ID, Decision: "accept"}
	a.events <- rpc.Message{Method: "serverRequest/resolved", Params: json.RawMessage(`{"threadId":"thread-owned","requestId":"pending-1"}`)}
	require.Eventually(t, func() bool { return len(call(b, "alice", poll).Approvals) == 0 }, time.Second, time.Millisecond)
	require.Equal(t, "invalid_request", call(b, "alice", approve).Status)
	approval.ID = json.RawMessage(`"pending-2"`)
	a.events <- approval
	require.Eventually(t, func() bool { return len(call(b, "alice", poll).Approvals) == 1 }, time.Second, time.Millisecond)
	approve.ApprovalID = call(b, "alice", poll).Approvals[0].ID
	a.events <- rpc.Message{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-owned","turn":{"id":"turn-owned","status":"interrupted"}}`)}
	require.Eventually(t, func() bool { return !call(b, "alice", poll).Running }, time.Second, time.Millisecond)
	require.Empty(t, call(b, "alice", poll).Approvals)
	require.Equal(t, "invalid_request", call(b, "alice", approve).Status)
	require.Zero(t, a.approved)
}
