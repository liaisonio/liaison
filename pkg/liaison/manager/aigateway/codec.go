package aigateway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var ErrUnsupported = errors.New("unsupported request or protocol capability")
var ErrModelDenied = errors.New("model is not allowed")
var ErrResponse = errors.New("invalid or incomplete upstream response")

type Prepared struct {
	Body      []byte
	Alias     string
	Stream    bool
	Operation string
}
type Usage struct {
	Input    *int64
	Output   *int64
	Complete bool
}

// IncludeStreamUsage requests upstream accounting for a limited-key stream.
// Preserve other stream options, but callers cannot opt out of metering.
func IncludeStreamUsage(p *Prepared) error {
	var obj map[string]json.RawMessage
	if p == nil || json.Unmarshal(p.Body, &obj) != nil || obj == nil {
		return ErrUnsupported
	}
	options := map[string]json.RawMessage{}
	if raw := obj["stream_options"]; len(raw) > 0 && string(raw) != "null" {
		if json.Unmarshal(raw, &options) != nil {
			return ErrUnsupported
		}
	}
	options["include_usage"] = json.RawMessage("true")
	encoded, err := json.Marshal(options)
	if err != nil {
		return err
	}
	obj["stream_options"] = encoded
	p.Body, err = json.Marshal(obj)
	return err
}

func Prepare(raw []byte, allowed map[string]string, protocol string) (Prepared, error) {
	var p Prepared
	var obj map[string]json.RawMessage
	if len(raw) > 1<<20 || json.Unmarshal(raw, &obj) != nil || obj == nil {
		return p, ErrUnsupported
	}
	if json.Unmarshal(obj["model"], &p.Alias) != nil {
		return p, ErrUnsupported
	}
	model, ok := allowed[p.Alias]
	if !ok {
		return p, ErrModelDenied
	}
	if b, ok := obj["stream"]; ok && (string(b) == "null" || json.Unmarshal(b, &p.Stream) != nil) {
		return p, ErrUnsupported
	}
	var messages []json.RawMessage
	if json.Unmarshal(obj["messages"], &messages) != nil || len(messages) == 0 || len(messages) > 1000 {
		return p, ErrUnsupported
	}
	encoded, err := json.Marshal(model)
	if err != nil {
		return p, err
	}
	obj["model"] = encoded
	p.Operation = "chat/completions"
	if protocol == "anthropic" {
		// Intentionally text-only conversion. No silent dropping of tools, images,
		// reasoning, response_format, penalties or provider-specific parameters.
		for field := range obj {
			switch field {
			case "model", "messages", "stream", "max_tokens", "temperature", "top_p", "stop":
			default:
				return p, fmt.Errorf("%w: %s", ErrUnsupported, field)
			}
		}
		var converted []map[string]string
		var system []string
		started := false
		for _, rawMessage := range messages {
			var msg struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}
			dec := json.NewDecoder(bytes.NewReader(rawMessage))
			dec.DisallowUnknownFields()
			if dec.Decode(&msg) != nil {
				return p, ErrUnsupported
			}
			switch msg.Role {
			case "system":
				if started {
					return p, ErrUnsupported
				}
				system = append(system, msg.Content)
			case "user", "assistant":
				started = true
				converted = append(converted, map[string]string{"role": msg.Role, "content": msg.Content})
			default:
				return p, ErrUnsupported
			}
		}
		if len(converted) == 0 || converted[0]["role"] != "user" || converted[len(converted)-1]["role"] != "user" {
			return p, ErrUnsupported
		}
		out := map[string]any{"model": model, "messages": converted, "stream": p.Stream, "max_tokens": 1024}
		if len(system) > 0 {
			out["system"] = strings.Join(system, "\n\n")
		}
		for _, name := range []string{"temperature", "top_p"} {
			if v, ok := obj[name]; ok {
				var n float64
				if json.Unmarshal(v, &n) != nil || n < 0 || n > 1 || string(v) == "null" {
					return p, ErrUnsupported
				}
				out[name] = n
			}
		}
		if v, ok := obj["max_tokens"]; ok {
			var n int
			if json.Unmarshal(v, &n) != nil || n < 1 || n > 32768 {
				return p, ErrUnsupported
			}
			out["max_tokens"] = n
		}
		if v, ok := obj["stop"]; ok {
			var stops []string
			var stop string
			if json.Unmarshal(v, &stop) == nil {
				stops = []string{stop}
			} else if json.Unmarshal(v, &stops) != nil || stops == nil {
				return p, ErrUnsupported
			}
			out["stop_sequences"] = stops
		}
		p.Operation = "messages"
		p.Body, err = json.Marshal(out)
		return p, err
	}
	if protocol != "openai-compatible" {
		return p, ErrUnsupported
	}
	p.Body, err = json.Marshal(obj)
	return p, err
}

func responseObject(raw []byte) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil || obj["error"] != nil {
		return nil, ErrResponse
	}
	return obj, nil
}
func observeUsage(obj map[string]json.RawMessage, u *Usage, anthropic bool) {
	var usage map[string]json.RawMessage
	if json.Unmarshal(obj["usage"], &usage) != nil {
		return
	}
	input, output := "prompt_tokens", "completion_tokens"
	if anthropic {
		input, output = "input_tokens", "output_tokens"
	}
	var in, out *int64
	if json.Unmarshal(usage[input], &in) == nil && in != nil && *in >= 0 {
		u.Input = in
	}
	if json.Unmarshal(usage[output], &out) == nil && out != nil && *out >= 0 {
		u.Output = out
	}
}
func finishReason(reason string) (string, error) {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop", nil
	case "max_tokens":
		return "length", nil
	case "refusal":
		return "content_filter", nil
	default:
		return "", ErrResponse
	}
}
func RewriteJSON(raw []byte, protocol, alias string, u *Usage) ([]byte, error) {
	obj, err := responseObject(raw)
	if err != nil {
		return nil, err
	}
	observeUsage(obj, u, protocol == "anthropic")
	if protocol == "openai-compatible" {
		var choices []json.RawMessage
		if json.Unmarshal(obj["choices"], &choices) != nil || len(choices) == 0 {
			return nil, ErrResponse
		}
		obj["model"], err = json.Marshal(alias)
		if err != nil {
			return nil, err
		}
		result, err := json.Marshal(obj)
		u.Complete = err == nil
		return result, err
	}
	var msg struct {
		ID      string `json:"id"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}
	if json.Unmarshal(raw, &msg) != nil {
		return nil, ErrResponse
	}
	text := ""
	for _, c := range msg.Content {
		if c.Type != "text" {
			return nil, ErrResponse
		}
		text += c.Text
	}
	reason, err := finishReason(msg.StopReason)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"id": msg.ID, "object": "chat.completion", "created": time.Now().Unix(), "model": alias, "choices": []any{map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": text}, "finish_reason": reason}}}
	if u.Input != nil && u.Output != nil {
		result["usage"] = map[string]int64{"prompt_tokens": *u.Input, "completion_tokens": *u.Output, "total_tokens": *u.Input + *u.Output}
	}
	data, err := json.Marshal(result)
	u.Complete = err == nil
	return data, err
}

// RelaySSE preserves OpenAI payload fields while rewriting only the model alias.
// Frames are bounded, dispatched on blank lines, and emitted immediately. An
// interrupted stream never gains a synthetic successful [DONE].
func RelaySSE(reader io.Reader, protocol, alias string, emit func([]byte) error, u *Usage) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var frame bytes.Buffer
	id := ""
	started := false
	finished := false
	send := func(delta map[string]string, reason any) error {
		raw, err := json.Marshal(map[string]any{"id": id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": alias, "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
		if err != nil {
			return err
		}
		return emit(append(append([]byte("data: "), raw...), []byte("\n\n")...))
	}
	dispatch := func() error {
		data := bytes.TrimSpace(frame.Bytes())
		if len(data) == 0 {
			return nil
		}
		if protocol == "openai-compatible" {
			if string(data) == "[DONE]" {
				if !started {
					return ErrResponse
				}
				if err := emit([]byte("data: [DONE]\n\n")); err != nil {
					return err
				}
				u.Complete = true
				return nil
			}
			obj, err := responseObject(data)
			if err != nil {
				return err
			}
			observeUsage(obj, u, false)
			var choices []json.RawMessage
			if json.Unmarshal(obj["choices"], &choices) != nil {
				return ErrResponse
			}
			started = true
			obj["model"], err = json.Marshal(alias)
			if err != nil {
				return err
			}
			raw, err := json.Marshal(obj)
			if err != nil {
				return err
			}
			return emit(append(append([]byte("data: "), raw...), []byte("\n\n")...))
		}
		obj, err := responseObject(data)
		if err != nil {
			return err
		}
		var event string
		if json.Unmarshal(obj["type"], &event) != nil {
			return ErrResponse
		}
		switch event {
		case "ping":
			return nil
		case "message_start":
			if started {
				return ErrResponse
			}
			started = true
			message, err := responseObject(obj["message"])
			if err != nil {
				return err
			}
			if json.Unmarshal(message["id"], &id) != nil {
				return ErrResponse
			}
			observeUsage(message, u, true)
			return send(map[string]string{"role": "assistant", "content": ""}, nil)
		case "content_block_start":
			var c struct{ Type, Text string }
			if !started || json.Unmarshal(obj["content_block"], &c) != nil || c.Type != "text" {
				return ErrResponse
			}
			if c.Text != "" {
				return send(map[string]string{"content": c.Text}, nil)
			}
			return nil
		case "content_block_delta":
			var d struct{ Type, Text string }
			if !started || json.Unmarshal(obj["delta"], &d) != nil || d.Type != "text_delta" {
				return ErrResponse
			}
			return send(map[string]string{"content": d.Text}, nil)
		case "content_block_stop":
			return nil
		case "message_delta":
			if !started {
				return ErrResponse
			}
			observeUsage(obj, u, true)
			var d struct {
				StopReason string `json:"stop_reason"`
			}
			if json.Unmarshal(obj["delta"], &d) != nil {
				return ErrResponse
			}
			reason, err := finishReason(d.StopReason)
			if err != nil {
				return err
			}
			finished = true
			return send(map[string]string{}, reason)
		case "message_stop":
			if !finished {
				return ErrResponse
			}
			if err := emit([]byte("data: [DONE]\n\n")); err != nil {
				return err
			}
			u.Complete = true
			return nil
		default:
			return ErrResponse
		}
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			frame.Reset()
			if u.Complete {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if frame.Len() > 0 {
				frame.WriteByte('\n')
			}
			frame.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			if frame.Len() > 1<<20 {
				return ErrResponse
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return ErrResponse
}
