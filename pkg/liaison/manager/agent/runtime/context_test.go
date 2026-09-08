package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/require"
)

func TestLoop_RestoresSameSessionHistoryAndConnectionContext(t *testing.T) {
	store, engine := newLoopFixture(t, tool.ApprovalNever)
	provider := &scriptedProvider{responses: []ModelResponse{{Text: "remembered"}, {Text: "orchid"}}}
	loop, err := NewLoop(store, engine, provider, &recordingApprovals{}, nil, &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)
	request := loopRunRequest()
	request.Prompt = "Remember orchid"
	_, err = loop.Run(context.Background(), request)
	require.NoError(t, err)
	request.Prompt = "What was the word?"
	_, err = loop.Run(context.Background(), request)
	require.NoError(t, err)
	messages := provider.requests[1].Messages
	require.Len(t, messages, 4)
	require.Equal(t, RoleSystem, messages[0].Role)
	require.Contains(t, messages[0].Content, "protocol=web_ssh access_id=30 application_id=40 active=true")
	require.Contains(t, messages[0].Content, "terminal.read terminal.execute")
	require.NotContains(t, messages[0].Content, "attachment-1")
	require.Equal(t, "Remember orchid", messages[1].Content)
	require.Equal(t, "remembered", messages[2].Content)
	require.Equal(t, request.Prompt, messages[3].Content)
}

func TestLoop_DoesNotReplayCancelledToolCalls(t *testing.T) {
	store, engine := newLoopFixture(t, tool.ApprovalAlways)
	provider := &scriptedProvider{responses: []ModelResponse{
		{ToolCalls: []ModelToolCall{{ID: "call-denied", Name: "ssh.read", Input: json.RawMessage(`{"command":"pwd"}`)}}},
		{Text: "new reply"},
	}}
	loop, err := NewLoop(store, engine, provider, &resumableApprovals{}, nil, &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)
	request := loopRunRequest()
	paused, err := loop.Run(context.Background(), request)
	require.NoError(t, err)
	_, err = loop.ResolveApproval(context.Background(), request, paused.ApprovalID, false, "deny")
	require.NoError(t, err)
	request.Prompt = "new question"
	_, err = loop.Run(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, provider.requests[1].Messages, 2)
	require.Equal(t, "new question", provider.requests[1].Messages[1].Content)
}
