package reporter

import (
	"context"

	"github.com/liaisonio/liaison/pkg/edge/frontierbound"
)

type Reporter interface {
}

type reporter struct {
	frontierBound frontierbound.FrontierBound
}

func NewReporter(frontierBound frontierbound.FrontierBound) (Reporter, error) {
	reporter := &reporter{
		frontierBound: frontierBound,
	}
	go reporter.loopReportDevice(context.Background())
	go reporter.loopReportDeviceUsage(context.Background())
	return reporter, nil
}

func (r *reporter) Close() error {
	return r.frontierBound.Close()
}
