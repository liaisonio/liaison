package codex

import (
	"context"
	"encoding/json"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

func TestCatalogAndPerTurnModelDoNotWriteConfiguration(t *testing.T) {
	client, server := net.Pipe()
	c := rpc.New(client, client)
	defer c.Close()
	defer server.Close()
	received := make(chan map[string]json.RawMessage, 2)
	errors := make(chan error, 1)
	go func() {
		defer func() {
			if recover() != nil {
				errors <- context.Canceled
			}
		}()
		decoder, encoder := json.NewDecoder(server), json.NewEncoder(server)
		for i := 0; i < 2; i++ {
			var request map[string]json.RawMessage
			if err := decoder.Decode(&request); err != nil {
				errors <- err
				return
			}
			received <- request
			result := json.RawMessage(`{"data":[{"model":"native-model","displayName":"Native model"},{"model":"hidden","hidden":true}],"nextCursor":null}`)
			if i == 1 {
				result = json.RawMessage(`{"turn":{"id":"turn-1"}}`)
			}
			if err := encoder.Encode(map[string]any{"id": request["id"], "result": result}); err != nil {
				errors <- err
				return
			}
		}
		errors <- nil
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s := &Session{client: c, threads: map[string]string{"owned": ""}}
	models, err := s.Models(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)
	turn, err := s.SendModel(ctx, "owned", "hello", "", "native-model")
	require.NoError(t, err)
	require.Equal(t, "turn-1", turn)
	require.NoError(t, <-errors)
	require.JSONEq(t, `"model/list"`, string((<-received)["method"]))
	request := <-received
	require.JSONEq(t, `"turn/start"`, string(request["method"]))
	require.JSONEq(t, `{"threadId":"owned","model":"native-model","input":[{"type":"text","text":"hello"}],"approvalPolicy":"on-request","sandboxPolicy":{"type":"readOnly"}}`, string(request["params"]))
}
