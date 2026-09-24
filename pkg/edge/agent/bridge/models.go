package bridge

import (
	"context"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
)

func (s *session) manageModel(ctx context.Context, req proto.EdgeAgentRequest) proto.EdgeAgentResult {
	capable, ok := s.agent.(agentruntime.ModelSession)
	if !ok {
		return result("upgrade_required")
	}
	s.mu.Lock()
	if s.closed || s.running {
		s.mu.Unlock()
		return result("busy")
	}
	if req.Action == "model" {
		found := false
		for _, m := range s.models {
			if m.ID == req.Model {
				found = true
				break
			}
		}
		if !found {
			s.mu.Unlock()
			return result("invalid_request")
		}
		s.selectedModel = req.Model
		s.model = req.Model
		s.changedLocked()
		s.mu.Unlock()
		return s.snapshot()
	}
	s.mu.Unlock()
	models, err := capable.Models(ctx)
	if err != nil || len(models) > 256 {
		return result("unavailable")
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return result("session_closed")
	}
	s.models = nil
	for _, m := range models {
		s.models = append(s.models, proto.AgentModel{ID: m.ID, Name: m.Name})
	}
	s.modelsAvailable = true
	s.changedLocked()
	s.mu.Unlock()
	return s.snapshot()
}
