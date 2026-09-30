package bridge

import (
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNormalizedUpdatesRespectThreadAndTurn(t *testing.T) {
	s := &session{thread: "owned", turn: "turn", running: true, access: "access", changed: make(chan struct{})}
	u := agentruntime.Update{Kind: agentruntime.MessageDelta, ThreadID: "foreign", TurnID: "turn", ItemID: "m", Text: "no"}
	s.applyUpdateLocked(u)
	require.Empty(t, s.messages)
	u.ThreadID, u.TurnID = "owned", "stale"
	s.applyUpdateLocked(u)
	require.Empty(t, s.messages)
	u.TurnID, u.Text = "turn", "hello"
	s.applyUpdateLocked(u)
	u.Text = " world"
	s.applyUpdateLocked(u)
	require.Len(t, s.messages, 1)
	require.Equal(t, "hello world", s.messages[0].Text)
	u.Kind, u.ItemID, u.Tool, u.Status, u.Command = agentruntime.ActivityUpdate, "tool", "Bash", "running", "printf test"
	s.applyUpdateLocked(u)
	require.Len(t, s.activities, 1)
	require.Equal(t, "commandExecution", s.activities[0].Kind)
	require.Equal(t, "running", s.activities[0].Status)
	u.Status = "completed"
	s.applyUpdateLocked(u)
	require.Equal(t, "completed", s.activities[0].Status)
	u.Kind, u.Status = agentruntime.TurnEnded, "failed"
	s.applyUpdateLocked(u)
	require.False(t, s.running)
	require.Equal(t, "turn_failed", s.status)
	u.Kind, u.Text = agentruntime.MessageDelta, "late"
	s.applyUpdateLocked(u)
	require.Equal(t, "hello world", s.messages[0].Text)
}
