// Package runtime defines the Agent-neutral lifetime contract. The caller must
// authorize scope before opening a session; this is not a network auth layer.
package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
)

var ErrUnavailable = errors.New("agent session unavailable")

type Thread struct {
	Version string `json:"cliVersion"`
	ID      string `json:"id"`
	Model   string `json:"-"`
	Name    string `json:"name"`
}
type Model struct{ ID, Name string }

// Optional native catalog and per-turn model selection. Never writes local configuration.
type ModelSession interface {
	Models(context.Context) ([]Model, error)
	SendModel(context.Context, string, string, string, string) (string, error)
}

// ResumableSession reopens a native thread that was previously created by this
// integration. Callers must enforce ownership before passing a thread ID.
type ResumableSession interface {
	ResumeThread(context.Context, string) (Thread, error)
}

// Skills are an optional adapter capability. Paths never cross the Edge boundary.
type Skill struct{ ID, Name, Description string }
type SkillSession interface {
	Skills(context.Context) ([]Skill, error)
	SendSkill(context.Context, string, string, string) (string, error)
}

type Session interface {
	CheckAuthentication(context.Context) (bool, error)
	NewThread(context.Context) (Thread, error)
	Send(context.Context, string, string) (string, error)
	Interrupt(context.Context, string, string) error
	RejectRequest(context.Context, rpc.Message) error
	Events() <-chan rpc.Message
	Done() <-chan struct{}
	Close() error
}

// PermissionSession never accepts arbitrary JSON-RPC from remote clients.
type PermissionSession interface {
	SetPermissionMode(string) error
	ApproveCommand(context.Context, rpc.Message) error
}

type InputSession interface {
	AnswerInput(context.Context, rpc.Message, map[string][]string) error
}

type Adapter interface {
	Kind() string
	Launch(context.Context, discovery.Installation, string) (Session, error)
}

// Scope is supplied by a trusted control plane, never copied from a browser.
// Project should already be canonicalized and authorized by that control plane.
type Scope struct{ Owner, Access, Project string }
type entry struct {
	scope   Scope
	session Session
}
type Registry struct {
	mu      sync.Mutex
	entries map[string]entry
	limit   int
	closed  bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func New(ctx context.Context, limit int) (*Registry, error) {
	if limit < 1 || limit > 16 {
		return nil, errors.New("agent instance limit must be between 1 and 16")
	}
	life, cancel := context.WithCancel(ctx)
	return &Registry{entries: make(map[string]entry), limit: limit, ctx: life, cancel: cancel}, nil
}

func (r *Registry) Open(ctx context.Context, scope Scope, adapter Adapter, installation discovery.Installation) (string, error) {
	if scope.Owner == "" || scope.Access == "" || scope.Project == "" || adapter == nil || adapter.Kind() != installation.Agent {
		return "", errors.New("invalid agent session scope")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(random[:])
	r.mu.Lock()
	if r.closed || r.ctx.Err() != nil || len(r.entries) >= r.limit {
		r.mu.Unlock()
		return "", ErrUnavailable
	}
	// Reserve before launching, including in-flight handshakes in the limit.
	r.entries[id] = entry{scope: scope}
	r.wg.Add(1)
	r.mu.Unlock()
	defer r.wg.Done()
	life, cancel := context.WithCancel(r.ctx)
	stop := context.AfterFunc(ctx, cancel)
	session, err := adapter.Launch(life, installation, scope.Project)
	if err == nil && session == nil {
		err = errors.New("agent adapter returned no session")
	}
	stopped := stop()
	r.mu.Lock()
	if err != nil || !stopped || ctx.Err() != nil || r.closed || r.ctx.Err() != nil {
		delete(r.entries, id)
		r.mu.Unlock()
		cancel()
		if session != nil {
			err = errors.Join(err, session.Close())
		}
		if err == nil {
			err = ErrUnavailable
		}
		return "", err
	}
	r.entries[id] = entry{scope: scope, session: session}
	r.mu.Unlock()
	// Registry shutdown cancels life; after a natural exit release its slot.
	go func() {
		defer cancel()
		defer func() {
			if recover() != nil {
				cancel()
			}
		}()
		<-session.Done()
		r.mu.Lock()
		delete(r.entries, id)
		r.mu.Unlock()
	}()
	return id, nil
}

func (r *Registry) Lookup(scope Scope, id string) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok || r.closed || r.ctx.Err() != nil || e.scope != scope || e.session == nil {
		return nil, ErrUnavailable
	}
	select {
	case <-e.session.Done():
		return nil, ErrUnavailable
	default:
	}
	return e.session, nil
}

func (r *Registry) Close() error {
	r.mu.Lock()
	r.closed = true
	r.cancel()
	r.mu.Unlock()
	r.wg.Wait()
	r.mu.Lock()
	sessions := make([]Session, 0, len(r.entries))
	for _, e := range r.entries {
		if e.session != nil {
			sessions = append(sessions, e.session)
		}
	}
	r.entries = make(map[string]entry)
	r.mu.Unlock()
	var errs []error
	for _, session := range sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
