package claude

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
)

// Only explicit public catalog fields cross the adapter boundary.
type nativeCatalog struct {
	Models []struct {
		Value string `json:"value"`
		Name  string `json:"displayName"`
	} `json:"models"`
	Commands []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"commands"`
}

func catalogIdentifier(s string) bool {
	return s != "" && len(s) <= 256 && !strings.ContainsAny(s, "\\/ \t\r\n") && !strings.ContainsFunc(s, unicode.IsControl)
}

func (c *Client) models() []agentruntime.Model {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []agentruntime.Model{}
	for _, m := range c.catalog.Models {
		if len(out) >= 256 {
			break
		}
		if catalogIdentifier(m.Value) && len(m.Name) <= 256 {
			out = append(out, agentruntime.Model{ID: m.Value, Name: m.Name})
		}
	}
	return out
}

func (c *Client) skills() []agentruntime.Skill {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []agentruntime.Skill{}
	for _, cmd := range c.catalog.Commands {
		if len(out) >= 256 {
			break
		}
		if catalogIdentifier(cmd.Name) && len(cmd.Description) <= 4096 {
			hash := sha256.Sum256([]byte(cmd.Name))
			out = append(out, agentruntime.Skill{ID: fmt.Sprintf("%x", hash[:16]), Name: "/" + cmd.Name, Description: cmd.Description})
		}
	}
	return out
}

func (s *Session) Models(ctx context.Context) ([]agentruntime.Model, error) {
	s.mu.Lock()
	d := s.driver
	s.mu.Unlock()
	if d == nil || ctx.Err() != nil {
		return nil, agentruntime.ErrUnavailable
	}
	return d.models(), nil
}

func (s *Session) Skills(ctx context.Context) ([]agentruntime.Skill, error) {
	s.mu.Lock()
	d := s.driver
	s.mu.Unlock()
	if d == nil || ctx.Err() != nil {
		return nil, agentruntime.ErrUnavailable
	}
	return d.skills(), nil
}

func (s *Session) SendSkill(ctx context.Context, thread, prompt, skill string) (string, error) {
	return s.sendWithOptions(ctx, thread, prompt, skill, "")
}

func (s *Session) SendModel(ctx context.Context, thread, prompt, skill, model string) (string, error) {
	return s.sendWithOptions(ctx, thread, prompt, skill, model)
}

var _ agentruntime.ModelSession = (*Session)(nil)
var _ agentruntime.SkillSession = (*Session)(nil)
