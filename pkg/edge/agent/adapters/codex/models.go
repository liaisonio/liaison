package codex

import (
	"context"
	"encoding/json"
	"errors"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"strings"
	"unicode"
)

func (s *Session) Models(ctx context.Context) ([]agentruntime.Model, error) {
	var models []agentruntime.Model
	cursor := ""
	seen := map[string]bool{}
	for page := 0; page < 8; page++ {
		params := map[string]any{"limit": 32, "includeHidden": false}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, err := s.client.Call(ctx, "model/list", params)
		if err != nil {
			return nil, err
		}
		var response struct {
			Data []struct {
				Model       string `json:"model"`
				DisplayName string `json:"displayName"`
				Hidden      bool   `json:"hidden"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if json.Unmarshal(data, &response) != nil || response.Data == nil || len(response.Data) > 256 {
			return nil, errors.New("invalid model catalog")
		}
		for _, m := range response.Data {
			if m.Hidden || m.Model == "" || len(m.Model) > 128 || strings.ContainsFunc(m.Model, unicode.IsControl) || seen[m.Model] {
				continue
			}
			seen[m.Model] = true
			name := m.DisplayName
			if name == "" || len(name) > 256 || strings.ContainsFunc(name, unicode.IsControl) {
				name = m.Model
			}
			models = append(models, agentruntime.Model{ID: m.Model, Name: name})
			if len(models) > 256 {
				return nil, errors.New("model catalog limit reached")
			}
		}
		if response.NextCursor == nil || *response.NextCursor == "" {
			return models, nil
		}
		if *response.NextCursor == cursor {
			return nil, errors.New("invalid model cursor")
		}
		cursor = *response.NextCursor
	}
	return nil, errors.New("model catalog limit reached")
}

func (s *Session) SendModel(ctx context.Context, thread, text, skill, model string) (string, error) {
	return s.send(ctx, thread, text, skill, model)
}
