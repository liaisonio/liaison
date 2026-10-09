package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jumboframes/armorigo/log"
)

// Only allowlisted metadata crosses this logging boundary. Never add native
// payloads, model names, paths, commands, prompts or errors to this record.
type turnDiagnosticRecord struct {
	Service            string `json:"service"`
	Event              string `json:"event"`
	TraceID            string `json:"trace_id"`
	SessionID          string `json:"session_id,omitempty"`
	Window             uint64 `json:"window"`
	ElapsedMS          int64  `json:"elapsed_ms"`
	FirstProgressMS    *int64 `json:"first_progress_ms,omitempty"`
	DispatchMS         *int64 `json:"dispatch_ms,omitempty"`
	FirstReplyMS       *int64 `json:"first_reply_ms,omitempty"`
	HumanWaitMS        int64  `json:"human_wait_ms"`
	ToolActiveMS       int64  `json:"tool_active_ms"`
	ToolCount          int    `json:"tool_count"`
	Incomplete         bool   `json:"incomplete"`
	InterruptRequested bool   `json:"interrupt_requested"`
	Outcome            string `json:"outcome,omitempty"`
}

type observedInterval struct {
	start time.Time
	total time.Duration
}

func (i *observedInterval) observe(active bool, now time.Time) {
	if active && i.start.IsZero() {
		i.start = now
	} else if !active && !i.start.IsZero() {
		i.total += max(time.Duration(0), now.Sub(i.start))
		i.start = time.Time{}
	}
}

func (i observedInterval) milliseconds(now time.Time) int64 {
	if !i.start.IsZero() {
		i.total += max(time.Duration(0), now.Sub(i.start))
	}
	return i.total.Milliseconds()
}

type turnDiagnostics struct {
	record       turnDiagnosticRecord
	start        time.Time
	finished     bool
	human, tools observedInterval
	// Bounded independently from display eviction. True means still active.
	items       map[string]bool
	activeTools int
	emit        func(turnDiagnosticRecord)
}

func newTurnDiagnostics(id string, window uint64, now time.Time, emit func(turnDiagnosticRecord)) *turnDiagnostics {
	if _, err := hex.DecodeString(id); err != nil || len(id) != 32 {
		id = "" // An untrusted/native identifier must never become a log field.
	}
	d := &turnDiagnostics{start: now, items: make(map[string]bool), emit: emit,
		record: turnDiagnosticRecord{Service: "liaison-edge", TraceID: rand.Text(), SessionID: id, Window: window}}
	d.publish("agent_turn_started", now)
	return d
}

func writeTurnDiagnostic(record turnDiagnosticRecord) {
	data, err := json.Marshal(record)
	if err != nil {
		log.Warn("agent diagnostic encoding failed")
		return
	}
	// Keep the existing Edge log destination, rotation and level configuration.
	log.Info(string(data))
}

func (d *turnDiagnostics) publish(event string, now time.Time) {
	d.record.Event = event
	d.record.ElapsedMS = max(int64(0), now.Sub(d.start).Milliseconds())
	d.record.HumanWaitMS = d.human.milliseconds(now)
	d.record.ToolActiveMS = d.tools.milliseconds(now)
	d.emit(d.record)
}

func (d *turnDiagnostics) progress(now time.Time) {
	if d == nil || d.finished || d.record.FirstProgressMS != nil {
		return
	}
	ms := max(int64(0), now.Sub(d.start).Milliseconds())
	d.record.FirstProgressMS = &ms
	d.publish("agent_first_progress", now)
}

func (d *turnDiagnostics) activity(key, kind string, completed bool, now time.Time) {
	if d == nil || d.finished {
		return
	}
	d.progress(now)
	switch kind {
	case "commandExecution", "fileChange", "webSearch", "mcpToolCall", "dynamicToolCall", "imageView":
	default:
		return
	}
	active, found := d.items[key]
	if !found {
		if len(d.items) >= 256 {
			d.record.Incomplete = true
			return
		}
		d.record.ToolCount++
		if !completed {
			d.activeTools++
		}
		d.items[key] = !completed
	} else if active && completed {
		d.activeTools--
		d.items[key] = false
	}
	d.tools.observe(d.activeTools > 0, now)
}

// Caller holds session.mu. At most five milestones are emitted per turn;
// token updates and approval churn only update in-memory counters.
func (s *session) observeDiagnosticsLocked(now time.Time) {
	d := s.diagnostics
	if d == nil || d.finished {
		return
	}
	waiting := len(s.approvals) > 0 || len(s.inputs) > 0
	if waiting {
		d.progress(now)
	}
	d.human.observe(s.running && waiting, now)
	if !s.running {
		d.tools.observe(false, now)
		d.record.Outcome = "ended"
		if s.closed {
			d.record.Outcome = "closed"
		}
		if s.status == "turn_failed" {
			d.record.Outcome = "failed"
		}
		d.record.Incomplete = d.record.Incomplete || d.activeTools > 0
		d.publish("agent_turn_ended", now)
		d.finished = true
	}
}
