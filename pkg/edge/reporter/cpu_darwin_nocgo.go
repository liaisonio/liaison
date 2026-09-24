//go:build darwin && !cgo

package reporter

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// The legacy gopsutil Darwin CPU sampler requires CGO. Release edges are
// pure Go, so use the system sampler with a bounded two-sample interval.
func hostCPUPercent(ctx context.Context) (float64, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/top", "-l", "2", "-s", "1", "-n", "0")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("sample system CPU: %w", err)
	}
	return parseTopCPU(string(output))
}
