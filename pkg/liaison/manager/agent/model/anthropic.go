package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"io"
	"net/http"
	"strings"
)

// AnthropicProvider translates the runtime's messages/tools to native Messages API.
// Wire reference: https://platform.claude.com/docs/en/build-with-claude/streaming
type AnthropicProvider struct {
	config OpenAIConfig
	client *http.Client
}

func NewAnthropicProvider(c OpenAIConfig, client *http.Client) (*AnthropicProvider, error) {
	if _, err := NewOpenAICompatibleProvider(c, client); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: c.Timeout}
	}
	return &AnthropicProvider{c, client}, nil
}

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}
type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func (p *AnthropicProvider) Generate(ctx context.Context, request runtime.ModelRequest, emit runtime.ModelEventSink) (runtime.ModelResponse, error) {
	request, restoreNames := encodeToolNames(request)
	body := struct {
		Model     string             `json:"model"`
		MaxTokens int                `json:"max_tokens"`
		Stream    bool               `json:"stream"`
		System    string             `json:"system,omitempty"`
		Messages  []anthropicMessage `json:"messages"`
		Tools     []anthropicTool    `json:"tools,omitempty"`
	}{Model: p.config.Model, MaxTokens: 4096, Stream: true}
	for _, message := range request.Messages {
		if message.Role == runtime.RoleSystem {
			body.System += message.Content + "\n"
			continue
		}
		role := string(message.Role)
		blocks := []anthropicBlock{}
		if message.Role == runtime.RoleTool {
			role = "user"
			blocks = append(blocks, anthropicBlock{Type: "tool_result", ToolUseID: message.ToolCallID, Content: message.Content})
		} else {
			if message.Content != "" {
				blocks = append(blocks, anthropicBlock{Type: "text", Text: message.Content})
			}
			for _, call := range message.ToolCalls {
				blocks = append(blocks, anthropicBlock{Type: "tool_use", ID: call.ID, Name: call.Name, Input: call.Input})
			}
		}
		if len(blocks) == 0 {
			continue
		}
		if len(body.Messages) > 0 && body.Messages[len(body.Messages)-1].Role == role {
			last := len(body.Messages) - 1
			body.Messages[last].Content = append(body.Messages[last].Content, blocks...)
		} else {
			body.Messages = append(body.Messages, anthropicMessage{role, blocks})
		}
	}
	for _, tool := range request.Tools {
		body.Tools = append(body.Tools, anthropicTool{tool.Name, tool.Description, tool.InputSchema})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return runtime.ModelResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.config.BaseURL, "/")+"/messages", bytes.NewReader(raw))
	if err != nil {
		return runtime.ModelResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	response, err := p.client.Do(req)
	if err != nil {
		return runtime.ModelResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return runtime.ModelResponse{}, errors.New("anthropic request rejected")
	}
	result, err := readAnthropicStream(ctx, response.Body, emit)
	return restoreNames(result), err
}
func readAnthropicStream(ctx context.Context, reader io.Reader, emit runtime.ModelEventSink) (runtime.ModelResponse, error) {
	var result runtime.ModelResponse
	scanner := bufio.NewScanner(io.LimitReader(reader, 8<<20))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	calls := map[int]*runtime.ModelToolCall{}
	input := map[int]string{}
	order := []int{}
	completed := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event struct {
			Type         string         `json:"type"`
			Index        int            `json:"index"`
			ContentBlock anthropicBlock `json:"content_block"`
			Delta        struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &event); err != nil {
			return result, errors.New("invalid anthropic stream")
		}
		switch event.Type {
		case "error":
			return result, errors.New("anthropic stream failed")
		case "message_start":
			result.Usage.InputTokens = event.Message.Usage.InputTokens
		case "message_delta":
			result.Usage.OutputTokens = event.Usage.OutputTokens
		case "content_block_start":
			b := event.ContentBlock
			if b.Type == "tool_use" {
				if calls[event.Index] != nil {
					return result, errors.New("duplicate tool block")
				}
				calls[event.Index] = &runtime.ModelToolCall{ID: b.ID, Name: b.Name, Input: b.Input}
				order = append(order, event.Index)
			}
			if b.Type == "text" && b.Text != "" {
				result.Text += b.Text
				if emit != nil {
					if err := emit(ctx, runtime.ModelEvent{Kind: runtime.ModelTextDelta, Delta: b.Text}); err != nil {
						return result, err
					}
				}
			}
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				result.Text += event.Delta.Text
				if emit != nil {
					if err := emit(ctx, runtime.ModelEvent{Kind: runtime.ModelTextDelta, Delta: event.Delta.Text}); err != nil {
						return result, err
					}
				}
			}
			if event.Delta.Type == "input_json_delta" {
				if calls[event.Index] == nil {
					return result, errors.New("missing tool block")
				}
				input[event.Index] += event.Delta.PartialJSON
			}
		case "message_stop":
			completed = true
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if !completed {
		return result, io.ErrUnexpectedEOF
	}
	for _, index := range order {
		call := calls[index]
		if input[index] != "" {
			call.Input = json.RawMessage(input[index])
		}
		if !json.Valid(call.Input) || call.ID == "" || call.Name == "" {
			return result, errors.New("invalid tool call")
		}
		result.ToolCalls = append(result.ToolCalls, *call)
	}
	return result, nil
}
