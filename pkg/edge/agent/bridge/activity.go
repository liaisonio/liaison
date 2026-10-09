package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/liaisonio/liaison/pkg/proto"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// The caller holds the session lock. Only allowlisted activity types are kept.
func (s *session) recordActivityLocked(id, kind, status string, duration int64, completed bool) {
	if id == "" || len(id) > 256 {
		return
	}
	switch kind {
	case "commandExecution", "fileChange", "webSearch", "mcpToolCall", "dynamicToolCall", "reasoning", "contextCompaction", "userInput", "turnDiff", "turnPlan", "plan", "imageView", "enteredReviewMode", "exitedReviewMode":
	default:
		return
	}
	sum := sha256.Sum256([]byte(id))
	key := hex.EncodeToString(sum[:16])
	state := "running"
	if completed {
		state = "completed"
		switch status {
		case "failed":
			state = "failed"
		case "declined":
			state = "declined"
		case "interrupted", "cancelled":
			state = "ended"
		}
	}
	if duration < 0 {
		duration = 0
	}
	for i := range s.activities {
		if s.activities[i].ID == key {
			if s.activities[i].Status != "running" && !completed {
				return
			}
			s.activities[i].Status = state
			s.activities[i].DurationMS = duration
			s.diagnostics.activity(key, kind, completed, time.Now())
			s.changedLocked()
			return
		}
	}
	if len(s.activities) >= 128 {
		// Prefer evicting a completed row; never stop accepting new execution events.
		drop := 0
		for i, a := range s.activities {
			if a.Status != "running" {
				drop = i
				break
			}
		}
		s.activities = append(s.activities[:drop], s.activities[drop+1:]...)
		s.truncated = true
	}
	index := len(s.messages) - 1
	if index < 0 {
		index = 0
	}
	s.activities = append(s.activities, proto.AgentActivity{ID: key, Kind: kind, Status: state, DurationMS: duration, MessageIndex: index})
	s.diagnostics.activity(key, kind, completed, time.Now())
	s.changedLocked()
}

const maxActivityDetails = 96 << 10
const maxActivityText = 16 << 10

type nativeActivity struct {
	Text   string `json:"text"`
	Query  string `json:"query"`
	Tool   string `json:"tool"`
	Server string `json:"server"`
	Path   string `json:"path"`
	Action struct {
		Type    string   `json:"type"`
		Query   string   `json:"query"`
		Queries []string `json:"queries"`
		URL     string   `json:"url"`
		Pattern string   `json:"pattern"`
	} `json:"action"`
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`
	DurationMS int64   `json:"durationMs"`
	Command    string  `json:"command"`
	Cwd        string  `json:"cwd"`
	Output     *string `json:"aggregatedOutput"`
	ExitCode   *int    `json:"exitCode"`
	Changes    []struct {
		Path string `json:"path"`
		Diff string `json:"diff"`
		Kind struct {
			Type     string `json:"type"`
			MovePath string `json:"move_path"`
		} `json:"kind"`
	} `json:"changes"`
}

func activityKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:16])
}
func activitySize(a proto.AgentActivity) int {
	n := len(a.Command) + len(a.Directory) + len(a.Output) + len(a.Label)
	for _, step := range a.Plan {
		n += len(step.Step) + len(step.Status)
	}
	for _, c := range a.Changes {
		n += len(c.Path) + len(c.Diff) + len(c.MovePath)
	}
	return n
}

// Bound display history independently from model text. Truncation never kills a turn.
// Render as plain text; discard terminal control characters and bidi controls.
func boundedActivityText(value string, budget *int, truncated *bool) string {
	limit := min(*budget, maxActivityText)
	var b strings.Builder
	for _, r := range value {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.Is(unicode.Cf, r) {
			continue
		}
		if b.Len()+len(string(r)) > limit {
			*truncated = true
			break
		}
		b.WriteRune(r)
	}
	*budget -= b.Len()
	return b.String()
}
func (s *session) activityIndexLocked(id string) int {
	key := activityKey(id)
	for i := range s.activities {
		if s.activities[i].ID == key {
			return i
		}
	}
	return -1
}
func (s *session) activityBudgetLocked(index int) int {
	n := 0
	for i, a := range s.activities {
		if i != index {
			n += activitySize(a)
		}
	}
	// Reserve room for recent execution details rather than letting the first
	// large command consume every later step's display budget.
	for i := range s.activities {
		if n <= maxActivityDetails-maxActivityText || i == index {
			continue
		}
		a := &s.activities[i]
		if a.Status == "running" {
			continue
		}
		n -= len(a.Output)
		a.Output = ""
		a.Changes = append([]proto.AgentFileChange{}, a.Changes...)
		for j := range a.Changes {
			n -= len(a.Changes[j].Diff)
			a.Changes[j].Diff = ""
		}
		a.Truncated = true
		s.truncated = true
	}
	return max(0, maxActivityDetails-n)
}
func (s *session) recordActivityDetailsLocked(item nativeActivity) {
	// Discovery previews retain their metadata-only contract.
	if s.access == "" {
		return
	}
	i := s.activityIndexLocked(item.ID)
	if i < 0 {
		return
	}
	a := s.activities[i]
	budget := s.activityBudgetLocked(i)
	if item.Type != "commandExecution" && item.Type != "fileChange" {
		// Metadata only: never forward tool arguments/results, raw reasoning,
		// external image data, or provider errors into display history.
		label := ""
		switch item.Type {
		case "webSearch":
			label = item.Query
			if label == "" {
				label = item.Action.Query
			}
			if label == "" {
				label = strings.Join(item.Action.Queries, " · ")
			}
			if label == "" {
				if parsed, err := url.Parse(item.Action.URL); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
					parsed.User = nil
					parsed.RawQuery = ""
					parsed.Fragment = ""
					label = parsed.String()
				}
			}
		case "mcpToolCall", "dynamicToolCall":
			label = strings.Trim(item.Server+" / "+item.Tool, " / ")
		case "imageView":
			label = item.Path
		case "plan":
			a.Output = boundedActivityText(item.Text, &budget, &a.Truncated)
		default:
			return
		}
		a.Label = boundedActivityText(label, &budget, &a.Truncated)
		s.activities[i] = a
		s.changedLocked()
		return
	}
	a.Truncated = false
	a.Command = boundedActivityText(item.Command, &budget, &a.Truncated)
	a.Directory = boundedActivityText(item.Cwd, &budget, &a.Truncated)
	if item.Output != nil {
		a.Output = boundedActivityText(*item.Output, &budget, &a.Truncated)
	} else {
		a.Output = boundedActivityText(a.Output, &budget, &a.Truncated)
	}
	a.ExitCode = item.ExitCode
	a.Changes = nil
	for _, change := range item.Changes {
		if len(a.Changes) >= 32 || budget == 0 {
			a.Truncated = true
			break
		}
		if change.Kind.Type != "add" && change.Kind.Type != "delete" && change.Kind.Type != "update" {
			a.Truncated = true
			continue
		}
		c := proto.AgentFileChange{Kind: change.Kind.Type}
		c.Path = boundedActivityText(change.Path, &budget, &a.Truncated)
		c.MovePath = boundedActivityText(change.Kind.MovePath, &budget, &a.Truncated)
		c.Diff = boundedActivityText(change.Diff, &budget, &a.Truncated)
		a.Changes = append(a.Changes, c)
	}
	s.activities[i] = a
	s.changedLocked()
}

func (s *session) recordPlanLocked(turn, explanation string, steps []proto.AgentPlanStep) {
	if s.access == "" {
		return
	}
	id := "turn-plan:" + turn
	s.recordActivityLocked(id, "turnPlan", "", 0, false)
	i := s.activityIndexLocked(id)
	if i < 0 {
		return
	}
	a := s.activities[i]
	budget := min(maxActivityText, s.activityBudgetLocked(i))
	a.Truncated = false
	a.Output = boundedActivityText(explanation, &budget, &a.Truncated)
	a.Plan = nil
	a.Status = "completed"
	for _, step := range steps {
		if len(a.Plan) >= 32 || budget < 16 {
			a.Truncated = true
			break
		}
		if step.Status != "pending" && step.Status != "inProgress" && step.Status != "completed" {
			continue
		}
		budget -= len(step.Status)
		step.Step = boundedActivityText(step.Step, &budget, &a.Truncated)
		a.Plan = append(a.Plan, step)
		if step.Status != "completed" {
			a.Status = "running"
		}
	}
	s.activities[i] = a
	s.changedLocked()
}
func (s *session) recordActivityOutputLocked(id, text string, appendText bool) {
	if s.access == "" {
		return
	}
	i := s.activityIndexLocked(id)
	if i < 0 {
		return
	}
	a := s.activities[i]
	budget := s.activityBudgetLocked(i) - activitySize(a) + len(a.Output)
	if appendText {
		text = a.Output + text
	}
	a.Output = boundedActivityText(text, &budget, &a.Truncated)
	s.activities[i] = a
	s.changedLocked()
}
