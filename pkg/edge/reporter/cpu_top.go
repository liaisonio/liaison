package reporter

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

var topCPUIdle = regexp.MustCompile(`(?m)^CPU usage:.*?([0-9]+(?:\.[0-9]+)?)% idle`)

func parseTopCPU(output string) (float64, error) {
	matches := topCPUIdle.FindAllStringSubmatch(output, -1)
	// The first sample is an average since boot, not current activity.
	if len(matches) < 2 {
		return 0, fmt.Errorf("missing second CPU sample")
	}
	idle, err := strconv.ParseFloat(matches[len(matches)-1][1], 64)
	if err != nil || math.IsNaN(idle) || idle < 0 || idle > 100 {
		return 0, fmt.Errorf("invalid CPU idle sample")
	}
	return 100 - idle, nil
}
