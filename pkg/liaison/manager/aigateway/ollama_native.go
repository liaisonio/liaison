package aigateway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"math"
)

// PrepareOllamaChat preserves native tools, images and thinking. Shared model
// lifecycle controls (keep_alive, load-only requests) are not consumer operations.
func PrepareOllamaChat(raw []byte, allowed map[string]string) (Prepared, error) {
	p := Prepared{Operation: "chat", Stream: true}
	var obj map[string]json.RawMessage
	if len(raw) > 1<<20 || json.Unmarshal(raw, &obj) != nil || obj == nil {
		return p, ErrUnsupported
	}
	for field := range obj {
		switch field {
		case "model", "messages", "stream", "tools", "think", "format", "options", "logprobs", "top_logprobs":
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
	if v, ok := obj["stream"]; ok && (bytes.Equal(bytes.TrimSpace(v), []byte("null")) || json.Unmarshal(v, &p.Stream) != nil) {
		return p, ErrUnsupported
	}
	if raw, ok := obj["think"]; ok {
		var flag *bool
		var level string
		if (json.Unmarshal(raw, &flag) != nil || flag == nil) &&
			(json.Unmarshal(raw, &level) != nil || level != "low" && level != "medium" && level != "high" && level != "max") {
			return p, ErrUnsupported
		}
	}
	if raw, ok := obj["logprobs"]; ok {
		var flag *bool
		if json.Unmarshal(raw, &flag) != nil || flag == nil {
			return p, ErrUnsupported
		}
	}
	if raw, ok := obj["top_logprobs"]; ok {
		var n *int
		if json.Unmarshal(raw, &n) != nil || n == nil || *n < 0 || *n > 20 {
			return p, ErrUnsupported
		}
	}
	if raw, ok := obj["format"]; ok {
		var name string
		var schema map[string]json.RawMessage
		if (json.Unmarshal(raw, &name) != nil || name != "json") && (json.Unmarshal(raw, &schema) != nil || schema == nil) {
			return p, ErrUnsupported
		}
	}
	if raw, ok := obj["tools"]; ok {
		var tools []struct {
			Type     string `json:"type"`
			Function *struct {
				Name        string                     `json:"name"`
				Description string                     `json:"description,omitempty"`
				Parameters  map[string]json.RawMessage `json:"parameters"`
			} `json:"function"`
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if dec.Decode(&tools) != nil || tools == nil || len(tools) > 128 {
			return p, ErrUnsupported
		}
		for _, tool := range tools {
			if tool.Type != "function" || tool.Function == nil || tool.Function.Name == "" || tool.Function.Parameters == nil {
				return p, ErrUnsupported
			}
		}
	}
	var messages []struct {
		Role      string            `json:"role"`
		Content   *string           `json:"content"`
		Thinking  string            `json:"thinking,omitempty"`
		Images    []string          `json:"images,omitempty"`
		ToolCalls []json.RawMessage `json:"tool_calls,omitempty"`
		ToolName  string            `json:"tool_name,omitempty"`
	}
	dec := json.NewDecoder(bytes.NewReader(obj["messages"]))
	dec.DisallowUnknownFields()
	if dec.Decode(&messages) != nil || len(messages) == 0 || len(messages) > 1000 {
		return p, ErrUnsupported
	}
	for _, m := range messages {
		if m.Role != "system" && m.Role != "user" && m.Role != "assistant" && m.Role != "tool" || m.Content == nil {
			return p, ErrUnsupported
		}
	}
	if raw, ok := obj["options"]; ok {
		var opts map[string]json.RawMessage
		if json.Unmarshal(raw, &opts) != nil || opts == nil {
			return p, ErrUnsupported
		}
		for key, value := range opts {
			switch key {
			case "temperature", "top_p", "min_p", "repeat_penalty", "presence_penalty", "frequency_penalty":
				var n *float64
				if json.Unmarshal(value, &n) != nil || n == nil || *n < -2 || *n > 100 {
					return p, ErrUnsupported
				}
			case "num_predict", "top_k", "seed", "repeat_last_n":
				var n *int64
				if json.Unmarshal(value, &n) != nil || n == nil || key == "num_predict" && (*n < 1 || *n > 32768) {
					return p, ErrUnsupported
				}
			case "stop":
				var stops []string
				if json.Unmarshal(value, &stops) != nil || stops == nil || len(stops) > 64 {
					return p, ErrUnsupported
				}
			default:
				// Reject resource controls such as num_gpu, num_ctx and mmap.
				return p, ErrUnsupported
			}
		}
	}
	obj["model"], _ = json.Marshal(model) // A string is always JSON-encodable.
	var err error
	p.Body, err = json.Marshal(obj)
	return p, err
}

func nativeOllamaChunk(raw []byte, alias string, u *Usage) ([]byte, bool, error) {
	obj, err := responseObject(raw)
	if err != nil {
		return nil, false, err
	}
	var envelope struct {
		Model   string `json:"model"`
		Done    *bool  `json:"done"`
		Message *struct {
			Role string `json:"role"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Model == "" || envelope.Done == nil || envelope.Message == nil || envelope.Message.Role != "assistant" {
		return nil, false, ErrResponse
	}
	// Counters are cumulative, never sum successive chunks. Missing is unknown.
	var input, output *int64
	for field, dst := range map[string]**int64{"prompt_eval_count": &input, "eval_count": &output} {
		if value, ok := obj[field]; ok {
			if json.Unmarshal(value, dst) != nil || *dst == nil || **dst < 0 {
				return nil, false, ErrResponse
			}
		}
	}
	if input != nil && output != nil && *input > math.MaxInt64-*output {
		return nil, false, ErrResponse
	}
	if *envelope.Done {
		u.Input, u.Output = input, output
	}
	obj["model"], _ = json.Marshal(alias) // A string is always JSON-encodable.
	data, err := json.Marshal(obj)
	return data, *envelope.Done, err
}

func RewriteOllamaChatJSON(raw []byte, alias string, u *Usage) ([]byte, error) {
	data, done, err := nativeOllamaChunk(raw, alias, u)
	if err != nil {
		return nil, err
	}
	if !done {
		return nil, ErrResponse
	}
	u.Complete = true
	return data, nil
}

// RelayOllamaChat emits native NDJSON. Only delivered done:true is completion;
// EOF, malformed accounting or a downstream write error cannot become success.
func RelayOllamaChat(reader io.Reader, alias string, emit func([]byte) error, u *Usage) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		data, done, err := nativeOllamaChunk(raw, alias, u)
		if err != nil {
			return err
		}
		if err = emit(append(data, '\n')); err != nil {
			return err
		}
		if done {
			u.Complete = true
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return ErrResponse
}
