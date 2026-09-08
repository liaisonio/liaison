package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryTools_SearchThenDescribeDeferredTool(t *testing.T) {
	engine := NewEngine(nil, nil)
	require.NoError(t, RegisterDiscoveryTools(context.Background(), engine))
	deferred := testDescriptor("ssh", "upload_file", "1.0.0", DisclosureDeferred)
	deferred.Tags = []string{"upload", "file"}
	require.NoError(t, engine.Register(context.Background(), testRegistration(deferred, "ok")))

	request := DisclosureRequest{
		Principal:   Principal{UserID: 1, OrganizationID: 2},
		Attachments: []AttachmentSnapshot{{ID: "ssh", Protocol: ProtocolWebSSH, Capabilities: []Capability{"terminal.execute"}, Generation: 3}},
		Primary:     "ssh",
	}
	snapshot, err := engine.BuildSnapshot(context.Background(), request)
	require.NoError(t, err)
	_, deferredVisible := snapshot.Find(deferred.ID)
	assert.False(t, deferredVisible)

	searchInput := json.RawMessage(`{"query":"upload"}`)
	searchResult, err := engine.Execute(context.Background(), snapshot, ToolInvocation{
		ID:   "search-call",
		Call: ToolCall{ID: ToolSearchID, Input: searchInput},
		Binding: ToolBinding{
			Principal:      request.Principal,
			Attachment:     request.Attachments[0],
			ToolSnapshotID: snapshot.ID,
		},
	})
	require.NoError(t, err)
	assert.Contains(t, string(searchResult.Content), deferred.ID.Name)
	assert.NotContains(t, string(searchResult.Content), "input_schema")
	var summaries []struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(searchResult.Content, &summaries))
	require.Len(t, summaries, 1)
	require.Equal(t, deferred.ID.String(), summaries[0].ID)

	describeInput, err := json.Marshal(map[string][]string{"tool_ids": {summaries[0].ID}})
	require.NoError(t, err)
	describeResult, err := engine.Execute(context.Background(), snapshot, ToolInvocation{
		ID:   "describe-call",
		Call: ToolCall{ID: ToolDescribeID, Input: describeInput},
		Binding: ToolBinding{
			Principal:      request.Principal,
			Attachment:     request.Attachments[0],
			ToolSnapshotID: snapshot.ID,
		},
	})
	require.NoError(t, err)
	assert.Contains(t, string(describeResult.Content), "input_schema")
	assert.Contains(t, string(describeResult.Content), deferred.ID.Name)

	request.Promoted = []ToolID{deferred.ID}
	promoted, err := engine.BuildSnapshot(context.Background(), request)
	require.NoError(t, err)
	_, deferredVisible = promoted.Find(deferred.ID)
	assert.True(t, deferredVisible)
}

func TestDiscoveryTools_ListAllDoesNotExposeOtherProtocols(t *testing.T) {
	engine := NewEngine(nil, nil)
	ssh := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	mysql := testDescriptor("mysql", "query", "1.0.0", DisclosureAttachment)
	ctx := context.Background()
	require.NoError(t, engine.Register(ctx, testRegistration(ssh, "ok")))
	require.NoError(t, engine.Register(ctx, testRegistration(mysql, "ok")))
	request := DisclosureRequest{Principal: Principal{UserID: 1, OrganizationID: 2}, Attachments: []AttachmentSnapshot{{ID: "ssh", Protocol: ProtocolWebSSH, Capabilities: []Capability{"terminal.execute"}, Generation: 1}}}
	results, err := engine.Search(ctx, request, "list all available tools")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, ssh.ID, results[0].ID)
}

func TestDiscoveryTools_MalformedIDReturnsCorrectableResult(t *testing.T) {
	executor := discoveryExecutor{engine: NewEngine(nil, nil), kind: discoveryDescribe}
	result, err := executor.Execute(context.Background(), json.RawMessage(`{"tool_ids":["{\"namespace\":\"connector\",\"name\":\"list\",\"version\":\"1.0.0\"}"]}`))
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, string(result.Content), "namespace.name@version")
}
