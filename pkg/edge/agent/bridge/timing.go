package bridge

import (
	"github.com/liaisonio/liaison/pkg/proto"
	"time"
)

// 调用方持有 session.mu。每个阶段只记录一次，使用本进程单调时钟。
func (s *session) recordTurnTimingLocked(value **int64) {
	if s.turnStarted.IsZero() || *value != nil {
		return
	}
	ms := max(int64(0), time.Since(s.turnStarted).Milliseconds())
	*value = &ms
	if d := s.diagnostics; d != nil && !d.finished {
		switch value {
		case &s.timing.FirstReplyMS:
			d.progress(time.Now())
			d.record.FirstReplyMS = &ms
			d.publish("agent_first_reply", time.Now())
		case &s.timing.DispatchMS:
			d.record.DispatchMS = &ms
			d.publish("agent_dispatched", time.Now())
		}
	}
}

func (s *session) turnTimingLocked() *proto.AgentTurnTiming {
	if s.turnStarted.IsZero() {
		return nil
	}
	// 指针值只分配不修改，新一轮整体替换，不与已返回快照共享可变内存。
	copy := s.timing
	return &copy
}
