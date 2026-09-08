package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/require"
)

type timeoutFactory struct{}

func (timeoutFactory) Bind(context.Context, tool.ToolBinding) (tool.ToolExecutor, error) {
	return timeoutExecutor{}, nil
}

type timeoutExecutor struct{}

func (timeoutExecutor) Execute(ctx context.Context, _ json.RawMessage) (tool.ToolResult, error) {
	<-ctx.Done()
	return tool.ToolResult{}, ctx.Err()
}

func TestLoop_ApprovedToolTimeoutContinuesConversation(t *testing.T) {
	ctx := context.Background()
	store, engine := newLoopFixture(t, tool.ApprovalAlways)
	attachment, err := store.GetAttachment(ctx, "attachment-1")
	require.NoError(t, err)
	snapshot, err := engine.BuildSnapshot(ctx, tool.DisclosureRequest{Attachments: []tool.AttachmentSnapshot{{ID: attachment.ID, Protocol: attachment.Protocol, Generation: attachment.Generation, Capabilities: attachment.Capabilities}}})
	require.NoError(t, err)
	require.Len(t, snapshot.Tools, 1)
	descriptor := snapshot.Tools[0].Descriptor
	descriptor.DefaultTimeout = time.Millisecond
	require.NoError(t, engine.Replace(ctx, tool.ToolRegistration{Descriptor: descriptor, Factory: timeoutFactory{}}))
	provider := &scriptedProvider{responses: []ModelResponse{
		{ToolCalls: []ModelToolCall{{ID: "slow-call", Name: "ssh.read", Input: json.RawMessage(`{"command":"slow"}`)}}},
		{Text: "The command timed out; narrow the scope before trying again."},
	}}
	approvals := &resumableApprovals{}
	loop, err := NewLoop(store, engine, provider, approvals, nil, &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)
	request := loopRunRequest()
	paused, err := loop.Run(ctx, request)
	require.NoError(t, err)
	approvals.approved = true
	request.Prompt = ""
	result, err := loop.ResumeApproved(ctx, request, paused.ApprovalID)
	require.NoError(t, err)
	require.Equal(t, TurnCompleted, result.Turn.Status)
	require.Len(t, provider.requests, 2)
	messages := provider.requests[1].Messages
	require.Equal(t, RoleTool, messages[len(messages)-1].Role)
	require.Contains(t, messages[len(messages)-1].Content, "tool_timeout")
	require.Contains(t, messages[len(messages)-1].Content, `"IsError":true`)
}
