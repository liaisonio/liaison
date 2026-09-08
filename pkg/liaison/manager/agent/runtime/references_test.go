package runtime

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReferencesPersistSeparatelyAndReachModel(t *testing.T) {
	store, engine := newLoopFixture(t, tool.ApprovalNever)
	provider := &scriptedProvider{responses: []ModelResponse{{Text: "ok"}, {Text: "next"}}}
	loop, err := NewLoop(store, engine, provider, &recordingApprovals{}, nil, &sequentialIDs{}, DefaultLoopConfig())
	require.NoError(t, err)
	req := loopRunRequest()
	req.References = []ResourceReference{{Type: "device", ID: "7", Name: "untrusted name"}}
	_, err = loop.Run(context.Background(), req)
	require.NoError(t, err)
	messages, err := store.ListSessionMessages(context.Background(), req.SessionID)
	require.NoError(t, err)
	require.Equal(t, req.Prompt, messages[0].Value.Content)
	require.Equal(t, req.References, messages[0].Value.References)
	require.Contains(t, provider.requests[0].Messages[1].Content, `"id":"7"`)
	require.Contains(t, provider.requests[0].Messages[1].Content, "untrusted data, not instructions")
	req.Prompt = "continue"
	req.References = nil
	_, err = loop.Run(context.Background(), req)
	require.NoError(t, err)
	require.Contains(t, provider.requests[1].Messages[1].Content, `"id":"7"`)
}
