package claude

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
)

// Session adapts Claude's native stream to the shared runtime. Native default
// permissions decide which calls need the interaction bridge's approval.
type Session struct {
	ctx          context.Context
	cancel       context.CancelFunc
	installation discovery.Installation
	project      string
	mu           sync.Mutex
	driver       *Driver
	opening      chan struct{}
	thread, turn string
	normalizer   *Updates
	interactions map[string]string
	ready        chan struct{}
	done         chan struct{}
	updates      chan agentruntime.Update
}

var _ agentruntime.Adapter = Adapter{}
var _ agentruntime.UpdateSession = (*Session)(nil)
var _ agentruntime.ResumableSession = (*Session)(nil)

func (Adapter) Launch(ctx context.Context, i discovery.Installation, project string) (agentruntime.Session, error) {
	if _, err := launchSpec(i, project); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	life, cancel := context.WithCancel(ctx)
	s := &Session{ctx: life, cancel: cancel, installation: i, project: project, ready: make(chan struct{}), done: make(chan struct{}), updates: make(chan agentruntime.Update, 64)}
	go s.run()
	return s, nil
}

func (s *Session) NewThread(ctx context.Context) (agentruntime.Thread, error) {
	id, err := NewSessionID()
	if err != nil {
		return agentruntime.Thread{}, err
	}
	return s.open(ctx, id, false)
}

func (s *Session) ResumeThread(ctx context.Context, id string) (agentruntime.Thread, error) {
	return s.open(ctx, id, true)
}

func (s *Session) open(ctx context.Context, id string, resume bool) (agentruntime.Thread, error) {
	s.mu.Lock()
	if s.opening != nil || s.driver != nil || s.ctx.Err() != nil || ctx.Err() != nil {
		s.mu.Unlock()
		return agentruntime.Thread{}, agentruntime.ErrUnavailable
	}
	s.opening = make(chan struct{})
	opening := s.opening
	s.mu.Unlock()
	defer close(opening)
	// A cancelled launch is terminal; never reuse an uncertain native process.
	stop := context.AfterFunc(ctx, s.cancel)
	d, err := startPersistent(s.ctx, s.installation, s.project, id, resume, true)
	stopped := stop()
	if err != nil {
		s.cancel()
		return agentruntime.Thread{}, err
	}
	s.mu.Lock()
	s.driver = d
	s.thread = id
	s.mu.Unlock()
	close(s.ready)
	if !stopped || ctx.Err() != nil || s.ctx.Err() != nil {
		s.cancel()
		return agentruntime.Thread{}, agentruntime.ErrUnavailable
	}
	return agentruntime.Thread{ID: id}, nil
}

func (s *Session) Send(ctx context.Context, thread, prompt string) (string, error) {
	return s.sendWithOptions(ctx, thread, prompt, "", "")
}

func (s *Session) sendWithOptions(ctx context.Context, thread, prompt, skill, model string) (string, error) {
	s.mu.Lock()
	if s.driver == nil || s.ctx.Err() != nil || thread != s.thread || s.turn != "" || prompt == "" {
		s.mu.Unlock()
		return "", agentruntime.ErrUnavailable
	}
	turn, err := NewSessionID()
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	s.turn, s.normalizer = turn, NewUpdates()
	d := s.driver
	s.mu.Unlock()
	if skill != "" {
		found := false
		for _, item := range d.skills() {
			if item.ID == skill {
				prompt = item.Name + " " + prompt
				found = true
				break
			}
		}
		if !found {
			s.cancel()
			return "", agentruntime.ErrUnavailable
		}
	}
	if model != "" {
		if !catalogIdentifier(model) {
			s.cancel()
			return "", agentruntime.ErrUnavailable
		}
		if _, err := d.control(ctx, struct {
			Subtype string `json:"subtype"`
			Model   string `json:"model"`
		}{"set_model", model}); err != nil {
			s.cancel()
			return "", err
		}
	}
	if err := d.Send(ctx, prompt); err != nil {
		s.cancel()
		return "", err
	}
	return turn, nil
}

func (s *Session) Interrupt(ctx context.Context, thread, turn string) error {
	s.mu.Lock()
	if s.driver == nil || thread != s.thread || turn == "" || turn != s.turn {
		s.mu.Unlock()
		return agentruntime.ErrUnavailable
	}
	d := s.driver
	s.mu.Unlock()
	return d.Interrupt(ctx)
}

func (s *Session) Updates() <-chan agentruntime.Update { return s.updates }
func (s *Session) Done() <-chan struct{}               { return s.done }
func (s *Session) Close() error                        { s.cancel(); <-s.done; return nil }

func (s *Session) run() {
	defer close(s.done)
	defer close(s.updates)
	defer func() {
		// A protocol panic is terminal and must reap the owned child as well.
		_ = recover()
		s.cancel()
		s.mu.Lock()
		opening := s.opening
		s.mu.Unlock()
		if opening != nil {
			<-opening
		}
		s.mu.Lock()
		d := s.driver
		s.mu.Unlock()
		if d != nil {
			if err := d.Close(); err != nil { /* transport already terminal */
			}
		}
	}()
	select {
	case <-s.ctx.Done():
		return
	case <-s.ready:
	}
	s.mu.Lock()
	d := s.driver
	s.mu.Unlock()
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-d.Events():
			if !ok {
				return
			}
			if event.Type == "control_request" || event.Type == "control_cancel_request" {
				if !s.handleInteraction(event) {
					return
				}
				continue
			}
			if !s.consume(event) {
				return
			}
		}
	}
}

func (s *Session) consume(event Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.turn == "" || s.normalizer == nil {
		return true
	}
	if event.Type == "system" {
		var meta struct {
			Subtype string `json:"subtype"`
			Model   string `json:"model"`
			Version string `json:"claude_code_version"`
		}
		if json.Unmarshal(event.Raw, &meta) == nil && meta.Subtype == "init" && len(meta.Model) <= 256 && len(meta.Version) <= 64 {
			select {
			case s.updates <- agentruntime.Update{ThreadID: s.thread, TurnID: s.turn, Kind: agentruntime.SessionMetadata, Text: meta.Model, Status: meta.Version}:
			default:
				return false
			}
		}
		return true
	}
	updates, err := s.normalizer.Apply(event)
	if err != nil {
		return false
	}
	for _, u := range updates {
		u.ThreadID, u.TurnID = s.thread, s.turn
		select {
		case s.updates <- u:
		default:
			return false
		}
		if u.Kind == agentruntime.TurnEnded {
			s.turn = ""
			s.interactions = nil
		}
	}
	return true
}
