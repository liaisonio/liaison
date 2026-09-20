package lifecycle

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUserManagerExceptionRequiresNativeIdentity(t *testing.T) {
	metadata := "MainPID=123\nControlGroup=/user.slice/user-1001.slice/user@1001.service\nExecStart={ path=/usr/lib/systemd/systemd ; argv[]=/usr/lib/systemd/systemd --user ; }\n"
	scope := "0::/user.slice/user-1001.slice/user@1001.service/init.scope\n"
	for _, tc := range []struct {
		name, pid, metadata, scope, parent string
		rootValid, want                    bool
	}{
		{"manager", "123", metadata, scope, "", true, true},
		{"pam_helper", "124", metadata, scope, "PPid:\t123\n", true, true},
		{"unrelated_parent", "124", metadata, scope, "PPid:\t999\n", true, false},
		{"edge_scope", "123", metadata, strings.ReplaceAll(scope, "init.scope", "app.slice/edge.service"), "", true, false},
		{"other_user", "123", strings.ReplaceAll(metadata, "1001", "1002"), scope, "", true, false},
		{"wrong_program", "123", strings.ReplaceAll(metadata, "/usr/lib/systemd/systemd", "/tmp/systemd"), scope, "", true, false},
		{"unsafe_program_owner", "123", metadata, scope, "", false, false},
		{"extra_argument", "123", strings.ReplaceAll(metadata, "--user ;", "--user --extra ;"), scope, "", true, false},
		{"invalid_pid", "../123", metadata, scope, "", true, false},
		{"missing_metadata", "123", "", scope, "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(_ context.Context, name string, args ...string) ([]byte, error) {
				if name != "/usr/bin/systemctl" || !strings.Contains(strings.Join(args, " "), "user@1001.service") {
					t.Fatal("unexpected service query")
				}
				return []byte(tc.metadata), nil
			}
			read := func(path string) ([]byte, error) {
				if strings.HasSuffix(path, "/cgroup") {
					return []byte(tc.scope), nil
				}
				return []byte(tc.parent), nil
			}
			validate := func(path string, uid int) error {
				if uid != 0 || path != "/usr/lib/systemd/systemd" {
					t.Fatal("not validating root-owned systemd")
				}
				if !tc.rootValid {
					return errors.New("unsafe")
				}
				return nil
			}
			got := verifiedUserManager(context.Background(), Identity{OS: "linux", UID: 1001}, tc.pid, run, read, validate)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
