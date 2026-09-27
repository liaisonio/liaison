package codex

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestAttachmentsUseNativeImageInputAndKeepModel(t *testing.T) {
	client, server := net.Pipe()
	c := rpc.New(client, client)
	defer c.Close()
	defer server.Close()
	received := make(chan map[string]json.RawMessage, 1)
	done := make(chan error, 1)
	go func() {
		var request map[string]json.RawMessage
		if err := json.NewDecoder(server).Decode(&request); err != nil {
			done <- err
			return
		}
		received <- request
		done <- json.NewEncoder(server).Encode(map[string]any{"id": request["id"], "result": map[string]any{"turn": map[string]string{"id": "turn-image"}}})
	}()
	s := &Session{client: c, threads: map[string]string{"owned": ""}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	turn, err := s.SendAttachments(ctx, "owned", "Explain the image", "", "native-model", []agentruntime.Attachment{{Path: "/project/image.png", ImagePath: "/edge/private-image"}})
	require.NoError(t, err)
	require.Equal(t, "turn-image", turn)
	require.NoError(t, <-done)
	req := <-received
	require.JSONEq(t, `{"threadId":"owned","input":[{"type":"text","text":"Explain the image"},{"type":"localImage","path":"/edge/private-image"}],"model":"native-model","approvalPolicy":"on-request","sandboxPolicy":{"type":"readOnly"}}`, string(req["params"]))
}
