package aigateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareOllamaChatPreservesNativeContent(t *testing.T) {
	body := `{"model":"chat","messages":[{"role":"user","content":"describe","images":["aGVsbG8="]}],"think":true,"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],"format":"json","options":{"num_predict":50}}`
	p, err := PrepareOllamaChat([]byte(body), map[string]string{"chat": "private-model"})
	require.NoError(t, err)
	require.True(t, p.Stream)
	require.Equal(t, "chat", p.Operation)
	var before, after map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(body), &before))
	require.NoError(t, json.Unmarshal(p.Body, &after))
	require.JSONEq(t, `"private-model"`, string(after["model"]))
	delete(before, "model")
	delete(after, "model")
	require.Equal(t, before, after)
	p, err = PrepareOllamaChat([]byte(`{"model":"chat","stream":false,"messages":[{"role":"tool","content":"result","tool_name":"lookup"}]}`), map[string]string{"chat": "private-model"})
	require.NoError(t, err)
	require.False(t, p.Stream)
}

func TestPrepareOllamaChatRejectsLifecycleAndInvalidRequests(t *testing.T) {
	for name, body := range map[string]string{
		"null": "null", "empty messages": `{"model":"chat","messages":[]}`,
		"unload":           `{"model":"chat","messages":[{"role":"user","content":"hi"}],"keep_alive":0}`,
		"resource options": `{"model":"chat","messages":[{"role":"user","content":"hi"}],"options":{"num_gpu":20}}`,
		"unbounded output": `{"model":"chat","messages":[{"role":"user","content":"hi"}],"options":{"num_predict":-1}}`,
		"null stream":      `{"model":"chat","stream":null,"messages":[{"role":"user","content":"hi"}]}`,
		"unknown role":     `{"model":"chat","messages":[{"role":"other","content":"hi"}]}`,
		"missing content":  `{"model":"chat","messages":[{"role":"user"}]}`,
		"trailing":         "{} {}", "oversize": strings.Repeat(" ", 1<<20) + "{}",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := PrepareOllamaChat([]byte(body), map[string]string{"chat": "private"})
			require.Error(t, err)
		})
	}
	_, err := PrepareOllamaChat([]byte(`{"model":"other","messages":[{"role":"user","content":"hi"}]}`), map[string]string{"chat": "private"})
	require.ErrorIs(t, err, ErrModelDenied)
}

func TestNativeOllamaRelayPreservesToolsAndAccountsTerminalUsage(t *testing.T) {
	first := `{"model":"private","message":{"role":"assistant","content":"","thinking":"reason","tool_calls":[{"function":{"name":"lookup","arguments":{"x":1}}}]},"done":false}`
	last := `{"model":"private","message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":11,"eval_count":4}`
	var out bytes.Buffer
	u := Usage{}
	err := RelayOllamaChat(strings.NewReader(first+"\n"+last+"\n"), "chat", func(p []byte) error { _, e := out.Write(p); return e }, &u)
	require.NoError(t, err)
	require.True(t, u.Complete)
	require.EqualValues(t, 11, *u.Input)
	require.EqualValues(t, 4, *u.Output)
	require.NotContains(t, out.String(), "private")
	require.NotContains(t, out.String(), "data:")
	require.Contains(t, out.String(), `"thinking":"reason"`)
	require.Contains(t, out.String(), `"tool_calls"`)
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		require.True(t, json.Valid([]byte(line)))
	}
}

func TestNativeOllamaRejectsIncompleteAndMalformedUsage(t *testing.T) {
	for name, body := range map[string]string{
		"EOF":            `{"model":"private","message":{"role":"assistant"},"done":false}`,
		"provider error": `{"error":"private upstream details"}`,
		"negative":       `{"model":"private","message":{"role":"assistant"},"done":true,"eval_count":-1}`,
		"null":           `{"model":"private","message":{"role":"assistant"},"done":true,"eval_count":null}`,
		"overflow":       `{"model":"private","message":{"role":"assistant"},"done":true,"prompt_eval_count":9223372036854775807,"eval_count":1}`,
		"missing done":   `{"model":"private","message":{"role":"assistant"}}`,
		"oversize":       strings.Repeat("x", (1<<20)+1),
	} {
		t.Run(name, func(t *testing.T) {
			u := Usage{}
			require.Error(t, RelayOllamaChat(strings.NewReader(body), "chat", func([]byte) error { return nil }, &u))
			require.False(t, u.Complete)
		})
	}
}

func TestNativeOllamaUnknownUsageAndFailedDelivery(t *testing.T) {
	body := `{"model":"private","message":{"role":"assistant","content":"hi"},"done":true}`
	u := Usage{}
	_, err := RewriteOllamaChatJSON([]byte(body), "chat", &u)
	require.NoError(t, err)
	require.True(t, u.Complete)
	require.Nil(t, u.Input)
	require.Nil(t, u.Output)
	u = Usage{}
	failure := errors.New("client disconnected")
	err = RelayOllamaChat(strings.NewReader(body), "chat", func([]byte) error { return failure }, &u)
	require.ErrorIs(t, err, failure)
	require.False(t, u.Complete)
}
