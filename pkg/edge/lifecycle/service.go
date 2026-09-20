package lifecycle

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func systemdArgs(plan Plan, args ...string) []string {
	if plan.Kind == "systemd-user" {
		return append([]string{"--user"}, args...)
	}
	return args
}

// Native stop operations can return before the process fully exits (notably
// launchd). Never infer exit from a successful stop request alone.
func waitProcessGone(ctx context.Context, pid int, run Command) error {
	if pid <= 0 {
		return ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		data, err := run(ctx, "/bin/ps", "-axo", "pid=")
		if err != nil {
			return err
		}
		present := false
		for _, candidate := range strings.Fields(string(data)) {
			if candidate == strconv.Itoa(pid) {
				present = true
				break
			}
		}
		if !present {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("original process has not stopped: %w", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func waitExecutableGone(ctx context.Context, executable string, run Command) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		data, err := run(ctx, "/bin/ps", "-axo", "pid=,comm=")
		if err != nil {
			return err
		}
		present := false
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			split := strings.IndexAny(line, " \t")
			if split >= 0 && strings.TrimSpace(line[split:]) == executable {
				present = true
				break
			}
		}
		if !present {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("installation executable is still running: %w", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func servicePID(ctx context.Context, id Identity, plan Plan, run Command) (int, error) {
	var data []byte
	var err error
	if plan.Kind == "launchd" {
		data, err = run(ctx, "/bin/launchctl", "print", fmt.Sprintf("gui/%d/%s", id.UID, plan.Service))
		if err != nil {
			return 0, err
		}
		matches := regexp.MustCompile(`(?m)^\s*pid = ([0-9]+)\s*$`).FindAllSubmatch(data, -1)
		if len(matches) != 1 {
			return 0, ErrUnsupported
		}
		return strconv.Atoi(string(matches[0][1]))
	}
	data, err = run(ctx, "/usr/bin/systemctl", systemdArgs(plan, "show", plan.Service, "--property=MainPID", "--value")...)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}
