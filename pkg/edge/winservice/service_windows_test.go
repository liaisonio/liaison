//go:build windows

package winservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

func TestHandlerStartsInterrogatesAndStops(t *testing.T) {
	stopped := false
	h := handler{start: func(context.Context) (func() error, error) {
		return func() error { stopped = true; return nil }, nil
	}}
	requests := make(chan svc.ChangeRequest, 2)
	statuses := make(chan svc.Status, 10)
	done := make(chan uint32, 1)
	go func() { _, code := h.Execute(nil, requests, statuses); done <- code }()
	wait := func(want svc.State) {
		t.Helper()
		select {
		case got := <-statuses:
			if got.State != want {
				t.Fatalf("state %v, want %v", got.State, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("state timeout")
		}
	}
	wait(svc.StartPending)
	wait(svc.Running)
	requests <- svc.ChangeRequest{Cmd: svc.Interrogate}
	wait(svc.Running)
	requests <- svc.ChangeRequest{Cmd: svc.Stop}
	wait(svc.StopPending)
	select {
	case code := <-done:
		if code != 0 || !stopped {
			t.Fatal("stop not acknowledged")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stop timeout")
	}
}

func TestHandlerDoesNotReportRunningAfterStartupFailure(t *testing.T) {
	h := handler{start: func(context.Context) (func() error, error) { return nil, errors.New("fixture") }}
	statuses := make(chan svc.Status, 5)
	specific, code := h.Execute(nil, make(chan svc.ChangeRequest), statuses)
	if !specific || code == 0 {
		t.Fatal("startup failure lost")
	}
	close(statuses)
	for status := range statuses {
		if status.State == svc.Running {
			t.Fatal("reported running")
		}
	}
}

func TestStopWithin(t *testing.T) {
	specific, code := stopWithin(func() error { return errors.New("fixture") }, time.Second)
	if !specific || code != 3 {
		t.Fatal("stop failure lost")
	}
	release := make(chan struct{})
	defer close(release)
	specific, code = stopWithin(func() error { <-release; return nil }, time.Millisecond)
	if !specific || code != 4 {
		t.Fatal("stop timeout lost")
	}
}
