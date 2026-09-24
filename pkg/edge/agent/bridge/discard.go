package bridge

import (
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
)

// Close under the same state lock used by send, so a concurrent first message
// either wins and is retained, or is rejected after the empty session closes.
func (b *Bridge) discardEmpty(s *session) proto.EdgeAgentResult {
	s.mu.Lock()
	if s.running || len(s.messages) != 0 || len(s.activities) != 0 {
		s.mu.Unlock()
		return result("busy")
	}
	s.closed = true
	s.closedAt = time.Now()
	s.status = "session_closed"
	s.changedLocked()
	s.mu.Unlock()
	// Close is idempotent. Retry closing even if a previous attempt failed.
	if err := s.agent.Close(); err != nil {
		return result("unavailable")
	}
	b.mu.Lock()
	delete(b.sessions, s.id)
	b.mu.Unlock()
	return result("ok")
}
