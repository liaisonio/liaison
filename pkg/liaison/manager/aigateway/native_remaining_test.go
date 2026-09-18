package aigateway

import (
	"bytes"
	"errors"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestResponsesStatelessScope(t *testing.T) {
	scope := map[string]string{"public": "private"}
	p, err := PrepareResponses([]byte(`{"model":"public","input":"hello","stream":true}`), scope)
	require.NoError(t, err)
	require.True(t, p.Stream)
	require.Contains(t, string(p.Body), `"store":false`)
	require.Contains(t, string(p.Body), `"model":"private"`)
	for _, field := range []string{`"store":true`, `"previous_response_id":"someone-elses-response"`, `"conversation":"foreign"`, `"background":true`, `"tools":[{"type":"web_search"}]`, `"input":[{"type":"item_reference","id":"foreign"}]`} {
		_, err = PrepareResponses([]byte(`{"model":"public","input":"hello",`+field+`}`), scope)
		require.Error(t, err, field)
	}
	_, err = PrepareResponses([]byte(`{"model":"denied","input":"hello"}`), scope)
	require.ErrorIs(t, err, ErrModelDenied)
}

func TestResponsesStreamingAccounting(t *testing.T) {
	start := `data: {"type":"response.created","response":{"id":"r1","object":"response","model":"private","status":"in_progress","error":null}}` + "\n\n"
	delta := `data: {"type":"response.output_text.delta","delta":"hello"}` + "\n\n"
	end := `data: {"type":"response.completed","response":{"id":"r1","object":"response","model":"private","status":"completed","error":null,"output":[],"usage":{"input_tokens":10,"output_tokens":7,"output_tokens_details":{"reasoning_tokens":3}}}}` + "\n\n"
	var out bytes.Buffer
	u := Usage{}
	require.NoError(t, RelayResponsesSSE(strings.NewReader(start+delta+end), "public", func(b []byte) error { _, e := out.Write(b); return e }, &u))
	require.True(t, u.Complete)
	require.EqualValues(t, 10, *u.Input)
	require.EqualValues(t, 7, *u.Output)
	require.NotContains(t, out.String(), "private")
	require.Contains(t, out.String(), "response.output_text.delta")
	for _, stream := range []string{start + delta, delta + end, start + strings.TrimSpace(end), start + `data: {"type":"error","message":"secret"}` + "\n\n"} {
		u = Usage{}
		require.Error(t, RelayResponsesSSE(strings.NewReader(stream), "public", func([]byte) error { return nil }, &u))
		require.False(t, u.Complete)
	}
	u = Usage{}
	require.Error(t, RelayResponsesSSE(strings.NewReader(start+end), "public", func([]byte) error { return errors.New("disconnected") }, &u))
	require.False(t, u.Complete)
	u = Usage{}
	_, err := RewriteResponsesJSON([]byte(`{"id":"r1","object":"response","status":"completed","output":[],"usage":{"input_tokens":-1,"output_tokens":0}}`), "public", &u)
	require.Error(t, err)
	u = Usage{}
	_, err = RewriteResponsesJSON([]byte(`{"id":"r1","object":"response","status":"completed","output":[]}`), "public", &u)
	require.NoError(t, err)
	require.Nil(t, u.Input)
	require.Nil(t, u.Output)
}

func TestQwenNativeAndPlayground(t *testing.T) {
	scope := map[string]string{"public": "private"}
	p, err := PrepareQwen([]byte(`{"model":"public","input":{"messages":[{"role":"user","content":"hi"}]},"parameters":{"result_format":"message","incremental_output":true}}`), scope, true)
	require.NoError(t, err)
	require.True(t, p.Stream)
	require.Contains(t, string(p.Body), `"model":"private"`)
	_, err = PrepareQwen([]byte(`{"model":"public","input":{"messages":[]}}`), scope, true)
	require.Error(t, err)
	end := `data: {"output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}]},"usage":{"input_tokens":3,"output_tokens":2}}` + "\n\n"
	u := Usage{}
	var out bytes.Buffer
	require.NoError(t, RelayQwenSSE(strings.NewReader(end), func(b []byte) error { _, e := out.Write(b); return e }, &u))
	require.True(t, u.Complete)
	require.EqualValues(t, 3, *u.Input)
	u = Usage{}
	require.Error(t, RelayQwenSSE(strings.NewReader(strings.TrimSpace(end)), func([]byte) error { return nil }, &u))
	require.False(t, u.Complete)
	for _, protocol := range []string{"qwen", "gemini"} {
		p, err = PreparePlayground([]byte(`{"model":"public","messages":[{"role":"user","content":"hello"}],"max_tokens":1024,"stream":true}`), scope, protocol)
		require.NoError(t, err)
		require.True(t, p.Stream)
		require.Contains(t, string(p.Body), "hello")
	}
	u = Usage{}
	out.Reset()
	require.NoError(t, RelayPlayground(strings.NewReader(end), "qwen", "public", func(b []byte) error { _, e := out.Write(b); return e }, &u))
	require.Contains(t, out.String(), `"content":"hello"`)
	require.Contains(t, out.String(), "[DONE]")
}
