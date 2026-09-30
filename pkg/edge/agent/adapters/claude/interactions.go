package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
)

// 浏览器只获得审核所需字段，原始 input 保存在 Client 内。
func (c *Client) interaction(id string) (*agentruntime.Interaction, error) {
	c.mu.Lock()
	p, ok := c.approvals[id]
	c.mu.Unlock()
	if !ok {
		return nil, ErrApproval
	}
	v := &agentruntime.Interaction{ID: id, Tool: p.Tool}
	switch p.Tool {
	case "Bash":
		if len(p.Input) > 24<<10 {
			return nil, ErrApproval
		}
		var input struct {
			Command        string `json:"command"`
			Description    string `json:"description"`
			Timeout        int    `json:"timeout"`
			Background     bool   `json:"run_in_background"`
			DisableSandbox bool   `json:"dangerouslyDisableSandbox"`
		}
		decoder := json.NewDecoder(bytes.NewReader(p.Input))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || strings.TrimSpace(input.Command) == "" || len(input.Command) > 16384 || len(input.Description) > 4096 || input.Background || input.DisableSandbox || input.Timeout < 0 || input.Timeout > 600000 {
			return nil, ErrApproval
		}
		v.Command, v.Reason = input.Command, input.Description
	case "AskUserQuestion":
		questions, err := parseQuestions(p.Input)
		if err != nil || len(questions) > 3 {
			return nil, ErrAnswer
		}
		for _, q := range questions {
			if q.MultiSelect {
				return nil, ErrAnswer
			}
		}
		v.Questions = questions
	default:
		return nil, ErrApproval
	}
	return v, nil
}

func (s *Session) handleInteraction(event Event) bool {
	s.mu.Lock()
	d, thread, turn := s.driver, s.thread, s.turn
	s.mu.Unlock()
	if d == nil || turn == "" {
		return false
	}
	u := agentruntime.Update{ThreadID: thread, TurnID: turn, Kind: agentruntime.InteractionResolved, Interaction: &agentruntime.Interaction{ID: event.RequestID}}
	if event.Type == "control_request" {
		view, err := d.interaction(event.RequestID)
		if err != nil {
			// 原生取消可能已被读循环处理，而通知仍在队列中。
			d.mu.Lock()
			_, pending := d.approvals[event.RequestID]
			d.mu.Unlock()
			if !pending {
				return true
			}
			// 不支持的形态明确拒绝，不能假装完整展示后让用户批准。
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			defer cancel()
			return d.Decide(ctx, event.RequestID, false) == nil
		}
		u.Kind, u.Interaction = agentruntime.InteractionRequested, view
	}
	s.mu.Lock()
	if s.interactions == nil {
		s.interactions = make(map[string]string)
	}
	if event.Type == "control_request" {
		if len(s.interactions) >= 32 {
			s.mu.Unlock()
			return false
		}
		s.interactions[event.RequestID] = turn
	} else {
		delete(s.interactions, event.RequestID)
	}
	s.mu.Unlock()
	select {
	case s.updates <- u:
		return true
	default:
		return false
	}
}

func (s *Session) interactionDriver(turn, id string) (*Driver, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil || s.driver == nil || turn == "" || turn != s.turn || s.interactions[id] != turn {
		return nil, ErrApproval
	}
	delete(s.interactions, id)
	return s.driver, nil
}

func (s *Session) Decide(ctx context.Context, turn, id string, allow bool) error {
	d, err := s.interactionDriver(turn, id)
	if err != nil {
		return err
	}
	if allow {
		v, err := d.interaction(id)
		if err != nil || v.Tool != "Bash" {
			return ErrApproval
		}
	}
	return d.Decide(ctx, id, allow)
}

func (s *Session) Answer(ctx context.Context, turn, id string, answers map[string][]string) error {
	d, err := s.interactionDriver(turn, id)
	if err != nil {
		return err
	}
	if _, err := d.interaction(id); err != nil {
		return err
	}
	return d.Answer(ctx, id, answers)
}
