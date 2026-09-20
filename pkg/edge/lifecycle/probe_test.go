package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeSystemdReadOnly(t *testing.T) {
	id, _ := linuxIdentity()
	for _, bad := range []string{"", "pid", "dropin", "wrapper", "exec", "timeout"} {
		t.Run(bad, func(t *testing.T) {
			run := func(_ context.Context, command string, args ...string) ([]byte, error) {
				require.Equal(t, "/usr/bin/systemctl", command)
				require.Equal(t, "show", args[0])
				require.Equal(t, "liaison-edge.service", args[1])
				if bad == "timeout" {
					return nil, context.DeadlineExceeded
				}
				pid := "123"
				drop := ""
				exe := id.Executable
				prefix := ""
				if bad == "pid" {
					pid = "999"
				}
				if bad == "dropin" {
					drop = "override.conf"
				}
				if bad == "exec" {
					exe = "/tmp/another-edge"
				}
				if bad == "wrapper" {
					prefix = "/bin/sh "
				}
				return []byte("MainPID=" + pid + "\nFragmentPath=/etc/systemd/system/liaison-edge.service\nDropInPaths=" + drop + "\nExecStart={ path=" + exe + " ; argv[]=" + prefix + exe + " -c " + id.Config + " ; ignore_errors=no ; }\n"), nil
			}
			_, err := Probe(context.Background(), id, run)
			if bad == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestProbeLaunchAgentReadOnly(t *testing.T) {
	id := Identity{OS: "darwin", UID: 501, PID: 321, Home: "/Users/example", Executable: "/Users/example/Library/Application Support/liaison/bin/liaison-edge", Config: "/Users/example/Library/Application Support/liaison/liaison-edge.yaml"}
	for _, bad := range []string{"", "pid", "label", "config", "multiple", "error"} {
		t.Run(bad, func(t *testing.T) {
			calls := 0
			run := func(_ context.Context, command string, args ...string) ([]byte, error) {
				calls++
				if bad == "error" {
					return nil, errors.New("denied")
				}
				switch command {
				case "/usr/bin/plutil":
					require.Equal(t, []string{"-convert", "json", "-o", "-", id.Home + "/Library/LaunchAgents/com.liaison.edge.plist"}, args)
					label := "com.liaison.edge"
					config := id.Config
					if bad == "label" {
						label = "com.ongrid.edge"
					}
					if bad == "config" {
						config = "/tmp/shared.yaml"
					}
					return json.Marshal(map[string]any{"Label": label, "ProgramArguments": []string{id.Executable, "-c", config}})
				case "/bin/launchctl":
					require.Equal(t, []string{"print", "gui/501/com.liaison.edge"}, args)
					pid := "321"
					if bad == "pid" {
						pid = "999"
					}
					out := "\tpid = " + pid + "\n"
					if bad == "multiple" {
						out += out
					}
					return []byte(out), nil
				default:
					t.Fatalf("unexpected command: %s %s", command, strings.Join(args, " "))
					return nil, errors.New("unexpected")
				}
			}
			_, err := Probe(context.Background(), id, run)
			if bad == "" {
				require.NoError(t, err)
				require.Equal(t, 2, calls)
			} else {
				require.Error(t, err)
			}
		})
	}
}
