package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolTools_OnlyDiscloseAndRouteMatchingProtocol(t *testing.T) {
	router := NewRouter()
	backend := &recordingExecutor{protocols: []tool.Protocol{tool.ProtocolMySQL}, operations: []Operation{OperationDataSchema, OperationDataQuery}}
	require.NoError(t, router.Register(backend))
	engine := tool.NewEngine(nil, nil)
	source := NewToolSource(router)
	manager := tool.NewSourceManager(engine)
	require.NoError(t, manager.Load(context.Background(), source))

	attachment := tool.AttachmentSnapshot{ID: "mysql-session", Protocol: tool.ProtocolMySQL,
		Capabilities: []tool.Capability{"data.schema", "data.query"}, Generation: 3}
	snapshot, err := engine.BuildSnapshot(context.Background(), tool.DisclosureRequest{Attachments: []tool.AttachmentSnapshot{attachment}})
	require.NoError(t, err)
	require.Len(t, snapshot.Tools, 2)

	queryID := tool.ToolID{Namespace: "data", Name: "query", Version: "1.0.0"}
	exposed, ok := snapshot.Find(queryID)
	require.True(t, ok)
	result, err := engine.Execute(context.Background(), snapshot, tool.ToolInvocation{ID: "call", RegistrationGeneration: exposed.RegistrationGeneration,
		Call:    tool.ToolCall{ID: queryID, Input: json.RawMessage(`{"statement":"select 1"}`)},
		Binding: tool.ToolBinding{ToolSnapshotID: snapshot.ID, Attachment: attachment}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"rows":[[1]]}`, string(result.Content))
	assert.Equal(t, OperationDataQuery, backend.last.Operation)
}

type recordingExecutor struct {
	protocols  []tool.Protocol
	operations []Operation
	last       Request
}

func (executor *recordingExecutor) Protocols() []tool.Protocol { return executor.protocols }
func (executor *recordingExecutor) Operations() []Operation    { return executor.operations }
func (executor *recordingExecutor) Execute(_ context.Context, request Request) (Result, error) {
	executor.last = request
	return Result{Kind: tool.OutputTable, Content: json.RawMessage(`{"rows":[[1]]}`)}, nil
}
