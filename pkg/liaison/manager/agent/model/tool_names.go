package model

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"regexp"
)

var wireToolName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Runtime IDs retain namespaces. Provider wire names must satisfy the API's
// restricted alphabet and length, and must map back before tool authorization.
func encodeToolNames(request runtime.ModelRequest) (runtime.ModelRequest, func(runtime.ModelResponse) runtime.ModelResponse) {
	names := map[string]string{}
	alias := func(name string) string {
		encoded := name
		if !wireToolName.MatchString(name) {
			sum := sha256.Sum256([]byte(name))
			encoded = "t_" + hex.EncodeToString(sum[:30])
		}
		names[encoded] = name
		return encoded
	}
	request.Tools = append([]runtime.ModelTool(nil), request.Tools...)
	for i, t := range request.Tools {
		request.Tools[i].Name = alias(t.Name)
	}
	request.Messages = append([]runtime.ModelMessage(nil), request.Messages...)
	for i, m := range request.Messages {
		calls := append([]runtime.ModelToolCall(nil), m.ToolCalls...)
		for j, c := range calls {
			calls[j].Name = alias(c.Name)
		}
		request.Messages[i].ToolCalls = calls
	}
	return request, func(response runtime.ModelResponse) runtime.ModelResponse {
		for i, c := range response.ToolCalls {
			if name, ok := names[c.Name]; ok {
				response.ToolCalls[i].Name = name
			}
		}
		return response
	}
}
