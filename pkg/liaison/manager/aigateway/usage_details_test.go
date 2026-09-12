package aigateway

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestUsageWithNestedDetails(t *testing.T) {
	var obj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(`{"usage":{"prompt_tokens":21,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":10},"completion_tokens_details":{"reasoning_tokens":2}}}`), &obj))
	var usage Usage
	observeUsage(obj, &usage, false)
	require.NotNil(t, usage.Input)
	require.NotNil(t, usage.Output)
	require.EqualValues(t, 21, *usage.Input)
	require.EqualValues(t, 7, *usage.Output)
}

func TestLimitedStreamRequestsUsage(t *testing.T) {
	for _, raw := range []string{`{}`, `{"stream_options":null}`, `{"stream_options":{"include_usage":false,"include_obfuscation":false}}`} {
		p := Prepared{Body: []byte(raw)}
		require.NoError(t, IncludeStreamUsage(&p))
		var obj map[string]map[string]bool
		require.NoError(t, json.Unmarshal(p.Body, &obj))
		require.True(t, obj["stream_options"]["include_usage"])
	}
	p := Prepared{Body: []byte(`{"stream_options":123}`)}
	require.ErrorIs(t, IncludeStreamUsage(&p), ErrUnsupported)
}
