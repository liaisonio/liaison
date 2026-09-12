package aigateway

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestPrepare_PreservesSameProtocolAndRestrictsConversion(t *testing.T) {
	allowed := map[string]string{"chat": "internal/model"}
	raw := []byte(`{"model":"chat","messages":[{"role":"user","content":"hello"}],"stream":true,"tools":[{"type":"function","function":{"name":"test"}}]}`)
	p, err := Prepare(raw, allowed, "openai-compatible")
	require.NoError(t, err)
	require.Contains(t, string(p.Body), `"model":"internal/model"`)
	require.Contains(t, string(p.Body), `"tools"`)
	require.True(t, p.Stream)
	_, err = Prepare(raw, allowed, "anthropic")
	require.ErrorIs(t, err, ErrUnsupported)
	for _, raw := range []string{
		`{"model":"chat","messages":[]}`, `{"model":"chat","messages":[{"role":"user","content":"hi"}],"stream":null}`,
		`{"model":"chat","messages":[{"role":"user","content":[{"type":"image_url"}]}]}`,
		`{"model":"chat","messages":[{"role":"user","content":"hi"},{"role":"system","content":"later"}]}`,
		`{"model":"chat","messages":[{"role":"assistant","content":"prefill"}]}`,
	} {
		_, err = Prepare([]byte(raw), allowed, "anthropic")
		require.ErrorIs(t, err, ErrUnsupported, raw)
	}
	_, err = Prepare([]byte(`{"model":"internal/model","messages":[{}]}`), allowed, "openai-compatible")
	require.ErrorIs(t, err, ErrModelDenied)
	p, err = Prepare([]byte(`{"model":"chat","messages":[{"role":"system","content":"Be brief"},{"role":"user","content":"hello"}],"max_tokens":200,"stop":"END"}`), allowed, "anthropic")
	require.NoError(t, err)
	require.Equal(t, "messages", p.Operation)
	require.Contains(t, string(p.Body), `"system":"Be brief"`)
	require.Contains(t, string(p.Body), `"stop_sequences":["END"]`)
}

func TestRewriteJSON_MapsModelAndUsage(t *testing.T) {
	for _, protocol := range []string{"openai-compatible", "anthropic"} {
		raw := `{"id":"c1","model":"private","choices":[{"message":{"role":"assistant","content":"hi"}}],"usage":{"prompt_tokens":2,"completion_tokens":3}}`
		if protocol == "anthropic" {
			raw = `{"id":"c1","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":3}}`
		}
		u := Usage{}
		out, err := RewriteJSON([]byte(raw), protocol, "chat", &u)
		require.NoError(t, err)
		require.True(t, u.Complete)
		require.EqualValues(t, 2, *u.Input)
		require.EqualValues(t, 3, *u.Output)
		require.Contains(t, string(out), `"model":"chat"`)
		require.NotContains(t, string(out), "private")
	}
	u := Usage{}
	_, err := RewriteJSON([]byte(`{"error":{"message":"private"}}`), "openai-compatible", "chat", &u)
	require.ErrorIs(t, err, ErrResponse)
}

func TestRelaySSE_AliasToolsBoundariesAndIncomplete(t *testing.T) {
	for _, tc := range []struct {
		name, stream string
		success      bool
	}{
		{"tools", "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"id\":\"t1\"}]}}],\"model\":\"private\"}\r\n\r\ndata: [DONE]\r\n\r\n", true},
		{"truncated", "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n", false},
		{"error", "data: {\"error\":{\"message\":\"private\"}}\n\n", false},
		{"premature done", "data: [DONE]\n\n", false},
		{"too large", "data: " + strings.Repeat("x", 1<<20) + "\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := Usage{}
			var output strings.Builder
			err := RelaySSE(strings.NewReader(tc.stream), "openai-compatible", "chat", func(b []byte) error { _, err := output.Write(b); return err }, &u)
			require.Equal(t, tc.success, u.Complete)
			if tc.success {
				require.NoError(t, err)
				require.Contains(t, output.String(), "tool_calls")
				require.Contains(t, output.String(), `"model":"chat"`)
			} else {
				require.Error(t, err)
				require.NotContains(t, output.String(), "[DONE]")
			}
			require.NotContains(t, output.String(), "private")
		})
	}
}

func TestRelaySSE_AnthropicTextAndUsage(t *testing.T) {
	events := []any{
		map[string]any{"type": "message_start", "message": map[string]any{"id": "m1", "usage": map[string]int{"input_tokens": 4, "output_tokens": 1}}},
		map[string]any{"type": "content_block_start", "content_block": map[string]string{"type": "text", "text": ""}},
		map[string]any{"type": "content_block_delta", "delta": map[string]string{"type": "text_delta", "text": "hello"}},
		map[string]any{"type": "message_delta", "delta": map[string]string{"stop_reason": "end_turn"}, "usage": map[string]int{"output_tokens": 2}},
		map[string]any{"type": "message_stop"},
	}
	var input, output strings.Builder
	for _, event := range events {
		raw, err := json.Marshal(event)
		require.NoError(t, err)
		input.WriteString("data: " + string(raw) + "\n\n")
	}
	u := Usage{}
	err := RelaySSE(strings.NewReader(input.String()), "anthropic", "chat", func(b []byte) error { _, err := output.Write(b); return err }, &u)
	require.NoError(t, err)
	require.True(t, u.Complete)
	require.EqualValues(t, 4, *u.Input)
	require.EqualValues(t, 2, *u.Output)
	require.Contains(t, output.String(), "hello")
	require.Contains(t, output.String(), "[DONE]")
}
