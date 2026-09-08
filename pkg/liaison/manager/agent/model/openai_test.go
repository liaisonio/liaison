package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	agentruntime "github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAICompatibleProvider_StreamsTextAndFragmentedToolCalls(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		var request map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, "test-model", request["model"])
		assert.Equal(t, true, request["stream"])
		stream := "data: {\"choices\":[{\"delta\":{\"content\":\"Checking \"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"function\":{\"name\":\"ssh.execute\",\"arguments\":\"{\\\"command\\\":\"}}]}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"pwd\\\"}\"}}]}}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":4}}\n\n" +
			"data: [DONE]\n\n"
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}, nil
	})}

	provider, err := NewOpenAICompatibleProvider(OpenAIConfig{BaseURL: "https://model.example/v1", APIKey: "secret", Model: "test-model"}, client)
	require.NoError(t, err)
	var deltas []string
	result, err := provider.Generate(context.Background(), agentruntime.ModelRequest{
		Messages: []agentruntime.ModelMessage{{Role: agentruntime.RoleUser, Content: "where am I?"}},
		Tools:    []agentruntime.ModelTool{{Name: "ssh.execute", Description: "execute", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	}, func(_ context.Context, event agentruntime.ModelEvent) error {
		deltas = append(deltas, event.Delta)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "Checking ", result.Text)
	assert.Equal(t, []string{"Checking "}, deltas)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "call-1", result.ToolCalls[0].ID)
	assert.Equal(t, "ssh.execute", result.ToolCalls[0].Name)
	assert.JSONEq(t, `{"command":"pwd"}`, string(result.ToolCalls[0].Input))
	assert.Equal(t, agentruntime.ModelUsage{InputTokens: 12, OutputTokens: 4}, result.Usage)
}

func TestOpenAICompatibleProvider_ReturnsBoundedHTTPError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"error":"bad token"}`))}, nil
	})}
	provider, err := NewOpenAICompatibleProvider(OpenAIConfig{BaseURL: "https://model.example/v1", Model: "test"}, client)
	require.NoError(t, err)
	_, err = provider.Generate(context.Background(), agentruntime.ModelRequest{}, nil)
	assert.ErrorContains(t, err, "HTTP 401")
	assert.ErrorContains(t, err, "bad token")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestNewOpenAICompatibleProvider_ValidatesConfig(t *testing.T) {
	_, err := NewOpenAICompatibleProvider(OpenAIConfig{BaseURL: "file:///tmp/model", Model: "test"}, nil)
	assert.Error(t, err)
	_, err = NewOpenAICompatibleProvider(OpenAIConfig{BaseURL: "https://example.com"}, nil)
	assert.Error(t, err)
}
