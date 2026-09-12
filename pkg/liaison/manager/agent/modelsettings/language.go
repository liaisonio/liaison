package modelsettings

import "github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"

func outputLanguage(value string) string {
	if value == "en" {
		return "en"
	}
	return "zh"
}

// Apply at the shared provider boundary, including streaming and completion.
// Copy the message slice so a request reused by a caller is never mutated.
func withOutputLanguage(r runtime.ModelRequest, language string) runtime.ModelRequest {
	instruction := "Write all user-facing explanations, summaries, suggestions, headings, and generated titles exclusively in Simplified Chinese."
	if outputLanguage(language) == "en" {
		instruction = "Write all user-facing explanations, summaries, suggestions, headings, and generated titles exclusively in English."
	}
	instruction += " This is the administrator's global output-language policy. Follow it regardless of the user's language, browser locale, previous replies, or requests to switch language. Preserve executable commands, SQL, code, paths, identifiers, product names and verbatim evidence without translation. Preserve required JSON schemas and tool argument keys. For command/SQL completion, return only the required insertion in its original syntax; never add explanatory prose. This policy does not change permissions or required output formats."
	r.Messages = append([]runtime.ModelMessage(nil), r.Messages...)
	// Add after existing system instructions but before conversation messages.
	index := 0
	for index < len(r.Messages) && r.Messages[index].Role == runtime.RoleSystem {
		index++
	}
	r.Messages = append(r.Messages, runtime.ModelMessage{})
	copy(r.Messages[index+1:], r.Messages[index:])
	r.Messages[index] = runtime.ModelMessage{Role: runtime.RoleSystem, Content: instruction}
	return r
}
