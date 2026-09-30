package bridge

import (
	"context"
	"testing"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

type interactionAgent struct {
	normalizedAgent
	decisions int
	allowed   bool
	answers   map[string][]string
}

func (a *interactionAgent) Decide(_ context.Context, turn, id string, allow bool) error {
	if turn != "turn" || id != "native" {
		return agentruntime.ErrUnavailable
	}
	a.decisions++
	a.allowed = allow
	return nil
}
func (a *interactionAgent) Answer(_ context.Context, turn, id string, answers map[string][]string) error {
	if turn != "turn" || id != "native" {
		return agentruntime.ErrUnavailable
	}
	a.answers = answers
	return nil
}
func interactionFixture(t *testing.T) (*session, *interactionAgent, agentruntime.Update) {
	t.Helper()
	a := &interactionAgent{normalizedAgent: normalizedAgent{done: make(chan struct{}), updates: make(chan agentruntime.Update)}}
	s := &session{agent: a, access: "access", project: t.TempDir(), thread: "thread", turn: "turn", running: true, changed: make(chan struct{}), status: "ok"}
	u := agentruntime.Update{Kind: agentruntime.InteractionRequested, ThreadID: "thread", TurnID: "turn", Interaction: &agentruntime.Interaction{ID: "native", Tool: "Bash", Command: "printf test"}}
	return s, a, u
}
func TestNormalizedApprovalCardRoundtrip(t *testing.T) {
	for _, decision := range []string{"accept", "decline"} {
		t.Run(decision, func(t *testing.T) {
			s, a, u := interactionFixture(t)
			require.True(t, s.applyInteractionLocked(u))
			view := s.snapshot()
			require.Len(t, view.Approvals, 1)
			require.NotEqual(t, "native", view.Approvals[0].ID)
			require.Equal(t, "printf test", view.Approvals[0].Command)
			q := proto.EdgeAgentRequest{Action: "approve", ApprovalID: view.Approvals[0].ID, Decision: decision}
			require.Equal(t, "ok", s.managePermissions(t.Context(), q).Status)
			require.Equal(t, 1, a.decisions)
			require.Equal(t, decision == "accept", a.allowed)
			require.Equal(t, "invalid_request", s.managePermissions(t.Context(), q).Status)
			require.Equal(t, 1, a.decisions)
		})
	}
}
func TestNormalizedQuestionCardRoundtrip(t *testing.T) {
	s, a, u := interactionFixture(t)
	u.Interaction.Tool = "AskUserQuestion"
	u.Interaction.Questions = []agentruntime.Question{{ID: "q1", Text: "Choose?", Options: []agentruntime.QuestionOption{{Label: "A"}, {Label: "B"}}}}
	require.True(t, s.applyInteractionLocked(u))
	require.Len(t, s.inputs, 1)
	id := s.inputs[0].view.ID
	q := proto.EdgeAgentRequest{Action: "answer", InputID: id, Answers: []proto.AgentInputAnswer{{QuestionID: "q1"}}}
	require.Equal(t, "invalid_request", s.answerInput(t.Context(), q).Status)
	require.Len(t, s.inputs, 1, "invalid input must not consume the question")
	q.Answers[0].Answers = []string{"custom answer"}
	require.Equal(t, "ok", s.answerInput(t.Context(), q).Status)
	require.Equal(t, []string{"custom answer"}, a.answers["q1"])
	require.Equal(t, "invalid_request", s.answerInput(t.Context(), q).Status)
}
func TestNormalizedInteractionScopeAndCancellation(t *testing.T) {
	s, a, u := interactionFixture(t)
	u.ThreadID = "foreign"
	require.False(t, s.applyInteractionLocked(u))
	u.ThreadID, u.TurnID = "thread", "stale"
	require.False(t, s.applyInteractionLocked(u))
	u.TurnID = "turn"
	require.True(t, s.applyInteractionLocked(u))
	id := s.approvals[0].view.ID
	u.Kind = agentruntime.InteractionResolved
	require.True(t, s.applyInteractionLocked(u))
	require.Empty(t, s.approvals)
	require.Equal(t, "invalid_request", s.managePermissions(t.Context(), proto.EdgeAgentRequest{Action: "approve", ApprovalID: id, Decision: "accept"}).Status)
	require.Zero(t, a.decisions)
}
