package aigateway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strings"
)

// GeminiOperation validates the whole public model path before it can reach a
// connector. The returned alias still needs authorization through allowed models.
func GeminiOperation(operation string) (alias string, stream, ok bool) {
	if !strings.HasPrefix(operation, "v1beta/") {
		return "", false, false
	}
	op := strings.TrimPrefix(operation, "v1beta/")
	if !allowedOperation("gemini", "POST", op) {
		return "", false, false
	}
	alias, action, _ := strings.Cut(strings.TrimPrefix(op, "models/"), ":")
	return alias, action == "streamGenerateContent", true
}

// PrepareGemini preserves stateless native content. Files, caches and server-side
// tools need separate ownership controls and are deliberately not accepted here.
func PrepareGemini(raw []byte, alias string, stream bool, allowed map[string]string) (Prepared, error) {
	p := Prepared{Alias: alias, Stream: stream}
	model, ok := allowed[alias]
	if !ok {
		return p, ErrModelDenied
	}
	model = strings.TrimPrefix(model, "models/")
	if !safeGeminiModel(model) {
		return p, ErrUnsupported
	}
	var obj map[string]json.RawMessage
	if len(raw) > 1<<20 || json.Unmarshal(raw, &obj) != nil || obj == nil {
		return p, ErrUnsupported
	}
	for field := range obj {
		switch field {
		case "contents", "systemInstruction", "generationConfig", "safetySettings", "tools", "toolConfig":
		default:
			return p, ErrUnsupported
		}
	}
	var contents []json.RawMessage
	if json.Unmarshal(obj["contents"], &contents) != nil || len(contents) == 0 || len(contents) > 1000 {
		return p, ErrUnsupported
	}
	for _, content := range contents {
		if !validGeminiContent(content, false) {
			return p, ErrUnsupported
		}
	}
	if v, ok := obj["systemInstruction"]; ok && !validGeminiContent(v, true) {
		return p, ErrUnsupported
	}
	if v, ok := obj["generationConfig"]; ok {
		var config map[string]json.RawMessage
		if json.Unmarshal(v, &config) != nil || config == nil {
			return p, ErrUnsupported
		}
		if count, ok := config["candidateCount"]; ok {
			var n *int
			if json.Unmarshal(count, &n) != nil || n == nil || *n != 1 {
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
			var functions []map[string]json.RawMessage
			if len(tool) != 1 || json.Unmarshal(tool["functionDeclarations"], &functions) != nil || len(functions) == 0 || len(functions) > 128 {
				return p, ErrUnsupported
			}
			for _, fn := range functions {
				var name string
				if json.Unmarshal(fn["name"], &name) != nil || name == "" {
					return p, ErrUnsupported
				}
			}
		}
	}
	p.Operation = "models/" + model + ":generateContent"
	if stream {
		p.Operation = "models/" + model + ":streamGenerateContent"
	}
	p.Body = append([]byte(nil), raw...)
	return p, nil
}

func validGeminiContent(raw []byte, system bool) bool {
	var content struct {
		Role  string                       `json:"role"`
		Parts []map[string]json.RawMessage `json:"parts"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&content) != nil || len(content.Parts) == 0 || len(content.Parts) > 1000 {
		return false
	}
	if !system && content.Role != "" && content.Role != "user" && content.Role != "model" {
		return false
	}
	for _, part := range content.Parts {
		payloads := 0
		for field, value := range part {
			switch field {
			case "text", "thoughtSignature":
				var text string
				if bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, &text) != nil {
					return false
				}
				if field == "text" {
					payloads++
				}
			case "thought":
				var flag *bool
				if json.Unmarshal(value, &flag) != nil || flag == nil {
					return false
				}
			case "inlineData", "functionCall", "functionResponse":
				if system {
					return false
				}
				var data map[string]json.RawMessage
				if json.Unmarshal(value, &data) != nil || data == nil {
					return false
				}
				for key := range data {
					if field == "inlineData" && key != "mimeType" && key != "data" ||
						field == "functionCall" && key != "id" && key != "name" && key != "args" ||
						field == "functionResponse" && key != "id" && key != "name" && key != "response" {
						return false
					}
				}
				payloads++
			default:
				return false
			}
		}
		if payloads != 1 {
			return false
		}
	}
	return true
}

func geminiChunk(raw []byte, alias string, u *Usage) ([]byte, bool, error) {
	obj, err := responseObject(raw)
	if err != nil {
		return nil, false, err
	}
	var candidates []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finishReason"`
	}
	if v, ok := obj["candidates"]; ok && json.Unmarshal(v, &candidates) != nil {
		return nil, false, ErrResponse
	}
	if len(candidates) > 1 || len(candidates) == 1 && candidates[0].Index != 0 {
		return nil, false, ErrResponse
	}
	done := len(candidates) == 1 && candidates[0].FinishReason != "" && candidates[0].FinishReason != "FINISH_REASON_UNSPECIFIED"
	if done {
		switch candidates[0].FinishReason {
		case "STOP", "MAX_TOKENS", "SAFETY", "RECITATION", "LANGUAGE", "OTHER", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII", "MALFORMED_FUNCTION_CALL", "IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT", "IMAGE_OTHER", "NO_IMAGE", "IMAGE_RECITATION", "UNEXPECTED_TOOL_CALL", "TOO_MANY_TOOL_CALLS", "MISSING_THOUGHT_SIGNATURE":
		default:
			return nil, false, ErrResponse
		}
	}
	var feedback struct {
		BlockReason string `json:"blockReason"`
	}
	if v, ok := obj["promptFeedback"]; ok {
		if json.Unmarshal(v, &feedback) != nil {
			return nil, false, ErrResponse
		}
		if feedback.BlockReason != "" && feedback.BlockReason != "BLOCK_REASON_UNSPECIFIED" {
			switch feedback.BlockReason {
			case "SAFETY", "OTHER", "BLOCKLIST", "PROHIBITED_CONTENT", "IMAGE_SAFETY":
			default:
				return nil, false, ErrResponse
			}
			done = true
		}
	}
	if len(candidates) == 0 && !done && obj["usageMetadata"] == nil {
		return nil, false, ErrResponse
	}
	if v, ok := obj["usageMetadata"]; ok {
		var counters map[string]json.RawMessage
		if json.Unmarshal(v, &counters) != nil || counters == nil {
			return nil, false, ErrResponse
		}
		values := map[string]int64{}
		for _, key := range []string{"promptTokenCount", "candidatesTokenCount", "thoughtsTokenCount", "cachedContentTokenCount", "totalTokenCount"} {
			if v, ok := counters[key]; ok {
				var n *int64
				if json.Unmarshal(v, &n) != nil || n == nil || *n < 0 {
					return nil, false, ErrResponse
				}
				values[key] = *n
			}
		}
		if n, ok := values["promptTokenCount"]; ok {
			u.Input = &n
		}
		if n, ok := values["candidatesTokenCount"]; ok {
			thoughts := values["thoughtsTokenCount"]
			if n > math.MaxInt64-thoughts {
				return nil, false, ErrResponse
			}
			n += thoughts
			u.Output = &n
		}
		if u.Input != nil && u.Output != nil && *u.Input > math.MaxInt64-*u.Output {
			return nil, false, ErrResponse
		}
	}
	obj["modelVersion"], _ = json.Marshal(alias) // Strings cannot fail JSON encoding.
	data, err := json.Marshal(obj)
	return data, done, err
}

func RewriteGeminiJSON(raw []byte, alias string, u *Usage) ([]byte, error) {
	data, done, err := geminiChunk(raw, alias, u)
	if err != nil {
		return nil, err
	}
	if !done {
		return nil, ErrResponse
	}
	u.Complete = true
	return data, nil
}

// Gemini has no [DONE] marker. Require a terminal candidate/block indication and
// clean EOF, retaining any trailing usage frame instead of ending prematurely.
func RelayGeminiSSE(reader io.Reader, alias string, emit func([]byte) error, u *Usage) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var frame bytes.Buffer
	ended := false
	dispatch := func() error {
		if frame.Len() == 0 {
			return nil
		}
		data, done, err := geminiChunk(bytes.TrimSpace(frame.Bytes()), alias, u)
		if err != nil {
			return err
		}
		if ended {
			var obj map[string]json.RawMessage
			if json.Unmarshal(data, &obj) != nil || obj["candidates"] != nil {
				return ErrResponse
			}
		}
		if err = emit(append(append([]byte("data: "), data...), '\n', '\n')); err != nil {
			return err
		}
		ended = ended || done
		frame.Reset()
		return nil
	}
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			if err := dispatch(); err != nil {
				return err
			}
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
	// An unterminated SSE frame is not a delivered event.
	if frame.Len() != 0 || !ended {
		return ErrResponse
	}
	u.Complete = true
	return nil
}
