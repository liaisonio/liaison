package model

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"strings"
	"testing"
)

func TestAnthropicStreamTextToolsAndTruncation(t *testing.T) {
	stream := `data: {"type":"message_start","message":{"usage":{"input_tokens":20}}}
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Checking"}}
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call1","name":"inspect","input":{}}}
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"/tmp\"}"}}
data: {"type":"message_delta","usage":{"output_tokens":12}}
data: {"type":"message_stop"}
`
	var delta string
	result, err := readAnthropicStream(context.Background(), strings.NewReader(stream), func(_ context.Context, e runtime.ModelEvent) error { delta += e.Delta; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Checking" || delta != result.Text || len(result.ToolCalls) != 1 || string(result.ToolCalls[0].Input) != `{"path":"/tmp"}` || result.Usage.InputTokens != 20 {
		t.Fatalf("unexpected response: %+v", result)
	}
	_, err = readAnthropicStream(context.Background(), strings.NewReader(strings.ReplaceAll(stream, `data: {"type":"message_stop"}`, "")), nil)
	if err == nil {
		t.Fatal("truncated stream accepted")
	}
}
