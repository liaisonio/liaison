//go:build windows

package winservice

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sys/windows/svc"
)

func Run(name string, start Start) error {
	managed, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !managed {
		return errors.New("service mode must be started by Windows Service Control Manager")
	}
	return svc.Run(name, &handler{start: start})
}

type handler struct{ start Start }
type started struct {
	stop func() error
	err  error
}

func (h *handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status <- svc.Status{State: svc.StartPending, WaitHint: 30000}
	ready := make(chan started)
	go func() {
		stop, err := h.start(ctx)
		select {
		case ready <- started{stop, err}:
		case <-ctx.Done():
			if stop != nil {
				// The SCM entry point is exiting; initialization must not leak
				// a live tunnel if the startup timeout expired.
				if err := stop(); err != nil {
					return
				}
			}
		}
	}()
	var running started
	select {
	case running = <-ready:
		if running.err != nil || running.stop == nil {
			return true, 1
		}
	case <-time.After(30 * time.Second):
		return true, 2
	}
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	status <- current
	for request := range requests {
		switch request.Cmd {
		case svc.Interrogate:
			status <- current
		case svc.Stop, svc.Shutdown:
			status <- svc.Status{State: svc.StopPending, WaitHint: 30000}
			cancel()
			return stopWithin(running.stop, 30*time.Second)
		}
	}
	cancel()
	return stopWithin(running.stop, 30*time.Second)
}

func stopWithin(stop func() error, timeout time.Duration) (bool, uint32) {
	done := make(chan error, 1)
	go func() { done <- stop() }()
	select {
	case err := <-done:
		if err != nil {
			return true, 3
		}
		return false, 0
	case <-time.After(timeout):
		return true, 4
	}
}
