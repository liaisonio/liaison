package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionExecutor_RoutesBoundDataSession(t *testing.T) {
	registry := accesssession.NewRegistry()
	descriptor, unregister, err := registry.Register(accesssession.Handle{
		Descriptor: accesssession.Descriptor{ID: "mysql-1", UserID: 7, AccessID: 11, ApplicationID: 13, Protocol: accesssession.ProtocolMySQL},
		Data:       &recordingDataSession{},
	})
	require.NoError(t, err)
	t.Cleanup(unregister)
	executor, err := NewSessionExecutor(registry)
	require.NoError(t, err)

	result, err := executor.Execute(context.Background(), Request{
		Operation: OperationDataQuery,
		Input:     json.RawMessage(`{"statement":"select 1"}`),
		Invocation: tool.ToolInvocation{Binding: tool.ToolBinding{
			Principal: tool.Principal{UserID: 7},
			Attachment: tool.AttachmentSnapshot{AccessHandleID: descriptor.ID, AccessID: descriptor.AccessID,
				ApplicationID: descriptor.ApplicationID, Protocol: tool.ProtocolMySQL, Generation: descriptor.Generation},
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, tool.OutputTable, result.Kind)
	assert.JSONEq(t, `{"rows":[{"value":1}]}`, string(result.Content))
}

func TestProtocolResult_DatabaseErrorAndTruncation(t *testing.T) {
	for _, tc := range []struct {
		name, payload     string
		failed, truncated bool
	}{
		{"sql_error", `{"error":"syntax error","rows":[]}`, true, false},
		{"mongo_error", `{"error":"unknown command"}`, true, false},
		{"redis_error", `{"error":"WRONGTYPE"}`, true, false},
		{"bounded_result", `{"rows":[],"truncated":true}`, false, true},
		{"success", `{"rows":[{"n":1}]}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := protocolResult(tool.OutputTable, json.RawMessage(tc.payload), nil)
			require.NoError(t, err)
			assert.Equal(t, tc.failed, result.IsError)
			assert.Equal(t, tc.truncated, result.Truncated)
			if tc.failed {
				assert.Equal(t, tool.OutputError, result.Kind)
			}
		})
	}
}

func TestSessionExecutor_AllDataProtocolsAndClosedHandles(t *testing.T) {
	for _, protocol := range []accesssession.Protocol{accesssession.ProtocolMySQL, accesssession.ProtocolMariaDB, accesssession.ProtocolSQLServer, accesssession.ProtocolPostgreSQL, accesssession.ProtocolRedis, accesssession.ProtocolMongoDB} {
		t.Run(string(protocol), func(t *testing.T) {
			registry := accesssession.NewRegistry()
			descriptor, unregister, err := registry.Register(accesssession.Handle{
				Descriptor: accesssession.Descriptor{ID: "connection", UserID: 7, AccessID: 11, ApplicationID: 13, Protocol: protocol}, Data: &recordingDataSession{},
			})
			require.NoError(t, err)
			t.Cleanup(unregister)
			executor, err := NewSessionExecutor(registry)
			require.NoError(t, err)
			request := Request{Operation: OperationDataQuery, Input: json.RawMessage(`{"statement":"test"}`), Invocation: tool.ToolInvocation{Binding: tool.ToolBinding{
				Principal: tool.Principal{UserID: 7}, Attachment: tool.AttachmentSnapshot{AccessHandleID: descriptor.ID, AccessID: 11, ApplicationID: 13, Protocol: tool.Protocol(protocol), Generation: descriptor.Generation},
			}}}
			_, err = executor.Execute(context.Background(), request)
			require.NoError(t, err)
			request.Invocation.Binding.Principal.UserID = 8
			_, err = executor.Execute(context.Background(), request)
			require.ErrorIs(t, err, accesssession.ErrHandleMismatch)
			request.Invocation.Binding.Principal.UserID = 7
			unregister()
			_, err = executor.Execute(context.Background(), request)
			require.Error(t, err)
		})
	}
}

func TestSessionExecutor_RejectsWrongPrincipal(t *testing.T) {
	registry := accesssession.NewRegistry()
	descriptor, unregister, err := registry.Register(accesssession.Handle{
		Descriptor: accesssession.Descriptor{ID: "mysql-1", UserID: 7, AccessID: 11, ApplicationID: 13, Protocol: accesssession.ProtocolMySQL},
		Data:       &recordingDataSession{},
	})
	require.NoError(t, err)
	t.Cleanup(unregister)
	executor, err := NewSessionExecutor(registry)
	require.NoError(t, err)

	_, err = executor.Execute(context.Background(), Request{Operation: OperationDataSchema, Input: json.RawMessage(`{}`),
		Invocation: tool.ToolInvocation{Binding: tool.ToolBinding{Principal: tool.Principal{UserID: 8}, Attachment: tool.AttachmentSnapshot{
			AccessHandleID: descriptor.ID, AccessID: descriptor.AccessID, ApplicationID: descriptor.ApplicationID,
			Protocol: tool.ProtocolMySQL, Generation: descriptor.Generation,
		}}}})
	assert.ErrorIs(t, err, accesssession.ErrHandleMismatch)
}

func TestSessionExecutor_ReportsBoundDesktopSession(t *testing.T) {
	registry := accesssession.NewRegistry()
	descriptor, unregister, err := registry.Register(accesssession.Handle{
		Descriptor: accesssession.Descriptor{ID: "rdp-1", UserID: 7, AccessID: 11, ApplicationID: 13, Protocol: accesssession.ProtocolRDP},
		Desktop:    recordingDesktopSession{},
	})
	require.NoError(t, err)
	t.Cleanup(unregister)
	executor, err := NewSessionExecutor(registry)
	require.NoError(t, err)

	result, err := executor.Execute(context.Background(), Request{Operation: OperationDesktopInfo, Input: json.RawMessage(`{}`),
		Invocation: tool.ToolInvocation{Binding: tool.ToolBinding{Principal: tool.Principal{UserID: 7}, Attachment: tool.AttachmentSnapshot{
			AccessHandleID: descriptor.ID, AccessID: descriptor.AccessID, ApplicationID: descriptor.ApplicationID,
			Protocol: tool.ProtocolRDP, Generation: descriptor.Generation,
		}}}})
	require.NoError(t, err)
	assert.Equal(t, tool.OutputFacts, result.Kind)
	assert.JSONEq(t, `{"connected":true,"width":1280,"height":720}`, string(result.Content))
}

type recordingDataSession struct{}

type recordingDesktopSession struct{}

func (recordingDesktopSession) SessionInfo(context.Context) (json.RawMessage, error) {
	return json.RawMessage(`{"connected":true,"width":1280,"height":720}`), nil
}

func (*recordingDataSession) Schema(context.Context, []string) (json.RawMessage, error) {
	return json.RawMessage(`{"nodes":[]}`), nil
}

func (*recordingDataSession) Query(context.Context, string) (json.RawMessage, error) {
	return json.RawMessage(`{"rows":[{"value":1}]}`), nil
}
