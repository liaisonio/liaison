package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"path/filepath"
	"strings"
)

type localSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Enabled     bool   `json:"enabled"`
}

func (s *Session) Skills(ctx context.Context) ([]agentruntime.Skill, error) {
	data, err := s.client.Call(ctx, "skills/list", map[string]any{"cwds": []string{s.directory}, "forceReload": true})
	if err != nil {
		return nil, err
	}
	items, local, err := parseSkills(data, s.directory)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.skills = local
	s.mu.Unlock()
	return items, nil
}

func parseSkills(data []byte, directory string) ([]agentruntime.Skill, map[string]localSkill, error) {
	var response struct {
		Data []struct {
			Cwd    string            `json:"cwd"`
			Skills []localSkill      `json:"skills"`
			Errors []json.RawMessage `json:"errors"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &response) != nil || response.Data == nil {
		return nil, nil, errors.New("skills unavailable")
	}
	items := []agentruntime.Skill{}
	local := map[string]localSkill{}
	for _, group := range response.Data {
		if filepath.Clean(group.Cwd) != filepath.Clean(directory) {
			continue
		}
		if len(group.Errors) > 0 {
			return nil, nil, errors.New("skills unavailable")
		}
		for _, skill := range group.Skills {
			if !skill.Enabled || skill.Name == "" || len(skill.Name) > 200 || len(skill.Description) > 2048 || len(skill.Path) > 4096 || !filepath.IsAbs(skill.Path) || strings.ContainsRune(skill.Path, 0) {
				continue
			}
			if len(items) >= 100 {
				break
			}
			hash := sha256.Sum256([]byte(skill.Name + "\x00" + skill.Path))
			id := hex.EncodeToString(hash[:16])
			if _, exists := local[id]; exists {
				continue
			}
			local[id] = skill
			items = append(items, agentruntime.Skill{ID: id, Name: skill.Name, Description: skill.Description})
		}
	}
	return items, local, nil
}
