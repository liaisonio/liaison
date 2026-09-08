package model

import (
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"testing"
)

func TestNamespacedToolsRoundTrip(t *testing.T) {
	original := runtime.ModelRequest{Tools: []runtime.ModelTool{{Name: "core.tool_search"}, {Name: "core__tool_search"}}, Messages: []runtime.ModelMessage{{Role: runtime.RoleAssistant, ToolCalls: []runtime.ModelToolCall{{ID: "call", Name: "core.tool_search"}}}}}
	encoded, restore := encodeToolNames(original)
	if !wireToolName.MatchString(encoded.Tools[0].Name) || encoded.Tools[0].Name == encoded.Tools[1].Name {
		t.Fatal("invalid or ambiguous wire names")
	}
	if encoded.Messages[0].ToolCalls[0].Name != encoded.Tools[0].Name {
		t.Fatal("history names differ")
	}
	result := restore(runtime.ModelResponse{ToolCalls: []runtime.ModelToolCall{{ID: "call", Name: encoded.Tools[0].Name}}})
	if result.ToolCalls[0].Name != "core.tool_search" || original.Tools[0].Name != "core.tool_search" || original.Messages[0].ToolCalls[0].Name != "core.tool_search" {
		t.Fatal("roundtrip mutated runtime names")
	}
}
