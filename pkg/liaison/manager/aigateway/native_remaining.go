package aigateway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"math"
)

func nativeObject(raw []byte) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return nil, ErrResponse
	}
	if v := obj["error"]; len(v) > 0 && !bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return nil, ErrResponse
	}
	return obj, nil
}

// responseTokens are cumulative totals; reasoning and cache details are already
// included. Never add those details again or turn missing accounting into zero.
func responseTokens(raw []byte, u *Usage) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return ErrResponse
	}
	var input, output *int64
	for field, dst := range map[string]**int64{"input_tokens": &input, "output_tokens": &output} {
		if v, ok := values[field]; ok {
			if json.Unmarshal(v, dst) != nil || *dst == nil || **dst < 0 {
				return ErrResponse
			}
		}
	}
	if input != nil && output != nil && *input > math.MaxInt64-*output {
		return ErrResponse
	}
	if input != nil {
		u.Input = input
	}
	if output != nil {
		u.Output = output
	}
	return nil
}

// PrepareResponses supports stateless generation with client function tools.
// Stored response, file, conversation and hosted-tool references are not owned
// by the calling Liaison user, so they cannot be forwarded to a shared account.
func PrepareResponses(raw []byte, allowed map[string]string) (Prepared, error) {
	p := Prepared{Operation: "responses"}
	obj, err := nativeObject(raw)
	if err != nil || len(raw) > 1<<20 {
		return p, ErrUnsupported
	}
	for key := range obj {
		switch key {
		case "model", "input", "instructions", "stream", "store", "temperature", "top_p", "max_output_tokens", "text", "reasoning", "tools", "tool_choice", "parallel_tool_calls", "metadata", "truncation", "service_tier":
		default:
			return p, ErrUnsupported
		}
	}
	if json.Unmarshal(obj["model"], &p.Alias) != nil {
		return p, ErrUnsupported
	}
	model, ok := allowed[p.Alias]
	if !ok {
		return p, ErrModelDenied
	}
	if v, ok := obj["instructions"]; ok {
		var text *string
		if json.Unmarshal(v, &text) != nil || text == nil {
			return p, ErrUnsupported
		}
	}
	if v, ok := obj["stream"]; ok {
		var b *bool
		if json.Unmarshal(v, &b) != nil || b == nil {
			return p, ErrUnsupported
		}
		p.Stream = *b
	}
	if v, ok := obj["store"]; ok {
		var b *bool
		if json.Unmarshal(v, &b) != nil || b == nil || *b {
			return p, ErrUnsupported
		}
	}
	var text string
	if json.Unmarshal(obj["input"], &text) != nil || bytes.Equal(bytes.TrimSpace(obj["input"]), []byte("null")) {
		var items []map[string]json.RawMessage
		if json.Unmarshal(obj["input"], &items) != nil || len(items) == 0 || len(items) > 1000 {
			return p, ErrUnsupported
		}
		for _, item := range items {
			var kind, role string
			if v := item["type"]; len(v) > 0 && json.Unmarshal(v, &kind) != nil {
				return p, ErrUnsupported
			}
			switch kind {
			case "", "message":
				if json.Unmarshal(item["role"], &role) != nil || role != "user" && role != "assistant" && role != "system" && role != "developer" {
					return p, ErrUnsupported
				}
				for key := range item {
					if key != "type" && key != "role" && key != "content" {
						return p, ErrUnsupported
					}
				}
				var content string
				if json.Unmarshal(item["content"], &content) != nil || bytes.Equal(bytes.TrimSpace(item["content"]), []byte("null")) {
					var parts []map[string]json.RawMessage
					if json.Unmarshal(item["content"], &parts) != nil || len(parts) == 0 {
						return p, ErrUnsupported
					}
					for _, part := range parts {
						var typ string
						if json.Unmarshal(part["type"], &typ) != nil {
							return p, ErrUnsupported
						}
						if typ != "input_text" && typ != "output_text" {
							return p, ErrUnsupported
						}
						var text *string
						if json.Unmarshal(part["text"], &text) != nil || text == nil {
							return p, ErrUnsupported
						}
						for key := range part {
							if key != "type" && key != "text" && key != "annotations" {
								return p, ErrUnsupported
							}
						}
					}
				}
			case "function_call", "function_call_output":
				for key := range item {
					if key != "type" && key != "call_id" && key != "name" && key != "arguments" && key != "output" {
						return p, ErrUnsupported
					}
				}
			default:
				return p, ErrUnsupported
			}
		}
	}
	if v, ok := obj["tools"]; ok {
		var tools []map[string]json.RawMessage
		if json.Unmarshal(v, &tools) != nil || tools == nil || len(tools) > 128 {
			return p, ErrUnsupported
		}
		for _, tool := range tools {
			var kind string
			if json.Unmarshal(tool["type"], &kind) != nil || kind != "function" {
				return p, ErrUnsupported
			}
		}
	}
	obj["store"] = json.RawMessage("false")
	obj["model"], _ = json.Marshal(model) // Strings always encode.
	p.Body, err = json.Marshal(obj)
	return p, err
}

func responsesEnvelope(raw []byte, alias string, u *Usage, terminal bool) ([]byte, error) {
	obj, err := nativeObject(raw)
	if err != nil {
		return nil, err
	}
	var kind, status, id string
	if json.Unmarshal(obj["object"], &kind) != nil || kind != "response" || json.Unmarshal(obj["id"], &id) != nil || id == "" || json.Unmarshal(obj["status"], &status) != nil {
		return nil, ErrResponse
	}
	if err = responseTokens(obj["usage"], u); err != nil {
		return nil, err
	}
	if terminal && status != "completed" && status != "incomplete" {
		return nil, ErrResponse
	}
	if !terminal && status != "in_progress" && status != "queued" {
		return nil, ErrResponse
	}
	obj["model"], _ = json.Marshal(alias) // Strings always encode.
	return json.Marshal(obj)
}
func RewriteResponsesJSON(raw []byte, alias string, u *Usage) ([]byte, error) {
	data, err := responsesEnvelope(raw, alias, u, true)
	if err == nil {
		u.Complete = true
	}
	return data, err
}

// relayNativeFrames only dispatches complete bounded SSE events. Callbacks may
// stop on a protocol terminal event; scanner EOF is never implicitly success.
func relayNativeFrames(reader io.Reader, dispatch func([]byte) (bool, error)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var frame bytes.Buffer
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			if frame.Len() == 0 {
				continue
			}
			done, err := dispatch(bytes.TrimSpace(frame.Bytes()))
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			frame.Reset()
		} else if bytes.HasPrefix(line, []byte("data:")) {
			if frame.Len()+len(line) > 1<<20 {
				return ErrResponse
			}
			frame.Write(bytes.TrimPrefix(line, []byte("data:")))
			frame.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return ErrResponse
}
func RelayResponsesSSE(reader io.Reader, alias string, emit func([]byte) error, u *Usage) error {
	started := false
	return relayNativeFrames(reader, func(raw []byte) (bool, error) {
		obj, err := nativeObject(raw)
		if err != nil {
			return false, err
		}
		var kind string
		if json.Unmarshal(obj["type"], &kind) != nil {
			return false, ErrResponse
		}
		done := false
		switch kind {
		case "response.created", "response.in_progress":
			if kind == "response.created" && started {
				return false, ErrResponse
			}
			obj["response"], err = responsesEnvelope(obj["response"], alias, u, false)
			started = true
		case "response.completed", "response.incomplete":
			if !started {
				return false, ErrResponse
			}
			obj["response"], err = responsesEnvelope(obj["response"], alias, u, true)
			done = true
		case "response.output_item.added", "response.output_item.done", "response.content_part.added", "response.content_part.done", "response.output_text.delta", "response.output_text.done", "response.refusal.delta", "response.refusal.done", "response.function_call_arguments.delta", "response.function_call_arguments.done", "response.reasoning_summary_part.added", "response.reasoning_summary_part.done", "response.reasoning_summary_text.delta", "response.reasoning_summary_text.done", "response.reasoning_text.delta", "response.reasoning_text.done":
			if !started {
				return false, ErrResponse
			}
		default:
			return false, ErrResponse
		}
		if err != nil {
			return false, err
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return false, err
		}
		if err = emit(append(append([]byte("event: "+kind+"\ndata: "), data...), '\n', '\n')); err != nil {
			return false, err
		}
		if done {
			u.Complete = true
		}
		return done, nil
	})
}

func PrepareQwen(raw []byte, allowed map[string]string, stream bool) (Prepared, error) {
	p := Prepared{Operation: "services/aigc/text-generation/generation", Stream: stream}
	obj, err := nativeObject(raw)
	if err != nil || len(raw) > 1<<20 {
		return p, ErrUnsupported
	}
	for key := range obj {
		if key != "model" && key != "input" && key != "parameters" {
			return p, ErrUnsupported
		}
	}
	if json.Unmarshal(obj["model"], &p.Alias) != nil {
		return p, ErrUnsupported
	}
	model, ok := allowed[p.Alias]
	if !ok {
		return p, ErrModelDenied
	}
	var input map[string]json.RawMessage
	if json.Unmarshal(obj["input"], &input) != nil || len(input) != 1 {
		return p, ErrUnsupported
	}
	var messages []struct {
		Role       string            `json:"role"`
		Content    *string           `json:"content"`
		ToolCalls  []json.RawMessage `json:"tool_calls,omitempty"`
		ToolCallID string            `json:"tool_call_id,omitempty"`
		Name       string            `json:"name,omitempty"`
	}
	dec := json.NewDecoder(bytes.NewReader(input["messages"]))
	dec.DisallowUnknownFields()
	if dec.Decode(&messages) != nil || len(messages) == 0 || len(messages) > 1000 {
		return p, ErrUnsupported
	}
	for _, m := range messages {
		if m.Role != "system" && m.Role != "user" && m.Role != "assistant" && m.Role != "tool" || m.Content == nil {
			return p, ErrUnsupported
		}
	}
	if v, ok := obj["parameters"]; ok {
		var params map[string]json.RawMessage
		if json.Unmarshal(v, &params) != nil || params == nil {
			return p, ErrUnsupported
		}
		for key := range params {
			switch key {
			case "result_format", "incremental_output", "max_tokens", "temperature", "top_p", "top_k", "seed", "stop", "repetition_penalty", "presence_penalty", "enable_thinking", "thinking_budget", "tools", "tool_choice", "parallel_tool_calls":
			default:
				return p, ErrUnsupported
			}
		}
	}
	obj["model"], _ = json.Marshal(model)
	p.Body, err = json.Marshal(obj)
	return p, err
}
func qwenChunk(raw []byte, u *Usage) ([]byte, bool, error) {
	obj, err := nativeObject(raw)
	if err != nil {
		return nil, false, err
	}
	if obj["code"] != nil {
		return nil, false, ErrResponse
	}
	var output struct {
		Text         *string `json:"text"`
		FinishReason string  `json:"finish_reason"`
		Choices      []struct {
			FinishReason string          `json:"finish_reason"`
			Message      json.RawMessage `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(obj["output"], &output) != nil {
		return nil, false, ErrResponse
	}
	done := false
	terminal := func(reason string) bool { return reason == "stop" || reason == "length" || reason == "tool_calls" }
	if len(output.Choices) > 0 {
		done = true
		for _, c := range output.Choices {
			if len(c.Message) == 0 {
				return nil, false, ErrResponse
			}
			done = done && terminal(c.FinishReason)
		}
	} else if output.Text != nil {
		done = terminal(output.FinishReason)
	} else {
		return nil, false, ErrResponse
	}
	if err = responseTokens(obj["usage"], u); err != nil {
		return nil, false, err
	}
	// DashScope normally omits model; never forward a private ID if supplied.
	delete(obj, "model")
	data, err := json.Marshal(obj)
	return data, done, err
}
func RewriteQwenJSON(raw []byte, u *Usage) ([]byte, error) {
	data, done, err := qwenChunk(raw, u)
	if err != nil {
		return nil, err
	}
	if !done {
		return nil, ErrResponse
	}
	u.Complete = true
	return data, nil
}
func RelayQwenSSE(reader io.Reader, emit func([]byte) error, u *Usage) error {
	return relayNativeFrames(reader, func(raw []byte) (bool, error) {
		data, done, err := qwenChunk(raw, u)
		if err != nil {
			return false, err
		}
		if err = emit(append(append([]byte("data: "), data...), '\n', '\n')); err != nil {
			return false, err
		}
		if done {
			u.Complete = true
		}
		return done, nil
	})
}

// ChatProtocol identifies the existing Chat Completions wire codec, not vendor.
func ChatProtocol(protocol string) string {
	if protocol == "openai" || protocol == "ark" {
		return "openai-compatible"
	}
	return protocol
}

func NativeClientProtocols(protocol string) []string {
	switch protocol {
	case "gemini", "qwen":
		return []string{protocol}
	case "openai", "ark":
		return []string{protocol, "openai-compatible"}
	case "anthropic", "ollama":
		return []string{"openai-compatible", protocol}
	default:
		return []string{"openai-compatible"}
	}
}
