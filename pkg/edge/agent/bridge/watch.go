package bridge

import (
	"context"
	"github.com/liaisonio/liaison/pkg/proto"
	"time"
)

// Caller holds s.mu. Closing and replacing broadcasts without dropping changes.
func (s *session) changedLocked() {
	s.updated = time.Now()
	s.revision++
	if s.changed != nil {
		close(s.changed)
	}
	s.changed = make(chan struct{})
}
func (s *session) watch(ctx context.Context, revision uint64) proto.EdgeAgentResult {
	s.mu.Lock()
	if s.changed == nil {
		s.changed = make(chan struct{})
	}
	changed := s.changed
	wait := s.revision == revision && !s.closed
	s.mu.Unlock()
	if wait {
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return result("unavailable")
		case <-timer.C:
		case <-changed:
		}
	}
	// Coalesce token bursts, without a fixed one-second polling delay.
	timer := time.NewTimer(60 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return result("unavailable")
	case <-timer.C:
	}
	return s.snapshot()
}
