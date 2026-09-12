package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShellSessionDisclosesOnlyTerminalAndDiscovery(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(nil, nil)
	require.NoError(t, RegisterDiscoveryTools(ctx, engine))
	for _, id := range []ToolID{{Namespace: "terminal", Name: "read", Version: "1.0.0"}, {Namespace: "terminal", Name: "execute", Version: "1.0.0"}, {Namespace: "data", Name: "query", Version: "1.0.0"}, {Namespace: "terminal", Name: "future", Version: "1.0.0"}} {
		d := testDescriptor(id.Namespace, id.Name, id.Version, DisclosureAlways)
		d.Protocols = []Protocol{ProtocolAny}
		d.Capabilities = nil
		require.NoError(t, engine.Register(ctx, testRegistration(d, "ok")))
	}
	snapshot, err := engine.BuildSnapshot(ctx, DisclosureRequest{SessionKind: SessionShell})
	require.NoError(t, err)
	require.Len(t, snapshot.Tools, 4)
	results, err := engine.Search(ctx, DisclosureRequest{SessionKind: SessionShell}, "")
	require.NoError(t, err)
	for _, result := range results {
		require.NotEqual(t, "data", result.ID.Namespace)
		require.NotEqual(t, "future", result.ID.Name)
	}
	result, err := engine.Execute(ctx, snapshot, ToolInvocation{ID: "search", Call: ToolCall{ID: ToolSearchID, Input: []byte(`{"query":""}`)}, Binding: ToolBinding{ToolSnapshotID: snapshot.ID, SessionKind: SessionShell}})
	require.NoError(t, err)
	require.NotContains(t, string(result.Content), "data.query")
}

func TestSessionKindIsolatesSearchDescribeSnapshotAndExecution(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(nil, nil)
	legacy := testDescriptor("legacy", "read", "1.0.0", DisclosureAlways)
	legacy.Protocols = []Protocol{ProtocolAny}
	legacy.Capabilities = nil
	management := legacy
	management.ID = ToolID{Namespace: "connector", Name: "list", Version: "1.0.0"}
	management.SessionKinds = []SessionKind{SessionManagement}
	for _, descriptor := range []ToolDescriptor{legacy, management} {
		require.NoError(t, engine.Register(ctx, testRegistration(descriptor, "ok")))
	}
	for _, kind := range []SessionKind{"", SessionAccess, SessionManagement, "invalid"} {
		t.Run(string(kind), func(t *testing.T) {
			request := DisclosureRequest{SessionKind: kind}
			summaries, err := engine.Search(ctx, request, "")
			require.NoError(t, err)
			described, err := engine.Describe(ctx, request, []ToolID{legacy.ID, management.ID})
			require.NoError(t, err)
			snapshot, err := engine.BuildSnapshot(ctx, request)
			require.NoError(t, err)
			if kind == "invalid" {
				require.Empty(t, summaries)
				require.Empty(t, described)
				require.Empty(t, snapshot.Tools)
				return
			}
			want := legacy.ID
			if kind == SessionManagement {
				want = management.ID
			}
			require.Len(t, summaries, 1)
			require.Equal(t, want, summaries[0].ID)
			require.Len(t, described, 1)
			require.Equal(t, want, described[0].ID)
			require.Len(t, snapshot.Tools, 1)
			invocation := ToolInvocation{ID: "call", Call: ToolCall{ID: want, Input: []byte(`{}`)}, Binding: ToolBinding{ToolSnapshotID: snapshot.ID, SessionKind: kind}}
			_, err = engine.Execute(ctx, snapshot, invocation)
			require.NoError(t, err)
			invocation.Binding.SessionKind = SessionManagement
			if kind == SessionManagement {
				invocation.Binding.SessionKind = SessionAccess
			}
			_, err = engine.Execute(ctx, snapshot, invocation)
			require.ErrorIs(t, err, ErrPolicyDenied)
		})
	}
}

func TestManagementDiscoveryRetainsSessionKind(t *testing.T) {
	ctx := context.Background()
	engine := NewEngine(nil, nil)
	require.NoError(t, RegisterDiscoveryTools(ctx, engine))
	d := testDescriptor("connector", "list", "1.0.0", DisclosureDeferred)
	d.Protocols = []Protocol{ProtocolAny}
	d.Capabilities = nil
	d.SessionKinds = []SessionKind{SessionManagement}
	require.NoError(t, engine.Register(ctx, testRegistration(d, "ok")))
	snapshot, err := engine.BuildSnapshot(ctx, DisclosureRequest{SessionKind: SessionManagement})
	require.NoError(t, err)
	for _, call := range []ToolCall{
		{ID: ToolSearchID, Input: []byte(`{"query":""}`)},
		{ID: ToolDescribeID, Input: []byte(`{"tool_ids":["connector.list@1.0.0"]}`)},
	} {
		result, err := engine.Execute(ctx, snapshot, ToolInvocation{ID: "discovery", Call: call, Binding: ToolBinding{ToolSnapshotID: snapshot.ID, SessionKind: SessionManagement}})
		require.NoError(t, err)
		require.Contains(t, string(result.Content), "connector")
	}
}
