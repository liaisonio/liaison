package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

const discoverySourceID = "liaison-core"

var (
	ToolSearchID   = ToolID{Namespace: "core", Name: "tool_search", Version: "1.0.0"}
	ToolDescribeID = ToolID{Namespace: "core", Name: "tool_describe", Version: "1.0.0"}
)

func RegisterDiscoveryTools(ctx context.Context, engine *Engine) error {
	if engine == nil {
		return fmt.Errorf("register discovery tools: engine is required")
	}
	for _, registration := range discoveryRegistrations(engine) {
		if err := engine.Register(ctx, registration); err != nil {
			return fmt.Errorf("register discovery tool %s: %w", registration.Descriptor.ID.String(), err)
		}
	}
	return nil
}

func discoveryRegistrations(engine *Engine) []ToolRegistration {
	source := ToolSourceRef{ID: discoverySourceID, Kind: "builtin", Trust: TrustBuiltin}
	return []ToolRegistration{
		{
			Descriptor: ToolDescriptor{
				ID:           ToolSearchID,
				SessionKinds: []SessionKind{SessionAccess, SessionManagement},
				DisplayName:  "Search tools",
				Description:  "Search the tools available for the current user and attachment. Use query=\"\" to list all available tools, or concise keywords to search. Results contain summaries, not executable schemas.",
				WhenToUse:    "Use when the currently disclosed tools do not cover the user's request.",
				InputSchema:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","maxLength":256}},"required":["query"],"additionalProperties":false}`),
				OutputKinds:  []OutputKind{OutputFacts},
				Protocols:    []Protocol{ProtocolAny},
				Risk:         RiskReadOnly,
				Approval:     ApprovalNever,
				Disclosure:   DisclosureAlways,
				Source:       source,
			},
			Factory: discoveryFactory{engine: engine, kind: discoverySearch},
		},
		{
			Descriptor: ToolDescriptor{
				ID:           ToolDescribeID,
				SessionKinds: []SessionKind{SessionAccess, SessionManagement},
				DisplayName:  "Describe tools",
				Description:  "Load the executable JSON schemas for selected tools returned by tool_search.",
				WhenToUse:    "Use after tool_search and before calling a deferred tool.",
				InputSchema:  json.RawMessage(`{"type":"object","properties":{"tool_ids":{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":8}},"required":["tool_ids"],"additionalProperties":false}`),
				OutputKinds:  []OutputKind{OutputFacts},
				Protocols:    []Protocol{ProtocolAny},
				Risk:         RiskReadOnly,
				Approval:     ApprovalNever,
				Disclosure:   DisclosureAlways,
				Source:       source,
			},
			Factory: discoveryFactory{engine: engine, kind: discoveryDescribe},
		},
	}
}

type discoveryKind uint8

const (
	discoverySearch discoveryKind = iota
	discoveryDescribe
)

type discoveryFactory struct {
	engine *Engine
	kind   discoveryKind
}

func (factory discoveryFactory) Bind(_ context.Context, binding ToolBinding) (ToolExecutor, error) {
	return &discoveryExecutor{engine: factory.engine, kind: factory.kind, binding: binding}, nil
}

type discoveryExecutor struct {
	engine  *Engine
	kind    discoveryKind
	binding ToolBinding
}

func (executor *discoveryExecutor) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	request := DisclosureRequest{Principal: executor.binding.Principal, SessionKind: executor.binding.SessionKind}
	if executor.binding.Attachment.ID != "" {
		request.Attachments = []AttachmentSnapshot{executor.binding.Attachment}
		request.Primary = executor.binding.Attachment.ID
	}
	switch executor.kind {
	case discoverySearch:
		var parameters struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(input, &parameters); err != nil {
			return ToolResult{}, fmt.Errorf("decode tool search input: %w", err)
		}
		results, err := executor.engine.Search(ctx, request, parameters.Query)
		if err != nil {
			return ToolResult{}, err
		}
		// Discovery IDs must match tool_describe's string contract. Do not
		// expose the internal structured ToolID to the model here.
		type summary struct {
			ToolSummary
			ID string `json:"id"`
		}
		wire := make([]summary, 0, len(results))
		for _, result := range results {
			wire = append(wire, summary{ToolSummary: result, ID: result.ID.String()})
		}
		content, err := json.Marshal(wire)
		if err != nil {
			return ToolResult{}, fmt.Errorf("encode tool search results: %w", err)
		}
		return ToolResult{Kind: OutputFacts, Content: content}, nil
	case discoveryDescribe:
		var parameters struct {
			ToolIDs []string `json:"tool_ids"`
		}
		if err := json.Unmarshal(input, &parameters); err != nil {
			return ToolResult{}, fmt.Errorf("decode tool describe input: %w", err)
		}
		ids := make([]ToolID, 0, len(parameters.ToolIDs))
		for _, value := range parameters.ToolIDs {
			id, err := ParseToolID(value)
			if err != nil {
				// Let the model correct malformed discovery arguments instead
				// of aborting the whole conversation. No tool is promoted.
				return ToolResult{Kind: OutputFacts, IsError: true, Content: json.RawMessage(`{"error":"invalid_tool_id","hint":"Call tool_search, then copy its id string exactly (namespace.name@version) into tool_ids."}`)}, nil
			}
			ids = append(ids, id)
		}
		descriptors, err := executor.engine.Describe(ctx, request, ids)
		if err != nil {
			return ToolResult{}, err
		}
		content, err := json.Marshal(descriptors)
		if err != nil {
			return ToolResult{}, fmt.Errorf("encode tool descriptions: %w", err)
		}
		return ToolResult{Kind: OutputFacts, Content: content}, nil
	default:
		return ToolResult{}, fmt.Errorf("unknown discovery tool kind %d", executor.kind)
	}
}
