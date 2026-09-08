package tool

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApprovalGrant_PersistenceEscapingPreservesBinding(t *testing.T) {
	now := time.Now()
	invocation := ToolInvocation{ID: "call", Call: ToolCall{ID: ToolSearchID, Input: json.RawMessage(`{ "command": "echo '<ok>' && pwd > /dev/null", "n": 9007199254740993 }`)}}
	snapshot := ToolSetSnapshot{ID: "snapshot"}
	grant := ApprovalGrant{ApprovalID: "approval", InvocationID: invocation.ID, ToolID: invocation.Call.ID, ToolSnapshotID: snapshot.ID, InputSHA256: InputDigest(invocation.Call.Input), ExpiresAt: now.Add(time.Minute)}
	encoded, err := json.Marshal(invocation)
	require.NoError(t, err)
	var restored ToolInvocation
	require.NoError(t, json.Unmarshal(encoded, &restored))
	require.NotEqual(t, string(invocation.Call.Input), string(restored.Call.Input))
	require.True(t, grant.validates(restored, snapshot, now))
	for _, invalid := range []string{`{"command":"different"}`, `{"n":9007199254740992}`, `invalid`, ``} {
		restored.Call.Input = json.RawMessage(invalid)
		require.False(t, grant.validates(restored, snapshot, now))
	}
}

func TestToolSearch_ListingAndSpecificQueries(t *testing.T) {
	descriptor := ToolDescriptor{ID: ToolID{Namespace: "terminal", Name: "read", Version: "1.0.0"}}
	for _, query := range []string{"", "list all available tools", "可用工具", "terminal"} {
		require.Positive(t, descriptorMatchScore(descriptor, query), query)
	}
	require.Zero(t, descriptorMatchScore(descriptor, "database"))
}
