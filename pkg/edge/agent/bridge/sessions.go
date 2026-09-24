package bridge

import (
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/liaisonio/liaison/pkg/proto"
)

// Called under s.mu. Merely listing sessions never extends their idle lease.
func (s *session) expirationLocked(now time.Time) (expire, forget bool) {
	if s.access == "" {
		expired := now.Sub(s.touched) > 90*time.Second || now.Sub(s.created) > 30*time.Minute
		return expired, expired
	}
	if s.closed {
		return false, now.Sub(s.closedAt) > 24*time.Hour
	}
	recent := s.touched
	if s.updated.After(recent) {
		recent = s.updated
	}
	started := s.created
	if !s.runtimeStarted.IsZero() {
		started = s.runtimeStarted
	}
	// An active native turn owns its lifetime, including waits for approval.
	// The lease reclaims idle processes, never interrupts a long-running task.
	return !s.running && (now.Sub(recent) > 30*time.Minute || now.Sub(started) > 24*time.Hour), false
}

func (b *Bridge) listSessions(owner, access string) proto.EdgeAgentResult {
	b.mu.Lock()
	list := make([]*session, 0, len(b.sessions))
	for _, s := range b.sessions {
		if s.owner == owner && s.access == access {
			list = append(list, s)
		}
	}
	b.mu.Unlock()
	out := result("ok")
	out.SessionsAvailable = true
	out.SessionManagement = true
	out.Sessions = make([]proto.AgentSessionSummary, 0, len(list))
	for _, s := range list {
		s.mu.Lock()
		title := s.displayTitleLocked()
		out.Sessions = append(out.Sessions, proto.AgentSessionSummary{SessionID: s.id, ThreadID: s.thread, Title: title, Project: s.project, UpdatedAt: s.updated.UTC().Format(time.RFC3339Nano), Running: s.running, Closed: s.closed, Status: s.status})
		s.mu.Unlock()
	}
	sort.Slice(out.Sessions, func(i, j int) bool { return out.Sessions[i].UpdatedAt > out.Sessions[j].UpdatedAt })
	return out
}

func validNativeTitle(title string) bool {
	return strings.TrimSpace(title) != "" && utf8.ValidString(title) && utf8.RuneCountInString(title) <= 120 && !strings.ContainsFunc(title, unicode.IsControl)
}
func (s *session) displayTitleLocked() string {
	if validNativeTitle(s.title) {
		return s.title
	}
	for _, message := range s.messages {
		if message.Role == "user" {
			title := strings.Join(strings.Fields(message.Text), " ")
			runes := []rune(title)
			if len(runes) > 80 {
				title = string(runes[:80]) + "…"
			}
			return title
		}
	}
	return "" // The client localizes "New conversation" until the first message.
}
