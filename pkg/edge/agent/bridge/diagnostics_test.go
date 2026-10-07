package bridge

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestDiagnosticsBoundedMilestonesAndPrivatePayloads(t *testing.T) {
	var records []turnDiagnosticRecord
	s := &session{id: strings.Repeat("a", 32), thread: "private-native-id", turn: "turn", running: true,
		project: "/private/project", model: "private-model", turnStarted: time.Now(),
		messages: []proto.EdgeAgentMessage{{Role: "user", Text: "private-prompt"}}}
	s.diagnostics = newTurnDiagnostics(s.id, 2, s.turnStarted, func(r turnDiagnosticRecord) { records = append(records, r) })
	s.recordTurnTimingLocked(&s.timing.DispatchMS)
	s.applyUpdateLocked(agentruntime.Update{ThreadID: "wrong", TurnID: "turn", Kind: agentruntime.MessageDelta, Text: "not accepted"})
	require.Len(t, records, 2, "foreign progress is not observed")
	for range 1000 {
		s.applyUpdateLocked(agentruntime.Update{ThreadID: s.thread, TurnID: s.turn, Kind: agentruntime.MessageDelta, Text: "private-reply"})
	}
	s.applyUpdateLocked(agentruntime.Update{ThreadID: s.thread, TurnID: s.turn, Kind: agentruntime.TurnEnded, Status: "completed"})
	s.changedLocked()
	require.Len(t, records, 5, "no per-token logging or repeated terminal events")
	last := records[len(records)-1]
	require.Equal(t, "agent_turn_ended", last.Event)
	require.Equal(t, "ended", last.Outcome)
	require.NotNil(t, last.FirstProgressMS)
	require.NotNil(t, last.FirstReplyMS)
	require.NotNil(t, last.DispatchMS)
	for _, record := range records {
		require.Equal(t, last.TraceID, record.TraceID)
		require.Equal(t, s.id, record.SessionID)
	}
	payload, err := json.Marshal(records)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "private-")
	require.NotContains(t, string(payload), "/private/")
	next := newTurnDiagnostics("untrusted\nidentifier", 3, time.Now(), func(turnDiagnosticRecord) {})
	require.Empty(t, next.record.SessionID)
	require.NotEqual(t, last.TraceID, next.record.TraceID)
}

func TestDiagnosticsOverlappingToolsAndHumanWait(t *testing.T) {
	now := time.Now()
	var records []turnDiagnosticRecord
	d := newTurnDiagnostics(strings.Repeat("b", 32), 0, now, func(r turnDiagnosticRecord) { records = append(records, r) })
	s := &session{diagnostics: d, running: true}
	d.activity("a", "commandExecution", false, now.Add(time.Second))
	d.activity("b", "dynamicToolCall", false, now.Add(2*time.Second))
	s.approvals = []pendingApproval{{}, {}}
	s.observeDiagnosticsLocked(now.Add(2 * time.Second))
	s.approvals = s.approvals[:1]
	s.observeDiagnosticsLocked(now.Add(3 * time.Second))
	d.activity("a", "commandExecution", true, now.Add(4*time.Second))
	d.activity("a", "commandExecution", true, now.Add(4*time.Second)) // duplicate completion
	s.approvals = nil
	s.observeDiagnosticsLocked(now.Add(5 * time.Second))
	d.activity("b", "dynamicToolCall", true, now.Add(6*time.Second))
	s.running = false
	s.observeDiagnosticsLocked(now.Add(7 * time.Second))
	last := records[len(records)-1]
	require.EqualValues(t, 3000, last.HumanWaitMS, "overlapping approvals count once")
	require.EqualValues(t, 5000, last.ToolActiveMS, "overlapping tools count once")
	require.Equal(t, 2, last.ToolCount)
	require.False(t, last.Incomplete)
	require.Len(t, records, 3)
}

func TestDiagnosticsClosedFailedAndBoundedActivityState(t *testing.T) {
	for _, status := range []string{"session_closed", "turn_failed"} {
		t.Run(status, func(t *testing.T) {
			now := time.Now()
			var last turnDiagnosticRecord
			d := newTurnDiagnostics(strings.Repeat("c", 32), 0, now, func(r turnDiagnosticRecord) { last = r })
			for i := range 1000 {
				d.activity(string(rune(i)), "commandExecution", false, now)
			}
			require.Len(t, d.items, 256)
			d.record.InterruptRequested = true
			s := &session{diagnostics: d, closed: true, status: status}
			s.observeDiagnosticsLocked(now.Add(time.Second))
			require.True(t, last.Incomplete)
			require.True(t, last.InterruptRequested)
			require.EqualValues(t, 1000, last.ToolActiveMS)
			if status == "turn_failed" {
				require.Equal(t, "failed", last.Outcome)
			} else {
				require.Equal(t, "closed", last.Outcome)
			}
		})
	}
}
