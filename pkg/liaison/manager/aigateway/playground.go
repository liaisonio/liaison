package aigateway

import (
	"bytes"
	"encoding/json"
	"io"
)

// PreparePlayground is a bounded text-only adapter for the console, not a claim
// of general Chat Completions compatibility for Gemini or DashScope.
func PreparePlayground(raw []byte, allowed map[string]string, protocol string) (Prepared, error) {
	if protocol != "qwen" && protocol != "gemini" {
		return Prepared{}, ErrUnsupported
	}
	var request struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		MaxTokens int  `json:"max_tokens"`
		Stream    bool `json:"stream"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&request) != nil || len(request.Messages) == 0 || request.MaxTokens < 1 || request.MaxTokens > 32768 || !request.Stream {
		return Prepared{}, ErrUnsupported
	}
	// Accept exactly one JSON value; trailing whitespace is harmless, but a
	// second value or malformed suffix must not be silently discarded.
	var trailing json.RawMessage
	if dec.Decode(&trailing) != io.EOF {
		return Prepared{}, ErrUnsupported
	}
	for _, m := range request.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			return Prepared{}, ErrUnsupported
		}
	}
	if protocol == "qwen" {
		body, err := json.Marshal(map[string]any{"model": request.Model, "input": map[string]any{"messages": request.Messages}, "parameters": map[string]any{"result_format": "message", "incremental_output": true, "max_tokens": request.MaxTokens}})
		if err != nil {
			return Prepared{}, err
		}
		return PrepareQwen(body, allowed, true)
	}
	contents := []any{}
	for _, m := range request.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]any{"role": role, "parts": []any{map[string]string{"text": m.Content}}})
	}
	body, err := json.Marshal(map[string]any{"contents": contents, "generationConfig": map[string]any{"maxOutputTokens": request.MaxTokens}})
	if err != nil {
		return Prepared{}, err
	}
	return PrepareGemini(body, request.Model, true, allowed)
}

func RelayPlayground(reader io.Reader, protocol, alias string, emit func([]byte) error, u *Usage) error {
	adapt := func(frame []byte) error {
		raw := bytes.TrimSpace(bytes.TrimPrefix(frame, []byte("data:")))
		var obj struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text    string `json:"text"`
						Thought bool   `json:"thought"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
			Output struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			} `json:"output"`
		}
		if json.Unmarshal(raw, &obj) != nil {
			return ErrResponse
		}
		text := ""
		if protocol == "qwen" {
			if len(obj.Output.Choices) > 0 {
				text = obj.Output.Choices[0].Message.Content
			}
		} else if len(obj.Candidates) > 0 {
			for _, p := range obj.Candidates[0].Content.Parts {
				if !p.Thought {
					text += p.Text
				}
			}
		}
		data, err := json.Marshal(map[string]any{"model": alias, "choices": []any{map[string]any{"index": 0, "delta": map[string]string{"content": text}}}})
		if err != nil {
			return err
		}
		return emit(append(append([]byte("data: "), data...), '\n', '\n'))
	}
	var err error
	if protocol == "qwen" {
		err = RelayQwenSSE(reader, adapt, u)
	} else {
		err = RelayGeminiSSE(reader, alias, adapt, u)
	}
	if err != nil {
		return err
	}
	if err = emit([]byte("data: [DONE]\n\n")); err != nil {
		u.Complete = false
	}
	return err
}
