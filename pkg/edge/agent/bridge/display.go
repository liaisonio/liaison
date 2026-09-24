package bridge

import (
	"encoding/json"
	"github.com/liaisonio/liaison/pkg/proto"
	"unicode/utf8"
)

// Display limits never cancel native work. Keep recent output on UTF-8 boundaries.
func tailText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	start := len(text) - limit
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	return text[start:]
}
func (s *session) boundDisplayLocked() {
	for i := range s.messages {
		if len(s.messages[i].Text) > 64<<10 {
			s.messages[i].Text = tailText(s.messages[i].Text, 64<<10)
			s.truncated = true
		}
	}
	s.bytes = 0
	for _, m := range s.messages {
		s.bytes += len(m.Text)
	}
	for len(s.messages) > 96 || (s.bytes > maxOutput && len(s.messages) > 1) {
		s.bytes -= len(s.messages[0].Text)
		s.messages = append([]proto.EdgeAgentMessage{}, s.messages[1:]...)
		s.truncated = true
		for i := range s.activities {
			s.activities[i].MessageIndex = max(0, s.activities[i].MessageIndex-1)
		}
	}
}

// JSON escaping can multiply bytes (e.g. '<' -> '\u003c'). Bound wire size,
// not only Go string lengths, so Manager's encrypted snapshot limit is respected.
func boundedSnapshot(out proto.EdgeAgentResult) proto.EdgeAgentResult {
	for {
		raw, err := json.Marshal(out)
		if err != nil || len(raw) <= 480<<10 {
			return out
		}
		out.Truncated = true
		largest := -1
		for i := range out.Messages {
			if largest < 0 || len(out.Messages[i].Text) > len(out.Messages[largest].Text) {
				largest = i
			}
		}
		if largest >= 0 && len(out.Messages[largest].Text) > 0 {
			out.Messages[largest].Text = tailText(out.Messages[largest].Text, len(out.Messages[largest].Text)/2)
			continue
		}
		if len(out.Activities) > 0 {
			out.Activities = out.Activities[1:]
			continue
		}
		// Live request capabilities must not be silently altered. Their native input
		// is separately bounded; persisted snapshots remove these transient fields.
		return out
	}
}
