package bridge

import (
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestTurnTimingIsOptionalStableAndScoped(t *testing.T) {
	s := &session{thread: "owned", turn: "turn", running: true, changed: make(chan struct{})}
	require.Nil(t, s.turnTimingLocked())
	s.turnStarted = time.Now().Add(-100 * time.Millisecond)
	s.recordTurnTimingLocked(&s.timing.DispatchMS)
	require.GreaterOrEqual(t, *s.timing.DispatchMS, int64(100))
	s.applyUpdateLocked(agentruntime.Update{ThreadID: "foreign", TurnID: "turn", Kind: agentruntime.MessageDelta, Text: "ignored"})
	s.applyUpdateLocked(agentruntime.Update{ThreadID: "owned", TurnID: "old", Kind: agentruntime.MessageDelta, Text: "ignored"})
	s.applyUpdateLocked(agentruntime.Update{ThreadID: "owned", TurnID: "turn", Kind: agentruntime.MessageDelta})
	require.Nil(t, s.timing.FirstReplyMS)
	s.applyUpdateLocked(agentruntime.Update{ThreadID: "owned", TurnID: "turn", Kind: agentruntime.MessageDelta, Text: "hello"})
	require.NotNil(t, s.timing.FirstReplyMS)
	first := *s.timing.FirstReplyMS
	s.applyUpdateLocked(agentruntime.Update{ThreadID: "owned", TurnID: "turn", Kind: agentruntime.MessageDelta, Text: " again"})
	require.Equal(t, first, *s.timing.FirstReplyMS)
	s.applyUpdateLocked(agentruntime.Update{ThreadID: "owned", TurnID: "turn", Kind: agentruntime.TurnEnded, Status: "completed"})
	require.GreaterOrEqual(t, *s.timing.FinishedMS, first)
	old := s.turnTimingLocked()
	s.timing = proto.AgentTurnTiming{}
	require.NotNil(t, old.FinishedMS, "new turn must not mutate prior snapshot")
	require.Nil(t, s.turnTimingLocked().FinishedMS)
}

func TestTurnTimingZeroIsNotMissing(t *testing.T) {
	zero := int64(0)
	data, err := json.Marshal(proto.AgentTurnTiming{DispatchMS: &zero})
	require.NoError(t, err)
	require.JSONEq(t, `{"dispatch_ms":0}`, string(data))
}
