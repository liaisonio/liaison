//go:build !darwin || cgo

package reporter

import (
	"context"
	"fmt"
	"github.com/shirou/gopsutil/cpu"
)

func hostCPUPercent(ctx context.Context) (float64, error) {
	values, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, fmt.Errorf("empty CPU sample")
	}
	return values[0], nil
}
