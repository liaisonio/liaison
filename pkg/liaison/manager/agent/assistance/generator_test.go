package assistance

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/stretchr/testify/require"
)

type providerFunc func(context.Context, runtime.ModelRequest, runtime.ModelEventSink) (runtime.ModelResponse, error)

func TestModelGenerator_ShellModeSharesSessionIdentityAndMemoryWithoutTools(t *testing.T) {
	g, err := NewModelGenerator(providerFunc(func(_ context.Context, r runtime.ModelRequest, _ runtime.ModelEventSink) (runtime.ModelResponse, error) {
		require.Equal(t, "shell-1", r.SessionID)
		require.Empty(t, r.TurnID)
		require.Empty(t, r.Tools)
		require.Equal(t, "shared shell identity", r.Messages[0].Content)
		require.Contains(t, r.Messages[1].Content, "/srv/project")
		require.Contains(t, r.Messages[2].Content, "Current mode: inline")
		require.Contains(t, r.Messages[2].Content, "never numbered prose")
		require.Contains(t, r.Messages[2].Content, "Preserve legitimate names")
		return runtime.ModelResponse{Text: `{"insertion":" /srv/project"}`}, nil
	}))
	require.NoError(t, err)
	_, err = g.Suggest(context.Background(), Binding{Protocol: "ssh"}, Input{AgentSessionID: "shell-1", Text: "cd", Cursor: 2, AgentContext: []runtime.ModelMessage{{Role: runtime.RoleSystem, Content: "shared shell identity"}, {Role: runtime.RoleUser, Content: "project located at /srv/project"}}})
	require.NoError(t, err)
}

func TestModelGenerator_ShellContextRemainsUntrustedData(t *testing.T) {
	g, err := NewModelGenerator(providerFunc(func(_ context.Context, r runtime.ModelRequest, _ runtime.ModelEventSink) (runtime.ModelResponse, error) {
		require.Empty(t, r.Tools)
		require.Contains(t, r.Messages[0].Content, "untrusted data")
		var payload map[string]string
		require.NoError(t, json.Unmarshal([]byte(r.Messages[1].Content), &payload))
		require.Equal(t, `{"directory":"/tmp/demo"}`, payload["shell_context"])
		return runtime.ModelResponse{Text: `{"insertion":"file"}`}, nil
	}))
	require.NoError(t, err)
	_, err = g.Suggest(context.Background(), Binding{Protocol: "ssh"}, Input{Text: "cat ", Cursor: 4, ShellContext: `{"directory":"/tmp/demo"}`})
	require.NoError(t, err)
}

func (f providerFunc) Generate(ctx context.Context, r runtime.ModelRequest, emit runtime.ModelEventSink) (runtime.ModelResponse, error) {
	return f(ctx, r, emit)
}

func TestModelGenerator_OnlySendsEditingContext(t *testing.T) {
	g, err := NewModelGenerator(providerFunc(func(_ context.Context, r runtime.ModelRequest, emit runtime.ModelEventSink) (runtime.ModelResponse, error) {
		require.Empty(t, r.SessionID)
		require.Empty(t, r.TurnID)
		require.Empty(t, r.Tools)
		require.Nil(t, emit)
		require.Len(t, r.Messages, 2)
		var payload map[string]string
		require.NoError(t, json.Unmarshal([]byte(r.Messages[1].Content), &payload))
		require.Equal(t, map[string]string{"protocol": "mysql", "prefix": "SELECT ", "suffix": " FROM users"}, payload)
		return runtime.ModelResponse{Text: `{"insertion":"id"}`}, nil
	}))
	require.NoError(t, err)
	text, err := g.Suggest(context.Background(), Binding{Protocol: "mysql"}, Input{Text: "SELECT  FROM users", Cursor: 7})
	require.NoError(t, err)
	require.Equal(t, "id", text)
}

func TestModelGenerator_RejectsChatAndToolResponses(t *testing.T) {
	for _, response := range []runtime.ModelResponse{
		{Text: "Try SELECT 1"}, {Text: "```json\n{}\n```"}, {Text: `{}`},
		{Text: `{"insertion":null}`}, {Text: `{"insertion":"x","execute":true}`},
		{Text: `{"insertion":"x"} {}`}, {Text: `{"insertion":"x"}`, ToolCalls: []runtime.ModelToolCall{{Name: "ssh.execute"}}},
	} {
		g, err := NewModelGenerator(providerFunc(func(context.Context, runtime.ModelRequest, runtime.ModelEventSink) (runtime.ModelResponse, error) {
			return response, nil
		}))
		require.NoError(t, err)
		_, err = g.Suggest(context.Background(), Binding{Protocol: "ssh"}, Input{})
		require.ErrorIs(t, err, ErrSuggestion)
	}
}

func TestModelGenerator_RetriesMalformedResponseOnce(t *testing.T) {
	for _, recover := range []bool{true, false} {
		calls := 0
		g, err := NewModelGenerator(providerFunc(func(context.Context, runtime.ModelRequest, runtime.ModelEventSink) (runtime.ModelResponse, error) {
			calls++
			if recover && calls == 2 {
				return runtime.ModelResponse{Text: `{"insertion":"1}"}`}, nil
			}
			return runtime.ModelResponse{Text: "not JSON"}, nil
		}))
		require.NoError(t, err)
		text, err := g.Suggest(context.Background(), Binding{Protocol: "mongodb"}, Input{})
		require.Equal(t, 2, calls)
		if recover {
			require.NoError(t, err)
			require.Equal(t, "1}", text)
		} else {
			require.ErrorIs(t, err, ErrSuggestion)
			require.Empty(t, text)
		}
	}
}

func TestModelGenerator_CancelledRequestNeverCallsProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g, err := NewModelGenerator(providerFunc(func(context.Context, runtime.ModelRequest, runtime.ModelEventSink) (runtime.ModelResponse, error) {
		t.Fatal("unexpected call")
		return runtime.ModelResponse{}, nil
	}))
	require.NoError(t, err)
	_, err = g.Suggest(ctx, Binding{Protocol: "ssh"}, Input{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestModelGenerator_CancellationStopsFormatRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	g, err := NewModelGenerator(providerFunc(func(context.Context, runtime.ModelRequest, runtime.ModelEventSink) (runtime.ModelResponse, error) {
		calls++
		cancel()
		return runtime.ModelResponse{Text: "invalid"}, nil
	}))
	require.NoError(t, err)
	_, err = g.Suggest(ctx, Binding{Protocol: "ssh"}, Input{})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}
