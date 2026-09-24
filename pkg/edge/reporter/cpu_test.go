package reporter

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestParseTopCPU(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       float64
		fail       bool
	}{
		{"latest", "CPU usage: 10% user, 10% sys, 80% idle\nCPU usage: 2.0% user, 1.5% sys, 96.5% idle", 3.5, false},
		{"zero", "CPU usage: 0% user, 0% sys, 100% idle\nCPU usage: 0% user, 0% sys, 100% idle", 0, false},
		{"busy", "CPU usage: 0% user, 0% sys, 100% idle\nCPU usage: 90% user, 10% sys, 0% idle", 100, false},
		{"missing", "", 0, true},
		{"one sample", "CPU usage: 10% user, 10% sys, 80% idle", 0, true},
		{"invalid", "CPU usage: 80% idle\nCPU usage: 101% idle", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTopCPU(tc.text)
			if (err != nil) != tc.fail || (!tc.fail && math.Abs(got-tc.want) > 0.001) {
				t.Fatalf("got %v, %v; want %v, fail=%v", got, err, tc.want, tc.fail)
			}
		})
	}
}

func TestHostCPUPercent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got, err := hostCPUPercent(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if math.IsNaN(got) || got < 0 || got > 100 {
		t.Fatalf("invalid CPU percentage: %v", got)
	}
}
