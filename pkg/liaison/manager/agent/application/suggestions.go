package application

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"time"
)

// Suggest 为每个用户、活动连接、编辑器保留独立的轻量辅助会话。
func (s *Service) Suggest(ctx context.Context, actor *model.User, handle, editor string, input assistance.Input, closeSession bool) (assistance.Suggestion, error) {
	if actor == nil || actor.ID == 0 || len(editor) < 8 || len(editor) > 128 || handle == "" || len(handle) > 256 {
		return assistance.Suggestion{}, ErrInvalid
	}
	key := assistanceKey{actor.ID, handle, editor}
	now := time.Now()
	s.assistanceMu.Lock()
	if s.assistants == nil {
		s.assistants = make(map[assistanceKey]assistanceEntry)
	}
	for k, entry := range s.assistants {
		if now.Sub(entry.used) > 10*time.Minute {
			entry.session.Close()
			delete(s.assistants, k)
		}
	}
	entry, exists := s.assistants[key]
	if closeSession {
		if exists {
			entry.session.Close()
			delete(s.assistants, key)
		}
		s.assistanceMu.Unlock()
		return assistance.Suggestion{}, nil
	}
	if s.assistanceGenerator == nil {
		s.assistanceMu.Unlock()
		return assistance.Suggestion{}, ErrUnavailable
	}
	if !exists && len(s.assistants) >= 1024 {
		s.assistanceMu.Unlock()
		return assistance.Suggestion{}, ErrUnavailable
	}
	s.assistanceMu.Unlock()
	if !exists {
		session, err := s.NewAssistanceSession(ctx, actor, handle, s.assistanceGenerator)
		if err != nil {
			return assistance.Suggestion{}, err
		}
		s.assistanceMu.Lock()
		if current, found := s.assistants[key]; found {
			session.Close()
			entry = current
		} else if len(s.assistants) >= 1024 {
			session.Close()
			s.assistanceMu.Unlock()
			return assistance.Suggestion{}, ErrUnavailable
		} else {
			entry = assistanceEntry{session, now}
			s.assistants[key] = entry
		}
		s.assistanceMu.Unlock()
	}
	s.assistanceMu.Lock()
	if current, found := s.assistants[key]; found && current.session == entry.session {
		current.used = now
		s.assistants[key] = current
	}
	s.assistanceMu.Unlock()
	return entry.session.Suggest(ctx, input)
}
