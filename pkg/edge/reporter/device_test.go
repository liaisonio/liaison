package reporter

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRepeatDeviceReportRetriesInitialFailureQuickly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts atomic.Int32
	done := make(chan struct{})
	go func() {
		repeatDeviceReport(ctx, time.Millisecond, time.Hour, func() error {
			if attempts.Add(1) == 1 {
				return errors.New("frontier registration is not ready")
			}
			close(done)
			return nil
		})
	}()

	select {
	case <-done:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("device report was not retried")
	}

	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestRepeatDeviceReportStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		repeatDeviceReport(ctx, time.Hour, time.Hour, func() error {
			return errors.New("not connected")
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("device reporter did not stop after context cancellation")
	}
}
