package aigateway

import (
	"net/http"
	"strings"
)

// Protocol describes upstream transport capabilities, not public route readiness.
// Adding a transport here does not enable its access endpoint or application UI.
type Protocol struct {
	ID       string
	BasePath string
	Models   bool
}

// LookupProtocol deliberately does not infer a vendor from a model identifier.
func LookupProtocol(id string) (Protocol, bool) {
	switch id {
	case "openai", "openai-compatible", "anthropic":
		return Protocol{id, "/v1", true}, true
	case "ark":
		return Protocol{id, "/api/v3", false}, true
	case "qwen":
		return Protocol{id, "/api/v1", false}, true
	case "gemini":
		return Protocol{id, "/v1beta", true}, true
	case "ollama":
		return Protocol{id, "/api", true}, true
	default:
		return Protocol{}, false
	}
}

func allowedOperation(protocol, method, operation string) bool {
	p, ok := LookupProtocol(protocol)
	if !ok {
		return false
	}
	if method == http.MethodGet {
		return p.Models && (operation == "models" && protocol != "ollama" || protocol == "ollama" && operation == "tags")
	}
	if method != http.MethodPost {
		return false
	}
	switch protocol {
	case "openai", "ark":
		return operation == "chat/completions" || operation == "responses"
	case "openai-compatible":
		return operation == "chat/completions"
	case "anthropic":
		return operation == "messages"
	case "ollama":
		return operation == "chat"
	case "qwen":
		return operation == "services/aigc/text-generation/generation"
	case "gemini":
		model, action, ok := strings.Cut(strings.TrimPrefix(operation, "models/"), ":")
		return strings.HasPrefix(operation, "models/") && ok && safeGeminiModel(model) &&
			(action == "generateContent" || action == "streamGenerateContent")
	}
	return false
}

// Model identifiers are path segments here, never caller-supplied URLs or paths.
func safeGeminiModel(model string) bool {
	if len(model) == 0 || len(model) > 256 || model == "." || model == ".." {
		return false
	}
	for _, c := range model {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}
