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

type pendingInput struct {
	view       proto.AgentInputRequest
	request    rpc.Message
	turn, item string
}

func (s *session) queueInput(event rpc.Message) bool {
	if event.Method != "item/tool/requestUserInput" || len(event.Params) > 24576 {
		return false
	}
	if _, ok := s.agent.(agentruntime.InputSession); !ok {
		return false
	}
	var p struct {
		Thread    string `json:"threadId"`
		Turn      string `json:"turnId"`
		Item      string `json:"itemId"`
		Blocking  *bool  `json:"isBlocking"`
		Questions []struct {
			ID       string                   `json:"id"`
			Header   string                   `json:"header"`
			Question string                   `json:"question"`
			Other    bool                     `json:"isOther"`
			Secret   bool                     `json:"isSecret"`
			Options  []proto.AgentInputOption `json:"options"`
		} `json:"questions"`
	}
	if json.Unmarshal(event.Params, &p) != nil || p.Item == "" || len(p.Item) > 256 || len(p.Questions) == 0 || len(p.Questions) > 3 {
		return false
	}
	view := proto.AgentInputRequest{Blocking: p.Blocking == nil || *p.Blocking}
	seen := map[string]bool{}
	for _, q := range p.Questions {
		if q.ID == "" || len(q.ID) > 128 || seen[q.ID] || q.Question == "" || len(q.Question) > 4096 || len(q.Header) > 256 || len(q.Options) > 8 {
			return false
		}
		seen[q.ID] = true
		labels := map[string]bool{}
		for _, o := range q.Options {
			if o.Label == "" || len(o.Label) > 512 || len(o.Description) > 2048 || labels[o.Label] {
				return false
			}
			labels[o.Label] = true
		}
		view.Questions = append(view.Questions, proto.AgentInputQuestion{ID: q.ID, Header: q.Header, Question: q.Question, IsOther: q.Other, IsSecret: q.Secret, Options: q.Options})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.access == "" || s.closed || !s.running || p.Thread != s.thread || p.Turn == "" || (s.turn != "" && p.Turn != s.turn) || len(s.inputs) >= 4 {
		return false
	}
	for _, input := range s.inputs {
		if string(input.request.ID) == string(event.ID) {
			return true
		}
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return false
	}
	view.ID = hex.EncodeToString(id[:])
	s.inputs = append(s.inputs, pendingInput{view: view, request: event, turn: p.Turn, item: p.Item})
	s.recordActivityLocked(p.Item, "userInput", "", 0, false)
	s.recordActivityOutputLocked(p.Item, inputSummary(view, nil), false)
	s.changedLocked()
	return true
}

func inputSummary(view proto.AgentInputRequest, answers map[string][]string) string {
	var b strings.Builder
	for _, q := range view.Questions {
		b.WriteString(q.Question)
		b.WriteByte('\n')
		if a, ok := answers[q.ID]; ok {
			b.WriteString("→ ")
			if q.IsSecret {
				b.WriteString("[hidden]")
			} else {
				b.WriteString(strings.Join(a, ", "))
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func (s *session) inputSnapshotLocked() []proto.AgentInputRequest {
	var out []proto.AgentInputRequest
	if !s.closed && s.turn != "" {
		for _, input := range s.inputs {
			if input.turn == s.turn {
				out = append(out, input.view)
			}
		}
	}
	return out
}

func (s *session) answerInput(ctx context.Context, req proto.EdgeAgentRequest) proto.EdgeAgentResult {
	capable, ok := s.agent.(agentruntime.InputSession)
	if !ok || s.access == "" {
		return result("upgrade_required")
	}
	s.mu.Lock()
	for i, input := range s.inputs {
		if input.view.ID != req.InputID {
			continue
		}
		if s.closed || !s.running || s.turn == "" || input.turn != s.turn || len(req.Answers) != len(input.view.Questions) {
			s.mu.Unlock()
			return result("invalid_request")
		}
		answers := map[string][]string{}
		for _, q := range input.view.Questions {
			for _, a := range req.Answers {
				if a.QuestionID != q.ID {
					continue
				}
				allowed := len(q.Options) == 0 || q.IsOther
				for _, o := range q.Options {
					if a.Answers[0] == o.Label {
						allowed = true
					}
				}
				if allowed {
					answers[q.ID] = append([]string{}, a.Answers...)
				}
			}
		}
		if len(answers) != len(input.view.Questions) {
			s.mu.Unlock()
			return result("invalid_request")
		}
		// Consume once before IO. A network failure must never replay a user's answer.
		s.inputs = append(s.inputs[:i], s.inputs[i+1:]...)
		s.recordActivityLocked(input.item, "userInput", "completed", 0, true)
		s.recordActivityOutputLocked(input.item, inputSummary(input.view, answers), false)
		s.changedLocked()
		s.mu.Unlock()
		if err := capable.AnswerInput(ctx, input.request, answers); err != nil {
			s.stop("unavailable")
		}
		return s.snapshot()
	}
	s.mu.Unlock()
	return result("invalid_request")
}

// Native resolution can arrive after a timeout or a response from another surface.
func (s *session) resolveRequestLocked(id json.RawMessage) {
	for i := len(s.inputs) - 1; i >= 0; i-- {
		if string(s.inputs[i].request.ID) == string(id) {
			s.recordActivityLocked(s.inputs[i].item, "userInput", "cancelled", 0, true)
			s.inputs = append(s.inputs[:i], s.inputs[i+1:]...)
			s.changedLocked()
		}
	}
	for i := len(s.approvals) - 1; i >= 0; i-- {
		if string(s.approvals[i].request.ID) == string(id) {
			s.approvals = append(s.approvals[:i], s.approvals[i+1:]...)
			s.changedLocked()
		}
	}
}
