package codex

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	"github.com/stretchr/testify/require"
)

func TestCommandApprovalIsOnceOnly(t *testing.T) {
	client, server := net.Pipe()
	c := rpc.New(client, client)
	defer c.Close()
	defer server.Close()
	received := make(chan json.RawMessage, 1)
	go func() {
		var message json.RawMessage
		if json.NewDecoder(server).Decode(&message) == nil {
			received <- message
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &Session{client: c}
	require.Error(t, s.ApproveCommand(ctx, rpc.Message{ID: json.RawMessage(`7`), Method: "item/fileChange/requestApproval"}))
	require.NoError(t, s.ApproveCommand(ctx, rpc.Message{ID: json.RawMessage(`7`), Method: "item/commandExecution/requestApproval"}))
	select {
	case raw := <-received:
		var response struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &response))
		require.Equal(t, 7, response.ID)
		require.JSONEq(t, `{"decision":"accept"}`, string(response.Result))
	case <-ctx.Done():
		t.Fatal("approval not delivered")
	}
	require.Error(t, s.SetPermissionMode("danger-full-access"))
}

func TestInputAnswerPreservesNativeRequestID(t *testing.T) {
	client, server := net.Pipe()
	c := rpc.New(client, client)
	defer c.Close()
	defer server.Close()
	received := make(chan json.RawMessage, 1)
	go func() {
		var message json.RawMessage
		if json.NewDecoder(server).Decode(&message) == nil {
			received <- message
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &Session{client: c}
	answers := map[string][]string{"layout": {"Compact"}}
	require.Error(t, s.AnswerInput(ctx, rpc.Message{ID: json.RawMessage(`9`), Method: "unknown"}, answers))
	require.NoError(t, s.AnswerInput(ctx, rpc.Message{ID: json.RawMessage(`"request-9"`), Method: "item/tool/requestUserInput"}, answers))
	select {
	case raw := <-received:
		var response struct {
			ID     string          `json:"id"`
			Result json.RawMessage `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &response))
		require.Equal(t, "request-9", response.ID)
		require.JSONEq(t, `{"answers":{"layout":{"answers":["Compact"]}}}`, string(response.Result))
	case <-ctx.Done():
		t.Fatal("input answer not delivered")
	}
}
