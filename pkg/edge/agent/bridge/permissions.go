package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// Only complete, inspectable command/file requests can be approved remotely.
// File-root/session permission grants and other request types stay fail-closed.
func (s *session) queueApproval(event rpc.Message) bool {
	isFile := event.Method == "item/fileChange/requestApproval"
	if (event.Method != "item/commandExecution/requestApproval" && !isFile) || len(event.ID) == 0 || len(event.Params) > 32768 {
		return false
	}
	if _, ok := s.agent.(agentruntime.PermissionSession); !ok {
		return false
	}
	if isFile {
		if _, ok := s.agent.(agentruntime.FileApprovalSession); !ok {
			return false
		}
	}
	var p struct {
		Item       string          `json:"itemId"`
		GrantRoot  json.RawMessage `json:"grantRoot"`
		Additional json.RawMessage `json:"additionalPermissions"`
		Thread     string          `json:"threadId"`
		Turn       string          `json:"turnId"`
		Command    string          `json:"command"`
		Cwd        string          `json:"cwd"`
		Reason     string          `json:"reason"`
		Kind       string          `json:"kind"`
		Network    json.RawMessage `json:"networkApprovalContext"`
	}
	if json.Unmarshal(event.Params, &p) != nil || (!isFile && (strings.TrimSpace(p.Command) == "" || len(p.Command) > 16384 || (p.Kind != "" && p.Kind != "command"))) || (len(p.Network) > 0 && string(p.Network) != "null") || (len(p.GrantRoot) > 0 && string(p.GrantRoot) != "null") || (len(p.Additional) > 0 && string(p.Additional) != "null") {
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
	view := proto.AgentApproval{Kind: "commandExecution", Command: p.Command, Directory: p.Cwd, Reason: p.Reason}
	if isFile {
		i := s.activityIndexLocked(p.Item)
		if p.Item == "" || i < 0 {
			return false
		}
		a := s.activities[i]
		if a.Kind != "fileChange" || a.Status != "running" || a.Truncated || len(a.Changes) == 0 {
			return false
		}
		for _, change := range a.Changes {
			if change.Diff == "" || !approvalPathInProject(s.project, change.Path) || (change.MovePath != "" && !approvalPathInProject(s.project, change.MovePath)) {
				return false
			}
		}
		view.Kind = "fileChange"
		view.Command = ""
		view.Directory = s.project
		view.Changes = append([]proto.AgentFileChange{}, a.Changes...)
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return false
	}
	view.ID = hex.EncodeToString(id[:])
	views := []proto.AgentApproval{view}
	for _, pending := range s.approvals {
		views = append(views, pending.view)
	}
	encoded, err := json.Marshal(views)
	if err != nil || len(encoded) > 128<<10 {
		return false
	}
	s.approvals = append(s.approvals, pendingApproval{view: view, request: event, turn: p.Turn})
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
		s.changedLocked()
		s.mu.Unlock()
		var err error
		if req.Decision == "accept" {
			if pending.view.Kind == "fileChange" {
				fileCapable, ok := s.agent.(agentruntime.FileApprovalSession)
				if !ok {
					err = agentruntime.ErrUnavailable
				} else {
					err = fileCapable.ApproveFileChange(ctx, pending.request)
				}
			} else {
				err = capable.ApproveCommand(ctx, pending.request)
			}
		} else {
			err = s.agent.RejectRequest(ctx, pending.request)
		}
		if err != nil {
			s.stop("unavailable")
		}
		return s.snapshot()
	}
	s.mu.Unlock()
	return result("invalid_request")
}

// Lexical containment plus an existing canonical parent. New files are allowed;
// symlinks and missing intermediate directories are not remotely approved.
func approvalPathInProject(project, path string) bool {
	if project == "" || path == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(project, path)
	}
	rel, err := filepath.Rel(project, path)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
			return false
		}
		parent, e := filepath.EvalSymlinks(filepath.Dir(path))
		if e != nil {
			return false
		}
		resolved = filepath.Join(parent, filepath.Base(path))
	}
	rel, err = filepath.Rel(project, resolved)
	return err == nil && filepath.IsLocal(rel) && rel != "."
}
