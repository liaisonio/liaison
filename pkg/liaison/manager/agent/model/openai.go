package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	agentruntime "github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
)

const maxErrorBodyBytes = 64 << 10

type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// OpenAICompatibleProvider implements the OpenAI chat-completions streaming
// wire format. The Agent Runtime remains provider-neutral and owns all durable
// conversation state.
type OpenAICompatibleProvider struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

func NewOpenAICompatibleProvider(config OpenAIConfig, client *http.Client) (*OpenAICompatibleProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(config.Model) == "" {
		return nil, errors.New("model base URL and model are required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("model base URL must be an absolute HTTP(S) URL")
	}
	if client == nil {
		timeout := config.Timeout
		if timeout <= 0 {
			timeout = 2 * time.Minute
		}
		client = &http.Client{Timeout: timeout}
	}
	return &OpenAICompatibleProvider{
		endpoint: baseURL + "/chat/completions",
		apiKey:   strings.TrimSpace(config.APIKey),
		model:    strings.TrimSpace(config.Model),
		client:   client,
	}, nil
}

type chatRequest struct {
	Model         string        `json:"model"`
	Messages      []chatMessage `json:"messages"`
	Tools         []chatTool    `json:"tools,omitempty"`
	Stream        bool          `json:"stream"`
	StreamOptions streamOptions `json:"stream_options"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type chatToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type pendingToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

func (provider *OpenAICompatibleProvider) Generate(ctx context.Context, request agentruntime.ModelRequest, emit agentruntime.ModelEventSink) (agentruntime.ModelResponse, error) {
	request, restoreNames := encodeToolNames(request)
	payload, err := json.Marshal(chatRequest{
		Model: provider.model, Messages: encodeMessages(request.Messages), Tools: encodeTools(request.Tools),
		Stream: true, StreamOptions: streamOptions{IncludeUsage: true},
	})
	if err != nil {
		return agentruntime.ModelResponse{}, fmt.Errorf("encode model request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(payload))
	if err != nil {
		return agentruntime.ModelResponse{}, fmt.Errorf("create model request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	if provider.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}
	response, err := provider.client.Do(httpRequest)
	if err != nil {
		return agentruntime.ModelResponse{}, fmt.Errorf("call model: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
		if readErr != nil {
			return agentruntime.ModelResponse{}, fmt.Errorf("model returned HTTP %d (read error: %v)", response.StatusCode, readErr)
		}
		return agentruntime.ModelResponse{}, fmt.Errorf("model returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	result, err := decodeEventStream(ctx, response.Body, emit)
	return restoreNames(result), err
}

func encodeMessages(messages []agentruntime.ModelMessage) []chatMessage {
	result := make([]chatMessage, 0, len(messages))
	for _, message := range messages {
		encoded := chatMessage{Role: string(message.Role), Content: message.Content, ToolCallID: message.ToolCallID}
		for _, call := range message.ToolCalls {
			encoded.ToolCalls = append(encoded.ToolCalls, chatToolCall{ID: call.ID, Type: "function",
				Function: chatToolFunction{Name: call.Name, Arguments: string(call.Input)}})
		}
		result = append(result, encoded)
	}
	return result
}

func encodeTools(tools []agentruntime.ModelTool) []chatTool {
	result := make([]chatTool, 0, len(tools))
	for _, value := range tools {
		result = append(result, chatTool{Type: "function", Function: chatFunction{
			Name: value.Name, Description: value.Description, Parameters: append(json.RawMessage(nil), value.InputSchema...),
		}})
	}
	return result
}

func decodeEventStream(ctx context.Context, reader io.Reader, emit agentruntime.ModelEventSink) (agentruntime.ModelResponse, error) {
	var result agentruntime.ModelResponse
	pending := make(map[int]*pendingToolCall)
	maxIndex := -1
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return agentruntime.ModelResponse{}, err
		}
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return agentruntime.ModelResponse{}, fmt.Errorf("decode model stream event: %w", err)
		}
		if chunk.Usage != nil {
			result.Usage = agentruntime.ModelUsage{InputTokens: chunk.Usage.PromptTokens, OutputTokens: chunk.Usage.CompletionTokens}
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				result.Text += choice.Delta.Content
				if emit != nil {
					if err := emit(ctx, agentruntime.ModelEvent{Kind: agentruntime.ModelTextDelta, Delta: choice.Delta.Content}); err != nil {
						return agentruntime.ModelResponse{}, err
					}
				}
			}
			for _, fragment := range choice.Delta.ToolCalls {
				call := pending[fragment.Index]
				if call == nil {
					call = &pendingToolCall{}
					pending[fragment.Index] = call
				}
				if fragment.ID != "" {
					call.id = fragment.ID
				}
				if fragment.Function.Name != "" {
					call.name += fragment.Function.Name
				}
				call.arguments.WriteString(fragment.Function.Arguments)
				if fragment.Index > maxIndex {
					maxIndex = fragment.Index
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return agentruntime.ModelResponse{}, fmt.Errorf("read model stream: %w", err)
	}
	for index := 0; index <= maxIndex; index++ {
		call := pending[index]
		if call == nil {
			return agentruntime.ModelResponse{}, fmt.Errorf("model stream omitted tool call index %d", index)
		}
		input := json.RawMessage(call.arguments.String())
		if call.id == "" || call.name == "" || !json.Valid(input) {
			return agentruntime.ModelResponse{}, fmt.Errorf("model returned invalid tool call at index %d", index)
		}
		result.ToolCalls = append(result.ToolCalls, agentruntime.ModelToolCall{ID: call.id, Name: call.name, Input: input})
	}
	return result, nil
}
