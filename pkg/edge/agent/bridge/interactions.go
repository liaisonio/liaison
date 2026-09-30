package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
)

// 使用已有的远程卡片结构，不把供应商请求 ID 或工具原始 JSON 传给浏览器。
func (s *session) applyInteractionLocked(u agentruntime.Update) bool {
	v := u.Interaction
	if s.access == "" || s.closed || !s.running || u.ThreadID != s.thread || u.TurnID == "" || (s.turn != "" && s.turn != u.TurnID) || v == nil || v.ID == "" || len(v.ID) > 256 {
		return false
	}
	if _, ok := s.agent.(agentruntime.InteractionSession); !ok {
		return false
	}
	if u.Kind == agentruntime.InteractionResolved {
		for i := len(s.approvals) - 1; i >= 0; i-- {
			if s.approvals[i].nativeID == v.ID && s.approvals[i].turn == u.TurnID {
				s.approvals = append(s.approvals[:i], s.approvals[i+1:]...)
			}
		}
		for i := len(s.inputs) - 1; i >= 0; i-- {
			if s.inputs[i].nativeID == v.ID && s.inputs[i].turn == u.TurnID {
				s.recordActivityLocked(s.inputs[i].item, "userInput", "cancelled", 0, true)
				s.inputs = append(s.inputs[:i], s.inputs[i+1:]...)
			}
		}
		s.changedLocked()
		return true
	}
	for _, p := range s.approvals {
		if p.nativeID == v.ID {
			return true
		}
	}
	for _, p := range s.inputs {
		if p.nativeID == v.ID {
			return true
		}
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return false
	}
	key := hex.EncodeToString(id[:])
	switch v.Tool {
	case "Bash":
		if len(s.approvals) >= 8 || strings.TrimSpace(v.Command) == "" || len(v.Command) > 16384 || len(v.Reason) > 4096 {
			return false
		}
		view := proto.AgentApproval{ID: key, Kind: "commandExecution", Command: v.Command, Directory: s.project, Reason: v.Reason}
		views := []proto.AgentApproval{view}
		for _, p := range s.approvals {
			views = append(views, p.view)
		}
		encoded, err := json.Marshal(views)
		if err != nil || len(encoded) > 128<<10 {
			return false
		}
		s.approvals = append(s.approvals, pendingApproval{nativeID: v.ID, turn: u.TurnID, view: view})
	case "AskUserQuestion":
		if len(s.inputs) >= 4 || len(v.Questions) == 0 || len(v.Questions) > 3 {
			return false
		}
		view := proto.AgentInputRequest{ID: key, Blocking: true}
		seen := map[string]bool{}
		for _, q := range v.Questions {
			if q.MultiSelect || q.ID == "" || len(q.ID) > 128 || seen[q.ID] || strings.TrimSpace(q.Text) == "" || len(q.Text) > 4096 || len(q.Header) > 256 || len(q.Options) > 8 {
				return false
			}
			seen[q.ID] = true
			question := proto.AgentInputQuestion{ID: q.ID, Header: q.Header, Question: q.Text, IsOther: true}
			labels := map[string]bool{}
			for _, o := range q.Options {
				if strings.TrimSpace(o.Label) == "" || len(o.Label) > 512 || len(o.Description) > 2048 || labels[o.Label] {
					return false
				}
				labels[o.Label] = true
				question.Options = append(question.Options, proto.AgentInputOption{Label: o.Label, Description: o.Description})
			}
			view.Questions = append(view.Questions, question)
		}
		encoded, err := json.Marshal(view)
		if err != nil || len(encoded) > 24<<10 {
			return false
		}
		s.inputs = append(s.inputs, pendingInput{nativeID: v.ID, turn: u.TurnID, item: "input:" + key, view: view})
		s.recordActivityLocked("input:"+key, "userInput", "", 0, false)
		s.recordActivityOutputLocked("input:"+key, inputSummary(view, nil), false)
	default:
		return false
	}
	s.changedLocked()
	return true
}
