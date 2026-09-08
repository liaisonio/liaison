package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoop_ExecutesToolAndReturnsToModel(t *testing.T) {
	store, engine := newLoopFixture(t, tool.ApprovalNever)
	provider := &scriptedProvider{responses: []ModelResponse{
		{ToolCalls: []ModelToolCall{{ID: "call-1", Name: "ssh.read", Input: json.RawMessage(`{"command":"pwd"}`)}}},
		{Text: "The working directory is /srv/app."},
	}}
	approvals := &recordingApprovals{}
	var events []EventType
	loop, err := NewLoop(store, engine, provider, approvals, EventSinkFunc(func(_ context.Context, event Event) error {
		events = append(events, event.Type)
		return nil
	}), &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)

	result, err := loop.Run(context.Background(), loopRunRequest())
	require.NoError(t, err)
	assert.Equal(t, TurnCompleted, result.Turn.Status)
	assert.Equal(t, "The working directory is /srv/app.", result.Text)
	require.Len(t, provider.requests, 2)
	require.Len(t, provider.requests[1].Messages, 4)
	assert.Equal(t, RoleSystem, provider.requests[1].Messages[0].Role)
	assert.Equal(t, RoleAssistant, provider.requests[1].Messages[2].Role)
	require.Len(t, provider.requests[1].Messages[2].ToolCalls, 1)
	assert.Equal(t, RoleTool, provider.requests[1].Messages[3].Role)
	assert.Contains(t, provider.requests[1].Messages[3].Content, "/srv/app")
	assert.Equal(t, []EventType{EventTurnStarted, EventToolStarted, EventToolCompleted, EventTurnCompleted}, withoutModelDeltas(events))
	assert.Empty(t, approvals.requests)

	session, err := store.GetSession(context.Background(), "session-1")
	require.NoError(t, err)
	assert.Empty(t, session.ActiveTurnID)
}

func TestLoop_PersistsApprovalAndStopsWorker(t *testing.T) {
	store, engine := newLoopFixture(t, tool.ApprovalAlways)
	provider := &scriptedProvider{responses: []ModelResponse{{
		ToolCalls: []ModelToolCall{{ID: "call-approval", Name: "ssh.read", Input: json.RawMessage(`{"command":"rm -f /tmp/a"}`)}},
	}}}
	approvals := &recordingApprovals{}
	loop, err := NewLoop(store, engine, provider, approvals, nil, &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)

	result, err := loop.Run(context.Background(), loopRunRequest())
	require.NoError(t, err)
	assert.Equal(t, TurnWaitingApproval, result.Turn.Status)
	assert.NotEmpty(t, result.ApprovalID)
	require.Len(t, approvals.requests, 1)
	assert.Equal(t, result.ApprovalID, approvals.requests[0].ID)
	assert.NotEmpty(t, approvals.requests[0].InputSHA256)
	assert.Equal(t, uint64(7), approvals.requests[0].AttachmentEpoch)

	step, err := store.GetStep(context.Background(), approvals.requests[0].StepID)
	require.NoError(t, err)
	assert.Equal(t, StepWaitingApproval, step.Status)
}

func TestLoop_ResumeApprovedExecutesFrozenCallAndContinuesModel(t *testing.T) {
	store, engine := newLoopFixture(t, tool.ApprovalAlways)
	provider := &scriptedProvider{responses: []ModelResponse{
		{ToolCalls: []ModelToolCall{{ID: "call-approval", Name: "ssh.read", Input: json.RawMessage(`{"command":"pwd"}`)}}},
		{Text: "The working directory is /srv/app."},
	}}
	approvals := &resumableApprovals{}
	loop, err := NewLoop(store, engine, provider, approvals, nil, &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)

	request := loopRunRequest()
	request.Selection = ModelSelection{ProviderID: "chosen", Model: "chosen-model"}
	paused, err := loop.Run(context.Background(), request)
	require.NoError(t, err)
	require.NotEmpty(t, paused.ApprovalID)
	approvals.approved = true
	request.Prompt = ""
	request.Selection = ModelSelection{ProviderID: "different", Model: "different-model"}
	resumed, err := loop.ResumeApproved(context.Background(), request, paused.ApprovalID)
	require.NoError(t, err)
	assert.Equal(t, TurnCompleted, resumed.Turn.Status)
	assert.Equal(t, "The working directory is /srv/app.", resumed.Text)
	require.Equal(t, ModelSelection{ProviderID: "chosen", Model: "chosen-model"}, provider.requests[1].Selection)
	require.Len(t, provider.requests, 2)
	require.Len(t, provider.requests[1].Messages, 4)
	assert.Equal(t, RoleSystem, provider.requests[1].Messages[0].Role)
	assert.Equal(t, RoleAssistant, provider.requests[1].Messages[2].Role)
	assert.Equal(t, RoleTool, provider.requests[1].Messages[3].Role)
}

func TestLoop_DeniedApprovalCancelsFrozenStepAndTurn(t *testing.T) {
	store, engine := newLoopFixture(t, tool.ApprovalAlways)
	provider := &scriptedProvider{responses: []ModelResponse{{
		ToolCalls: []ModelToolCall{{ID: "call-approval", Name: "ssh.read", Input: json.RawMessage(`{"command":"pwd"}`)}},
	}}}
	approvals := &resumableApprovals{}
	loop, err := NewLoop(store, engine, provider, approvals, nil, &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)

	request := loopRunRequest()
	paused, err := loop.Run(context.Background(), request)
	require.NoError(t, err)
	request.Prompt = ""
	denied, err := loop.ResolveApproval(context.Background(), request, paused.ApprovalID, false, "not now")
	require.NoError(t, err)
	assert.Equal(t, TurnCancelled, denied.Turn.Status)
	step, err := store.GetStep(context.Background(), approvals.request.StepID)
	require.NoError(t, err)
	assert.Equal(t, StepCancelled, step.Status)
	session, err := store.GetSession(context.Background(), "session-1")
	require.NoError(t, err)
	assert.Empty(t, session.ActiveTurnID)
}

func TestLoop_FailsClosedForModelCallOutsideSnapshot(t *testing.T) {
	store, engine := newLoopFixture(t, tool.ApprovalNever)
	provider := &scriptedProvider{responses: []ModelResponse{{
		ToolCalls: []ModelToolCall{{ID: "call-hidden", Name: "mysql.query", Input: json.RawMessage(`{"sql":"select 1"}`)}},
	}}}
	loop, err := NewLoop(store, engine, provider, &recordingApprovals{}, nil, &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)

	result, err := loop.Run(context.Background(), loopRunRequest())
	assert.ErrorIs(t, err, ErrInvalidModelCall)
	assert.Equal(t, TurnFailed, result.Turn.Status)

	session, getErr := store.GetSession(context.Background(), "session-1")
	require.NoError(t, getErr)
	assert.Empty(t, session.ActiveTurnID)
}

func newLoopFixture(t *testing.T, approval tool.ApprovalMode) (*MemoryStore, *tool.Engine) {
	t.Helper()
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(context.Background(), Session{
		ID: "session-1", OrganizationID: 10, CreatedBy: 20, Status: SessionActive,
	}))
	attachment, err := store.CreateAttachment(context.Background(), Attachment{
		ID: "attachment-1", AgentSessionID: "session-1", AccessID: 30, ApplicationID: 40,
		Protocol: tool.ProtocolWebSSH, Generation: 7, State: AttachmentConnected,
		Capabilities: []tool.Capability{"terminal.read", "terminal.execute"},
	})
	require.NoError(t, err)
	session, err := store.GetSession(context.Background(), "session-1")
	require.NoError(t, err)
	_, err = store.SetActiveAttachment(context.Background(), session.ID, attachment.ID, session.Version)
	require.NoError(t, err)
	engine := tool.NewEngine(nil, nil)
	require.NoError(t, engine.Register(context.Background(), tool.ToolRegistration{
		Descriptor: tool.ToolDescriptor{
			ID:          tool.ToolID{Namespace: "ssh", Name: "read", Version: "1.0.0"},
			DisplayName: "Read SSH terminal", Description: "Read information from the attached SSH session.",
			InputSchema: json.RawMessage(`{"type":"object"}`), OutputKinds: []tool.OutputKind{tool.OutputText},
			Protocols: []tool.Protocol{tool.ProtocolWebSSH}, Disclosure: tool.DisclosureAttachment,
			Approval: approval, Source: tool.ToolSourceRef{ID: "ssh", Kind: "protocol", Trust: tool.TrustBuiltin},
		},
		Factory: staticToolFactory{},
	}))
	return store, engine
}

func loopRunRequest() RunRequest {
	return RunRequest{
		SessionID: "session-1", Prompt: "show the current directory",
		Principal: tool.Principal{UserID: 20, OrganizationID: 10},
	}
}

type staticToolFactory struct{}

func (staticToolFactory) Bind(context.Context, tool.ToolBinding) (tool.ToolExecutor, error) {
	return staticToolExecutor{}, nil
}

type staticToolExecutor struct{}

func (staticToolExecutor) Execute(context.Context, json.RawMessage) (tool.ToolResult, error) {
	return tool.ToolResult{Kind: tool.OutputText, Content: json.RawMessage(`{"stdout":"/srv/app"}`)}, nil
}

type scriptedProvider struct {
	mu        sync.Mutex
	responses []ModelResponse
	requests  []ModelRequest
}

func (provider *scriptedProvider) Generate(ctx context.Context, request ModelRequest, emit ModelEventSink) (ModelResponse, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.requests = append(provider.requests, request)
	if len(provider.responses) == 0 {
		return ModelResponse{}, fmt.Errorf("no scripted response")
	}
	response := provider.responses[0]
	provider.responses = provider.responses[1:]
	if response.Text != "" && emit != nil {
		if err := emit(ctx, ModelEvent{Kind: ModelTextDelta, Delta: response.Text}); err != nil {
			return ModelResponse{}, err
		}
	}
	return response, nil
}

type recordingApprovals struct {
	requests []ApprovalRequest
}

func (coordinator *recordingApprovals) Request(_ context.Context, request ApprovalRequest) error {
	coordinator.requests = append(coordinator.requests, request)
	return nil
}

type resumableApprovals struct {
	request  ApprovalRequest
	approved bool
	decided  bool
	consumed bool
}

func (coordinator *resumableApprovals) Request(_ context.Context, request ApprovalRequest) error {
	coordinator.request = request
	return nil
}

func (coordinator *resumableApprovals) Decide(_ context.Context, _ string, _ uint, approve bool, _ string) error {
	coordinator.approved = approve
	coordinator.decided = true
	return nil
}

func (coordinator *resumableApprovals) Get(_ context.Context, approvalID string) (ApprovalRequest, ApprovalStatus, error) {
	if coordinator.request.ID != approvalID {
		return ApprovalRequest{}, ApprovalPending, ErrApprovalNotFound
	}
	if !coordinator.decided {
		return coordinator.request, ApprovalPending, nil
	}
	if coordinator.approved {
		return coordinator.request, ApprovalApproved, nil
	}
	return coordinator.request, ApprovalDenied, nil
}

func (coordinator *resumableApprovals) List(_ context.Context, sessionID string) ([]ApprovalView, error) {
	if coordinator.request.ID == "" || coordinator.request.SessionID != sessionID {
		return nil, nil
	}
	_, status, err := coordinator.Get(context.Background(), coordinator.request.ID)
	if err != nil {
		return nil, err
	}
	return []ApprovalView{{ID: coordinator.request.ID, TurnID: coordinator.request.TurnID, StepID: coordinator.request.StepID, Status: status}}, nil
}

func (coordinator *resumableApprovals) GetApproved(_ context.Context, approvalID string, _ tool.Principal) (ApprovalRequest, error) {
	if coordinator.request.ID != approvalID || !coordinator.approved || coordinator.consumed {
		return ApprovalRequest{}, ErrApprovalConflict
	}
	return coordinator.request, nil
}

func (coordinator *resumableApprovals) Consume(_ context.Context, approvalID string, _ tool.Principal) (ApprovalRequest, error) {
	if coordinator.request.ID != approvalID || !coordinator.approved || coordinator.consumed {
		return ApprovalRequest{}, ErrApprovalConflict
	}
	coordinator.consumed = true
	return coordinator.request, nil
}

type sequentialIDs struct {
	n int
}

func (ids *sequentialIDs) NewID(prefix string) (string, error) {
	ids.n++
	return fmt.Sprintf("%s-%d", prefix, ids.n), nil
}

func withoutModelDeltas(events []EventType) []EventType {
	filtered := make([]EventType, 0, len(events))
	for _, event := range events {
		if event != EventModelDelta {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
