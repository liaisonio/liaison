//go:build linux && integration

package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNativeUserLifecycle(t *testing.T) {
	if os.Getenv("LIAISON_LIFECYCLE_NATIVE_TEST") != "1" {
		t.Skip("explicit disposable user systemd fixture opt-in required")
	}
	require.Positive(t, os.Geteuid())
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	self, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	create := func() (Identity, Plan) {
		raw := make([]byte, 16)
		_, err := rand.Read(raw)
		require.NoError(t, err)
		id := Identity{OS: "linux", UID: os.Geteuid(), Home: home}
		plan, err := InstallNew(ctx, id, self, InstallCredentials{"fixture.invalid:443", hex.EncodeToString(raw), "fixture-not-a-real-credential"})
		require.NoError(t, err)
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			if _, statErr := os.Stat(plan.ServiceFile); statErr == nil {
				if err := newInstaller().stop(cleanup, id, plan, true); err != nil {
					t.Errorf("stop fixture: %v", err)
					return
				}
			} else if !os.IsNotExist(statErr) {
				t.Error(statErr)
				return
			}
			base := installationBase(plan)
			for _, path := range []string{plan.ServiceFile, plan.Config, plan.Executable, manifestPath(plan), plan.Executable + ".next", plan.Executable + ".previous", filepath.Join(base, "bin"), filepath.Join(base, "logs"), base} {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					t.Errorf("fixture cleanup %s: %v", path, err)
				}
			}
			if _, err := NativeCommand(cleanup, "/usr/bin/systemctl", "--user", "daemon-reload"); err != nil {
				t.Error(err)
			}
		})
		id.InstanceID, id.Executable, id.Config = plan.InstanceID, plan.Executable, plan.Config
		id.PID, err = servicePID(ctx, id, plan, NativeCommand)
		require.NoError(t, err)
		return id, plan
	}
	target, p := create()
	other, otherPlan := create()
	require.Equal(t, "systemd-user", p.Kind)
	_, err = Check(ctx, target, NativeCommand)
	if err != nil {
		for _, pid := range []int{target.PID, other.PID} {
			exe, e := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
			t.Logf("fixture executable %d: %s (%v)", pid, exe, e)
		}
		manager, inspectErr := NativeCommand(ctx, "/usr/bin/systemctl", "show", "user@"+strconv.Itoa(os.Geteuid())+".service", "--property=MainPID,ExecStart,ControlGroup")
		t.Logf("user manager identity: %s (%v)", manager, inspectErr)
		data, inspectErr := NativeCommand(ctx, "/bin/ps", "-u", strconv.Itoa(os.Geteuid()), "-o", "pid=,comm=")
		t.Logf("user process names: %s (inspection: %v)", data, inspectErr)
	}
	require.NoError(t, err)
	alias := filepath.Join(home, ".config/systemd/user", p.Service+".fixture.service")
	require.NoError(t, exclusiveFile(alias, []byte(p.Executable), 0600))
	_, sharedErr := Check(ctx, target, NativeCommand)
	require.NoError(t, os.Remove(alias))
	require.Error(t, sharedErr)
	configSum, err := digestFile(p.Config)
	require.NoError(t, err)
	require.NoError(t, UpgradeInstallation(ctx, target, self))
	require.Error(t, UpgradeInstallation(ctx, target, "/usr/bin/false"))
	got, err := digestFile(p.Config)
	require.NoError(t, err)
	require.Equal(t, configSum, got)
	target.PID, err = servicePID(ctx, target, p, NativeCommand)
	require.NoError(t, err)
	statuses := make(chan string, 8)
	command := validCommand()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+command.CallbackToken {
			http.Error(w, "unauthorized", 401)
			return
		}
		var body struct{ Status string }
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			http.Error(w, "invalid", 400)
			return
		}
		statuses <- body.Status
		w.WriteHeader(200)
	}))
	defer server.Close()
	command.InstanceID = target.InstanceID
	command.CallbackURL = server.URL + "/api/v1/edge-uninstall-results/" + command.TaskID
	require.NoError(t, Prepare(ctx, target, command, []string{strings.TrimPrefix(server.URL, "https://")}, true, NativeCommand))
	for _, want := range []string{"running", "completed"} {
		select {
		case got := <-statuses:
			require.Equal(t, want, got)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for _, path := range []string{p.Executable, p.Config, p.ServiceFile} {
		_, err := os.Stat(path)
		require.True(t, os.IsNotExist(err), "target remains: %s", path)
	}
	pid, err := servicePID(ctx, other, otherPlan, NativeCommand)
	require.NoError(t, err)
	require.Equal(t, other.PID, pid)
	_, err = Check(ctx, other, NativeCommand)
	require.NoError(t, err)
	t.Log("user-level installation, upgrade, rollback and self-uninstall passed; second instance unchanged")
}
