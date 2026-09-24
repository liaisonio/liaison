package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
)

type fakeSession struct {
	done chan struct{}
	once sync.Once
}

func (s *fakeSession) CheckAuthentication(context.Context) (bool, error)    { return true, nil }
func (s *fakeSession) NewThread(context.Context) (Thread, error)            { return Thread{ID: "thread"}, nil }
func (s *fakeSession) Send(context.Context, string, string) (string, error) { return "turn", nil }
func (s *fakeSession) Interrupt(context.Context, string, string) error      { return nil }
func (s *fakeSession) RejectRequest(context.Context, rpc.Message) error     { return nil }
func (s *fakeSession) Events() <-chan rpc.Message                           { return nil }
func (s *fakeSession) Done() <-chan struct{}                                { return s.done }
func (s *fakeSession) Close() error                                         { s.once.Do(func() { close(s.done) }); return nil }

type fakeAdapter struct {
	launch func(context.Context) (Session, error)
}

func (fakeAdapter) Kind() string { return "fake" }
func (a fakeAdapter) Launch(ctx context.Context, _ discovery.Installation, _ string) (Session, error) {
	return a.launch(ctx)
}

func TestIsolationAndLimits(t *testing.T) {
	r, err := New(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	s := &fakeSession{done: make(chan struct{})}
	a := fakeAdapter{launch: func(context.Context) (Session, error) { return s, nil }}
	scope := Scope{"alice", "access-a", "/project"}
	id, err := r.Open(context.Background(), scope, a, discovery.Installation{Agent: "fake"})
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []Scope{{"bob", "access-a", "/project"}, {"alice", "access-b", "/project"}, {"alice", "access-a", "/other"}} {
		if _, err := r.Lookup(other, id); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("scope escaped: %v", err)
		}
	}
	if got, err := r.Lookup(scope, id); err != nil || got != s {
		t.Fatal("own scope unavailable")
	}
	if _, err := r.Open(context.Background(), scope, a, discovery.Installation{Agent: "fake"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("limit bypassed")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("session not closed")
	}
	if _, err := r.Lookup(scope, id); !errors.Is(err, ErrUnavailable) {
		t.Fatal("closed registry accessible")
	}
}

func TestCloseDuringLaunch(t *testing.T) {
	r, _ := New(context.Background(), 1)
	started := make(chan struct{})
	a := fakeAdapter{launch: func(ctx context.Context) (Session, error) { close(started); <-ctx.Done(); return nil, ctx.Err() }}
	result := make(chan error, 1)
	go func() {
		_, err := r.Open(context.Background(), Scope{"a", "b", "/p"}, a, discovery.Installation{Agent: "fake"})
		result <- err
	}()
	<-started
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err == nil {
		t.Fatal("launch survived shutdown")
	}
}

func TestFailedLaunchReleasesReservation(t *testing.T) {
	r, _ := New(context.Background(), 1)
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	a := fakeAdapter{launch: func(context.Context) (Session, error) { return nil, errors.New("unavailable") }}
	for i := 0; i < 2; i++ {
		if _, err := r.Open(context.Background(), Scope{"a", "b", "/p"}, a, discovery.Installation{Agent: "fake"}); err == nil || errors.Is(err, ErrUnavailable) {
			t.Fatal("reservation leaked")
		}
	}
}
