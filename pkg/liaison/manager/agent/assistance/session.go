// Package assistance manages cancellable editor lanes. Shell completion uses
// the owning Agent session context but does not start a tool loop or save drafts.
package assistance

import (
	"context"
	"errors"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
)

var (
	ErrInvalid = errors.New("invalid assistance request")
	ErrClosed  = errors.New("assistance session closed")
	ErrStale   = errors.New("assistance input superseded")
)

// Binding 必须由调用层从已认证用户和活动连接解析，不能直接信任浏览器。
type Binding struct {
	OwnerID  uint
	HandleID string
	Protocol string
}

// Input contains editing context plus server-resolved Agent context. Clients
// cannot supply Agent history. Cursor is a UTF-8 byte offset.
type Input struct {
	AgentSessionID string
	AgentContext   []runtime.ModelMessage // Server-resolved Shell Agent context; never accepted from HTTP.
	Revision       uint64
	Text           string
	Cursor         int
	RecentOutput   string
	Schema         string
	ShellContext   string // Server-resolved, explicitly shared WebSSH context only.
}

type Suggestion struct {
	Revision uint64
	Cursor   int
	Text     string
}

// Generator 只能生成文字；不提供工具执行入口。
type Generator interface {
	Suggest(context.Context, Binding, Input) (string, error)
}

// Guard 每次生成前后检查所有权、连接存活及权限；调用层负责连接断开时 Close。
type Guard interface {
	Check(context.Context, Binding) error
}

type Session struct {
	mu        sync.Mutex
	binding   Binding
	generator Generator
	guard     Guard
	revision  uint64
	closed    bool
	cancel    context.CancelFunc
}

func NewSession(binding Binding, generator Generator, guard Guard) (*Session, error) {
	if binding.OwnerID == 0 || binding.HandleID == "" || generator == nil || guard == nil {
		return nil, ErrInvalid
	}
	switch binding.Protocol {
	case "ssh", "mysql", "mariadb", "sqlserver", "oracle", "clickhouse", "elasticsearch", "opensearch", "postgresql", "redis", "mongodb":
	default:
		return nil, ErrInvalid
	}
	return &Session{binding: binding, generator: generator, guard: guard}, nil
}

func (s *Session) Suggest(ctx context.Context, input Input) (Suggestion, error) {
	if input.Revision == 0 || len(input.Text) > 8192 || len(input.RecentOutput) > 16384 || len(input.Schema) > 32768 || len(input.ShellContext) > 16384 || !utf8.ValidString(input.ShellContext) ||
		input.Cursor < 0 || input.Cursor > len(input.Text) || !utf8.ValidString(input.Text) ||
		!utf8.ValidString(input.Text[:input.Cursor]) || !utf8.ValidString(input.RecentOutput) || !utf8.ValidString(input.Schema) {
		return Suggestion{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Suggestion{}, err
	}
	// Manual suggestions allow provider latency; typing still cancels immediately.
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Suggestion{}, ErrClosed
	}
	if input.Revision <= s.revision {
		s.mu.Unlock()
		return Suggestion{}, ErrStale
	}
	previous := s.cancel
	s.revision, s.cancel = input.Revision, cancel
	s.mu.Unlock()
	if previous != nil {
		previous()
	}
	defer func() {
		s.mu.Lock()
		if s.revision == input.Revision {
			s.cancel = nil
		}
		s.mu.Unlock()
	}()
	if err := s.guard.Check(requestCtx, s.binding); err != nil {
		return Suggestion{}, err
	}
	text, err := s.generator.Suggest(requestCtx, s.binding, input)
	if err != nil {
		return Suggestion{}, err
	}
	if err := requestCtx.Err(); err != nil {
		return Suggestion{}, err
	}
	if err := s.guard.Check(requestCtx, s.binding); err != nil {
		return Suggestion{}, err
	}
	// 候选禁止终端控制字符；SSH 同时禁止换行，接受候选绝不能提交命令。
	if len(text) > 8192 || !utf8.ValidString(text) {
		return Suggestion{}, ErrSuggestion
	}
	for _, r := range text {
		if unicode.IsControl(r) && !(s.binding.Protocol != "ssh" && (r == '\n' || r == '\t')) {
			return Suggestion{}, ErrSuggestion
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Suggestion{}, ErrClosed
	}
	if s.revision != input.Revision {
		return Suggestion{}, ErrStale
	}
	if err := requestCtx.Err(); err != nil {
		return Suggestion{}, err
	}
	return Suggestion{Revision: input.Revision, Cursor: input.Cursor, Text: text}, nil
}

// Invalidate 在继续输入或按 Esc 时取消旧候选，不触发新的模型调用。
// 下一次 Suggest 应使用更大的 revision。
func (s *Session) Invalidate(revision uint64) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	if revision <= s.revision {
		s.mu.Unlock()
		return ErrStale
	}
	s.revision = revision
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (s *Session) Close() {
	s.mu.Lock()
	s.closed = true
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
