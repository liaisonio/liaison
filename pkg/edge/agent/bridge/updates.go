package bridge

import (
	"context"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
)

func checkAuthentication(ctx context.Context, agent agentruntime.Session) (bool, error) {
	if capable, ok := agent.(agentruntime.AuthenticationSession); ok {
		return capable.CheckAuthentication(ctx)
	}
	// Continue without claiming a credential check. Native turn failure remains
	// authoritative for providers without an authentication preflight.
	return true, nil
}

func (b *Bridge) consumeUpdates(s *session, updates <-chan agentruntime.Update) {
	for {
		select {
		case <-b.ctx.Done():
			s.stop("session_closed")
			return
		case u, ok := <-updates:
			if !ok {
				s.stop("session_closed")
				return
			}
			s.mu.Lock()
			if u.Kind == agentruntime.InteractionRequested || u.Kind == agentruntime.InteractionResolved {
				accepted := s.applyInteractionLocked(u)
				s.mu.Unlock()
				if !accepted {
					s.stop("unavailable")
					return
				}
				continue
			}
			s.applyUpdateLocked(u)
			s.mu.Unlock()
		}
	}
}

func (s *session) applyUpdateLocked(u agentruntime.Update) {
	if s.closed || !s.running || u.ThreadID != s.thread || u.TurnID == "" || (s.turn != "" && u.TurnID != s.turn) {
		return
	}
	s.truncated = s.truncated || u.Truncated
	switch u.Kind {
	case agentruntime.SessionMetadata:
		if len(u.Text) <= 256 && len(u.Status) <= 64 {
			s.model, s.version = u.Text, u.Status
		}
	case agentruntime.MessageDelta:
		if u.Text != "" {
			s.recordTurnTimingLocked(&s.timing.FirstReplyMS)
		}
		last := len(s.messages) - 1
		if last < 0 || s.messages[last].Role != "assistant" || (s.messages[last].ItemID != "" && s.messages[last].ItemID != u.ItemID) {
			s.messages = append(s.messages, proto.EdgeAgentMessage{Role: "assistant"})
			last++
		}
		s.messages[last].ItemID = u.ItemID
		s.messages[last].Text += u.Text
		s.bytes += len(u.Text)
		s.boundDisplayLocked()
	case agentruntime.ActivityUpdate:
		kind := "dynamicToolCall"
		if u.Tool == "Bash" {
			kind = "commandExecution"
		}
		s.recordActivityLocked(u.ItemID, kind, u.Status, 0, u.Status != "running")
		s.recordActivityDetailsLocked(nativeActivity{ID: u.ItemID, Type: kind, Tool: u.Tool, Command: u.Command, Path: u.Path})
		if u.Text != "" {
			s.recordActivityOutputLocked(u.ItemID, u.Text, false)
		}
	case agentruntime.TurnEnded:
		s.recordTurnTimingLocked(&s.timing.FinishedMS)
		s.running = false
		s.approvals = nil
		s.inputs = nil
		for i := range s.activities {
			if s.activities[i].Status == "running" {
				s.activities[i].Status = "ended"
			}
		}
		if u.Status == "failed" {
			s.status = "turn_failed"
		}
	default:
		return
	}
	s.changedLocked()
}
