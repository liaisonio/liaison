package runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
)

type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

type ModelMessage struct {
	References []ResourceReference `json:"references,omitempty"`
	Role       MessageRole         `json:"role"`
	Content    string              `json:"content,omitempty"`
	ToolCalls  []ModelToolCall     `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
	ToolName   string              `json:"tool_name,omitempty"`
}

type ResourceReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type Message struct {
	ID             string       `json:"id"`
	AgentSessionID string       `json:"agent_session_id"`
	TurnID         string       `json:"turn_id"`
	Sequence       uint32       `json:"sequence"`
	Value          ModelMessage `json:"value"`
	CreatedAt      time.Time    `json:"created_at"`
}

type ModelTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type ModelSelection struct {
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
}

type ModelRequest struct {
	Selection      ModelSelection
	SessionID      string
	TurnID         string
	Messages       []ModelMessage
	Tools          []ModelTool
	ToolSnapshotID string
}

type ModelToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type ModelUsage struct {
	InputTokens  int
	OutputTokens int
}

type ModelResponse struct {
	Text      string
	ToolCalls []ModelToolCall
	Usage     ModelUsage
}

type ModelEventKind uint8

const (
	ModelTextDelta ModelEventKind = iota
	ModelReasoningDelta
)

type ModelEvent struct {
	Kind  ModelEventKind `json:"kind"`
	Delta string         `json:"delta"`
}

type ModelEventSink func(context.Context, ModelEvent) error

// ModelProvider performs exactly one model inference. Liaison owns all session
// state; provider-side conversation identifiers are only optional optimizations.
type ModelProvider interface {
	Generate(ctx context.Context, request ModelRequest, emit ModelEventSink) (ModelResponse, error)
}

func modelTools(snapshot tool.ToolSetSnapshot) []ModelTool {
	result := make([]ModelTool, 0, len(snapshot.Tools))
	for _, exposed := range snapshot.Tools {
		result = append(result, ModelTool{
			Name:        exposed.Descriptor.ID.ModelName(),
			Description: exposed.Descriptor.Description,
			InputSchema: append(json.RawMessage(nil), exposed.Descriptor.InputSchema...),
		})
	}
	return result
}
