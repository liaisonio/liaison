//go:build darwin && !cgo

package reporter

import (
	"context"
	"testing"
)

func TestHostCPUPercentCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hostCPUPercent(ctx); err == nil {
		t.Fatal("cancelled sampling succeeded")
	}
}
