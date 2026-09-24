package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
)

type pendingApproval struct {
	view    proto.AgentApproval
	request rpc.Message
	turn    string
}

// Only complete, inspectable command requests can be approved remotely.
// File-root/session permission grants and other request types stay fail-closed.
func (s *session) queueApproval(event rpc.Message) bool {
	if event.Method != "item/commandExecution/requestApproval" || len(event.Params) > 32768 {
		return false
	}
	if _, ok := s.agent.(agentruntime.PermissionSession); !ok {
		return false
	}
	var p struct {
		Thread  string          `json:"threadId"`
		Turn    string          `json:"turnId"`
		Command string          `json:"command"`
		Cwd     string          `json:"cwd"`
		Reason  string          `json:"reason"`
		Kind    string          `json:"kind"`
		Network json.RawMessage `json:"networkApprovalContext"`
	}
	if json.Unmarshal(event.Params, &p) != nil || strings.TrimSpace(p.Command) == "" || len(p.Command) > 16384 || (p.Kind != "" && p.Kind != "command") || (len(p.Network) > 0 && string(p.Network) != "null") {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.access == "" || s.closed || !s.running || p.Thread != s.thread || p.Turn == "" || (s.turn != "" && p.Turn != s.turn) || len(s.approvals) >= 8 {
		return false
	}
	for _, pending := range s.approvals {
		if string(pending.request.ID) == string(event.ID) {
			return true
		}
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return false
	}
	s.approvals = append(s.approvals, pendingApproval{view: proto.AgentApproval{ID: hex.EncodeToString(id[:]), Command: p.Command, Directory: p.Cwd, Reason: p.Reason}, request: event, turn: p.Turn})
	s.changedLocked()
	return true
}

func (s *session) permissionSnapshotLocked() ([]proto.AgentApproval, bool, string) {
	_, capable := s.agent.(agentruntime.PermissionSession)
	mode := s.permissionMode
	if mode == "" {
		mode = "read-only"
	}
	var approvals []proto.AgentApproval
	if !s.closed {
		for _, p := range s.approvals {
			if s.turn != "" && p.turn == s.turn {
				approvals = append(approvals, p.view)
			}
		}
	}
	return approvals, capable && s.access != "", mode
}

func (s *session) managePermissions(ctx context.Context, req proto.EdgeAgentRequest) proto.EdgeAgentResult {
	capable, ok := s.agent.(agentruntime.PermissionSession)
	if !ok || s.access == "" {
		return result("upgrade_required")
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return result("session_closed")
	}
	if req.Action == "permissions" {
		if s.running {
			s.mu.Unlock()
			return result("busy")
		}
		if err := capable.SetPermissionMode(req.PermissionMode); err != nil {
			s.mu.Unlock()
			return result("invalid_request")
		}
		s.permissionMode = req.PermissionMode
		s.changedLocked()
		s.mu.Unlock()
		return s.snapshot()
	}
	for i, pending := range s.approvals {
		if pending.view.ID != req.ApprovalID {
			continue
		}
		if !s.running || s.turn == "" || pending.turn != s.turn {
			s.mu.Unlock()
			return result("invalid_request")
		}
		// Consume before writing; an uncertain write must never be replayed.
		s.approvals = append(s.approvals[:i], s.approvals[i+1:]...)
		var err error
		if req.Decision == "accept" {
			err = capable.ApproveCommand(ctx, pending.request)
		} else {
			err = s.agent.RejectRequest(ctx, pending.request)
		}
		s.changedLocked()
		s.mu.Unlock()
		if err != nil {
			s.stop("unavailable")
		}
		return s.snapshot()
	}
	s.mu.Unlock()
	return result("invalid_request")
}
