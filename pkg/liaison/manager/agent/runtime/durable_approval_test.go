package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurableApprovalCoordinator_DecideAndConsumeOnce(t *testing.T) {
	repository := newAgentTestDAO(t)
	store, err := NewDurableStore(repository)
	require.NoError(t, err)
	coordinator, err := NewDurableApprovalCoordinator(repository)
	require.NoError(t, err)
	ctx := context.Background()
	principal := tool.Principal{UserID: 2, OrganizationID: 1}
	require.NoError(t, store.CreateSession(ctx, Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	_, turn, err := store.StartTurn(ctx, "session", Turn{ID: "turn"})
	require.NoError(t, err)
	turn, err = store.TransitionTurn(ctx, turn.ID, turn.Version, TurnRunning, "", "")
	require.NoError(t, err)
	_, step, err := store.AppendStep(ctx, turn.ID, turn.Version, Step{ID: "step", Kind: StepTool, Status: StepRunning})
	require.NoError(t, err)

	toolID := tool.ToolID{Namespace: "ssh", Name: "execute", Version: "1.0.0"}
	snapshot := tool.ToolSetSnapshot{ID: "snapshot", AttachmentGeneration: map[string]uint64{"attachment": 7}}
	invocation := tool.ToolInvocation{ID: "call", Call: tool.ToolCall{ID: toolID, Input: []byte(`{"command":"id"}`)},
		Binding: tool.ToolBinding{Principal: principal, ToolSnapshotID: snapshot.ID,
			Attachment: tool.AttachmentSnapshot{ID: "attachment", Generation: 7}}}
	request := ApprovalRequest{ID: "approval", SessionID: "session", TurnID: "turn", StepID: step.ID,
		RequestedBy: principal.UserID, Invocation: invocation, Snapshot: snapshot, InputSHA256: inputDigest(invocation.Call.Input),
		Risk: tool.RiskHigh, ExpiresAt: time.Now().Add(time.Minute), AttachmentEpoch: 7}
	require.NoError(t, coordinator.Request(ctx, request))
	views, err := coordinator.List(ctx, request.SessionID)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, request.Invocation.Call.ID, views[0].ToolID)
	assert.JSONEq(t, string(request.Invocation.Call.Input), string(views[0].Input))
	assert.Equal(t, ApprovalPending, views[0].Status)
	require.NoError(t, coordinator.Decide(ctx, request.ID, 99, true, "reviewed"))

	consumed, err := coordinator.Consume(ctx, request.ID, principal)
	require.NoError(t, err)
	assert.Equal(t, request.Invocation.ID, consumed.Invocation.ID)
	assert.Equal(t, request.Risk, consumed.Risk)
	_, err = coordinator.Consume(ctx, request.ID, principal)
	assert.ErrorIs(t, err, ErrApprovalConflict)
}

func TestDurableRuntime_ResumesApprovedTurnAfterRuntimeRestart(t *testing.T) {
	repository := newAgentTestDAO(t)
	store, err := NewDurableStore(repository)
	require.NoError(t, err)
	approvals, err := NewDurableApprovalCoordinator(repository)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "session-1", OrganizationID: 10, CreatedBy: 20, Status: SessionActive}))
	attachment, err := store.CreateAttachment(ctx, Attachment{
		ID: "attachment-1", AgentSessionID: "session-1", AccessID: 30, ApplicationID: 40,
		Protocol: tool.ProtocolWebSSH, Generation: 7, State: AttachmentConnected,
		Capabilities: []tool.Capability{"terminal.read", "terminal.execute"},
	})
	require.NoError(t, err)
	session, err := store.GetSession(ctx, "session-1")
	require.NoError(t, err)
	_, err = store.SetActiveAttachment(ctx, session.ID, attachment.ID, session.Version)
	require.NoError(t, err)

	engine := tool.NewEngine(nil, nil)
	descriptor := tool.ToolDescriptor{ID: tool.ToolID{Namespace: "ssh", Name: "read", Version: "1.0.0"},
		DisplayName: "Read SSH", Description: "Read from SSH", InputSchema: json.RawMessage(`{"type":"object"}`),
		OutputKinds: []tool.OutputKind{tool.OutputText}, Protocols: []tool.Protocol{tool.ProtocolWebSSH},
		Disclosure: tool.DisclosureAttachment, Approval: tool.ApprovalAlways,
		Source: tool.ToolSourceRef{ID: "ssh", Kind: "protocol", Trust: tool.TrustBuiltin}}
	require.NoError(t, engine.Register(ctx, tool.ToolRegistration{Descriptor: descriptor, Factory: staticToolFactory{}}))
	provider := &scriptedProvider{responses: []ModelResponse{
		{ToolCalls: []ModelToolCall{{ID: "call-1", Name: descriptor.ID.ModelName(), Input: json.RawMessage(`{ "command": "printf '<ok>' && pwd > /dev/null" }`)}}},
		{Text: "resumed"},
	}}
	ids := &sequentialIDs{}
	firstLoop, err := NewLoop(store, engine, provider, approvals, nil, ids, DefaultLoopConfig())
	require.NoError(t, err)
	paused, err := firstLoop.Run(ctx, loopRunRequest())
	require.NoError(t, err)
	assert.Equal(t, TurnWaitingApproval, paused.Turn.Status)
	require.NoError(t, approvals.Decide(ctx, paused.ApprovalID, 99, true, "approved"))

	reopenedStore, err := NewDurableStore(repository)
	require.NoError(t, err)
	reopenedApprovals, err := NewDurableApprovalCoordinator(repository)
	require.NoError(t, err)
	restartedLoop, err := NewLoop(reopenedStore, engine, provider, reopenedApprovals, nil, ids, DefaultLoopConfig())
	require.NoError(t, err)
	request := loopRunRequest()
	request.Prompt = ""
	resumed, err := restartedLoop.ResumeApproved(ctx, request, paused.ApprovalID)
	require.NoError(t, err)
	assert.Equal(t, TurnCompleted, resumed.Turn.Status)
	assert.Equal(t, "resumed", resumed.Text)
}
