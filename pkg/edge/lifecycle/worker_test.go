package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func validCommand() proto.UninstallCommand {
	id := strings.Repeat("a", 32)
	return proto.UninstallCommand{RuntimeID: id, TaskID: id, InstanceID: id, CallbackToken: strings.Repeat("b", 64), ExpiresAt: time.Now().Add(time.Minute).Unix(), CallbackURL: "https://manager.test/api/v1/edge-uninstall-results/" + id}
}

func TestUninstallCommandBoundary(t *testing.T) {
	for _, name := range []string{"valid", "http", "wrong host", "credentials", "query", "wrong path", "expired", "too long", "token", "task traversal"} {
		t.Run(name, func(t *testing.T) {
			cmd := validCommand()
			switch name {
			case "http":
				cmd.CallbackURL = strings.Replace(cmd.CallbackURL, "https:", "http:", 1)
			case "wrong host":
				cmd.CallbackURL = strings.Replace(cmd.CallbackURL, "manager.test", "other.test", 1)
			case "credentials":
				cmd.CallbackURL = strings.Replace(cmd.CallbackURL, "https://", "https://user:pass@", 1)
			case "query":
				cmd.CallbackURL += "?secret=x"
			case "wrong path":
				cmd.CallbackURL += "/other"
			case "expired":
				cmd.ExpiresAt = time.Now().Add(-time.Second).Unix()
			case "too long":
				cmd.ExpiresAt = time.Now().Add(time.Hour).Unix()
			case "token":
				cmd.CallbackToken = strings.Repeat("z", 64)
			case "task traversal":
				cmd.TaskID = "../other"
			}
			err := validateCommand(cmd, []string{"manager.test:3001"}, time.Now())
			if name == "valid" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestApplyUninstallExactTargetsAndFailures(t *testing.T) {
	for _, kind := range []string{"systemd", "launchd"} {
		for _, failure := range []string{"", "preflight", "changed instance", "changed file", "stop", "still running", "file permission", "remove", "expired"} {
			t.Run(kind+"/"+failure, func(t *testing.T) {
				dir := t.TempDir()
				cmd := validCommand()
				plan := Plan{Kind: kind, InstanceID: cmd.InstanceID, Service: "liaison-edge-test", ServiceFile: filepath.Join(dir, "service"), Config: filepath.Join(dir, "config"), Executable: filepath.Join(dir, "edge")}
				job := workerJob{Identity: Identity{PID: 1234}, Plan: plan, Command: cmd, Digests: map[string]string{}}
				for _, p := range []string{plan.ServiceFile, plan.Config, plan.Executable} {
					require.NoError(t, os.WriteFile(p, []byte("original"), 0600))
					sum, err := digestFile(p)
					require.NoError(t, err)
					job.Digests[p] = sum
				}
				unrelated := filepath.Join(dir, "cloud-edge")
				require.NoError(t, os.WriteFile(unrelated, []byte("keep"), 0600))
				if failure == "changed file" {
					require.NoError(t, os.WriteFile(plan.Config, []byte("changed"), 0600))
				}
				if failure == "expired" {
					job.Command.ExpiresAt = 1
				}
				calls := []string{}
				removed := []string{}
				check := func(context.Context, Identity, Command) (Plan, error) {
					if failure == "preflight" {
						return Plan{}, ErrUnsupported
					}
					if failure == "changed instance" {
						p := plan
						p.InstanceID = "other"
						return p, nil
					}
					return plan, nil
				}
				run := func(_ context.Context, name string, args ...string) ([]byte, error) {
					calls = append(calls, name+" "+strings.Join(args, " "))
					if failure == "stop" {
						return nil, errors.New("stop failed")
					}
					if name == "/bin/ps" && failure == "still running" {
						return []byte(" 1234\n"), nil
					}
					return nil, nil
				}
				validate := func(string, int) error {
					if failure == "file permission" {
						return ErrUnsupported
					}
					return nil
				}
				remove := func(path string) error {
					removed = append(removed, path)
					if failure == "remove" {
						return errors.New("remove failed")
					}
					return os.Remove(path)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				err := apply(ctx, job, check, run, validate, remove)
				if failure == "" {
					require.NoError(t, err)
					require.Equal(t, []string{plan.ServiceFile, plan.Config, plan.Executable}, removed)
				} else {
					require.Error(t, err)
				}
				if failure == "preflight" || failure == "changed instance" || failure == "changed file" || failure == "expired" {
					require.Empty(t, calls)
				}
				if failure != "" && failure != "remove" {
					require.Empty(t, removed)
				}
				data, err := os.ReadFile(unrelated)
				require.NoError(t, err)
				require.Equal(t, "keep", string(data))
			})
		}
	}
}
