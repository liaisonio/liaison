package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
)

const builtinSourceID = "liaison-protocol-executors"

type ToolSource struct {
	router *Router
}

func NewToolSource(router *Router) *ToolSource {
	return &ToolSource{router: router}
}

func (*ToolSource) ID() string                  { return builtinSourceID }
func (*ToolSource) TrustLevel() tool.TrustLevel { return tool.TrustBuiltin }

func (source *ToolSource) Snapshot(context.Context) ([]tool.ToolRegistration, error) {
	if source == nil || source.router == nil {
		return nil, fmt.Errorf("protocol tool source requires router")
	}
	return builtinRegistrations(source.router), nil
}

func (*ToolSource) Watch(context.Context) (<-chan tool.ToolSourceEvent, error) {
	events := make(chan tool.ToolSourceEvent)
	close(events)
	return events, nil
}

type routeFactory struct {
	router    *Router
	operation Operation
}

func (factory routeFactory) Bind(_ context.Context, binding tool.ToolBinding) (tool.ToolExecutor, error) {
	if binding.Attachment.ID == "" {
		return nil, fmt.Errorf("%s requires an attached protocol session", factory.operation)
	}
	return routeExecutor{router: factory.router, operation: factory.operation, binding: binding}, nil
}

type routeExecutor struct {
	router    *Router
	operation Operation
	binding   tool.ToolBinding
}

func (executor routeExecutor) Execute(ctx context.Context, input json.RawMessage) (tool.ToolResult, error) {
	return executor.ExecuteInvocation(ctx, tool.ToolInvocation{Call: tool.ToolCall{Input: input}, Binding: executor.binding})

}

func (executor routeExecutor) ExecuteInvocation(ctx context.Context, invocation tool.ToolInvocation) (tool.ToolResult, error) {
	result, err := executor.router.Execute(ctx, Request{Operation: executor.operation, Input: invocation.Call.Input, Invocation: invocation})
	if err != nil {
		return tool.ToolResult{}, err
	}
	return tool.ToolResult{Kind: result.Kind, IsError: result.IsError, Content: result.Content, Truncated: result.Truncated, ArtifactID: result.ArtifactID}, nil
}

func builtinRegistrations(router *Router) []tool.ToolRegistration {
	source := tool.ToolSourceRef{ID: builtinSourceID, Kind: "builtin", Trust: tool.TrustBuiltin}
	return []tool.ToolRegistration{
		registration(router, source, "terminal", "read", OperationTerminalRead,
			"Read terminal output", "Read recent visible output from the attached SSH terminal.",
			[]tool.Protocol{tool.ProtocolSSH, tool.ProtocolWebSSH}, []tool.Capability{"terminal.read"}, tool.RiskReadOnly, tool.ApprovalNever,
			`{"type":"object","properties":{"max_lines":{"type":"integer","minimum":1,"maximum":500}},"additionalProperties":false}`,
			[]tool.OutputKind{tool.OutputText}),
		registration(router, source, "terminal", "execute", OperationTerminalExecute,
			"Execute terminal command", "Execute a command through the attached SSH connection using a separate exec channel. This does not type into the interactive terminal or inherit its current directory or shell variables. Use explicit paths and report the returned output. User approval may be required.",
			[]tool.Protocol{tool.ProtocolSSH, tool.ProtocolWebSSH}, []tool.Capability{"terminal.execute"}, tool.RiskMedium, tool.ApprovalByPolicy,
			`{"type":"object","properties":{"command":{"type":"string","minLength":1,"maxLength":8192}},"required":["command"],"additionalProperties":false}`,
			[]tool.OutputKind{tool.OutputText, tool.OutputArtifact}),
		registration(router, source, "data", "schema", OperationDataSchema,
			"Inspect data schema", "Inspect the attached database session. Empty path returns current_database and a tree of databases, schemas, tables, collections or keys. To inspect an object use path=[type,database,schema,name]: MySQL [table,db,\"\",table], PostgreSQL [table,db,schema,table], MongoDB [collection,db,\"\",collection], Redis [key,current_db_number,\"\",key]. Object details include columns/indexes/DDL for SQL, index-derived fields/indexes for MongoDB (not a complete document schema), and type/TTL/memory for Redis in its currently selected DB. Use names from the tree, do not invent them. Use an approved bounded query to sample documents or read key values.",
			[]tool.Protocol{tool.ProtocolMySQL, tool.ProtocolPostgreSQL, tool.ProtocolRedis, tool.ProtocolMongoDB}, []tool.Capability{"data.schema"}, tool.RiskReadOnly, tool.ApprovalNever,
			`{"type":"object","properties":{"path":{"type":"array","items":{"type":"string"},"maxItems":4}},"additionalProperties":false}`,
			[]tool.OutputKind{tool.OutputFacts, tool.OutputTable}),
		registration(router, source, "data", "query", OperationDataQuery,
			"Query data", "Run one statement in the attached data session after approval. MySQL/PostgreSQL: SQL, prefer qualified table names and LIMIT. Redis: native command text (GET, SCAN, HGETALL, etc.), prefer SCAN over KEYS. MongoDB: a JSON database command, NOT mongosh JavaScript; e.g. {\"find\":\"items\",\"filter\":{},\"limit\":20}, {\"aggregate\":\"items\",\"pipeline\":[{\"$limit\":20}],\"cursor\":{}}. MongoDB executes in current_database from data.schema; do not claim to switch databases. Writes, updates, deletes and DDL all require approval. Explain failures using the returned error, never claim success for an error result.",
			[]tool.Protocol{tool.ProtocolMySQL, tool.ProtocolPostgreSQL, tool.ProtocolRedis, tool.ProtocolMongoDB}, []tool.Capability{"data.query"}, tool.RiskHigh, tool.ApprovalByPolicy,
			`{"type":"object","properties":{"statement":{"type":"string","minLength":1,"maxLength":65536}},"required":["statement"],"additionalProperties":false}`,
			[]tool.OutputKind{tool.OutputTable, tool.OutputFacts, tool.OutputText, tool.OutputArtifact}),
		registration(router, source, "desktop", "session_info", OperationDesktopInfo,
			"Inspect desktop session", "Read the attached RDP or VNC session status, display size and target metadata.",
			[]tool.Protocol{tool.ProtocolRDP, tool.ProtocolVNC}, []tool.Capability{"desktop.session_info"}, tool.RiskReadOnly, tool.ApprovalNever,
			`{"type":"object","properties":{},"additionalProperties":false}`,
			[]tool.OutputKind{tool.OutputFacts}),
		registration(router, source, "desktop", "capture", OperationDesktopCapture,
			"Capture desktop", "Capture the current frame of the attached RDP or VNC desktop.",
			[]tool.Protocol{tool.ProtocolRDP, tool.ProtocolVNC}, []tool.Capability{"desktop.capture"}, tool.RiskReadOnly, tool.ApprovalNever,
			`{"type":"object","properties":{"quality":{"type":"integer","minimum":30,"maximum":100}},"additionalProperties":false}`,
			[]tool.OutputKind{tool.OutputImage, tool.OutputArtifact}),
		registration(router, source, "desktop", "input", OperationDesktopInput,
			"Control desktop", "Send keyboard or pointer input to the attached RDP or VNC desktop.",
			[]tool.Protocol{tool.ProtocolRDP, tool.ProtocolVNC}, []tool.Capability{"desktop.input"}, tool.RiskMedium, tool.ApprovalByPolicy,
			`{"type":"object","properties":{"events":{"type":"array","items":{"type":"object"},"minItems":1,"maxItems":100}},"required":["events"],"additionalProperties":false}`,
			[]tool.OutputKind{tool.OutputFacts}),
	}
}

func registration(router *Router, source tool.ToolSourceRef, namespace, name string, operation Operation,
	displayName, description string, protocols []tool.Protocol, capabilities []tool.Capability,
	risk tool.RiskLevel, approval tool.ApprovalMode, schema string, outputs []tool.OutputKind,
) tool.ToolRegistration {
	timeout := 30 * time.Second
	if operation == OperationTerminalExecute {
		timeout = 2 * time.Minute
	}
	return tool.ToolRegistration{Descriptor: tool.ToolDescriptor{
		ID: tool.ToolID{Namespace: namespace, Name: name, Version: "1.0.0"}, DisplayName: displayName,
		Description: description, WhenToUse: description, InputSchema: json.RawMessage(schema), OutputKinds: outputs,
		Protocols: protocols, Capabilities: capabilities, Risk: risk, Approval: approval,
		Disclosure: tool.DisclosureAttachment, DefaultTimeout: timeout, Source: source,
	}, Factory: routeFactory{router: router, operation: operation}}
}
