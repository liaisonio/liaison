//go:build linux && integration

package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Only the explicitly tagged native fixture binary has these service modes.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "--edge-uninstall-worker" {
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
		defer cancel()
		if err := RunWorker(ctx, os.Args[2]); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "-c" {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		<-signals
		return
	}
	os.Exit(m.Run())
}

func TestNativeIsolatedUninstall(t *testing.T) {
	if os.Getenv("LIAISON_LIFECYCLE_NATIVE_TEST") != "1" {
		t.Skip("explicit opt-in required: creates two disposable systemd fixtures")
	}
	require.Equal(t, 0, os.Geteuid())
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	create := func() Identity {
		raw := make([]byte, 16)
		_, err := rand.Read(raw)
		require.NoError(t, err)
		instance := hex.EncodeToString(raw)
		base := "/opt/liaison/edges/" + instance
		confBase := "/etc/liaison/edges/" + instance
		service := "liaison-edge-" + instance + ".service"
		file := "/etc/systemd/system/" + service
		// These are newly generated, exact paths; never reuse or delete a parent.
		_, err = os.Lstat(base)
		require.True(t, os.IsNotExist(err))
		_, err = os.Lstat(confBase)
		require.True(t, os.IsNotExist(err))
		_, err = os.Lstat(file)
		require.True(t, os.IsNotExist(err))
		require.NoError(t, os.MkdirAll(filepath.Join(base, "bin"), 0700))
		require.NoError(t, os.MkdirAll(confBase, 0700))
		id := Identity{OS: "linux", UID: 0, Executable: filepath.Join(base, "bin/liaison-edge"), Config: filepath.Join(confBase, "config.yaml"), InstanceID: instance}
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			_, _ = NativeCommand(cleanupCtx, "/usr/bin/systemctl", "stop", service)
			for _, p := range []string{file, id.Executable, id.Config, filepath.Dir(id.Executable), base, confBase} {
				if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
					t.Errorf("fixture cleanup %s: %v", p, err)
				}
			}
			_, _ = NativeCommand(cleanupCtx, "/usr/bin/systemctl", "daemon-reload")
		})
		self, err := os.Executable()
		require.NoError(t, err)
		src, err := os.Open(self)
		require.NoError(t, err)
		dst, err := os.OpenFile(id.Executable, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
		require.NoError(t, err)
		_, err = io.Copy(dst, src)
		require.NoError(t, err)
		require.NoError(t, src.Close())
		require.NoError(t, dst.Close())
		require.NoError(t, os.WriteFile(id.Config, []byte("native-test-fixture\n"), 0600))
		unit := "[Unit]\nDescription=Liaison disposable lifecycle test\n[Service]\nType=simple\nExecStart=" + id.Executable + " -c " + id.Config + "\nRestart=always\n[Install]\nWantedBy=multi-user.target\n"
		require.NoError(t, os.WriteFile(file, []byte(unit), 0600))
		_, err = NativeCommand(ctx, "/usr/bin/systemctl", "daemon-reload")
		require.NoError(t, err)
		_, err = NativeCommand(ctx, "/usr/bin/systemctl", "start", service)
		require.NoError(t, err)
		out, err := NativeCommand(ctx, "/usr/bin/systemctl", "show", service, "--property=MainPID", "--value")
		require.NoError(t, err)
		id.PID, err = strconv.Atoi(strings.TrimSpace(string(out)))
		require.NoError(t, err)
		require.Positive(t, id.PID)
		return id
	}
	target, other := create(), create()
	statuses := make(chan string, 8)
	command := validCommand()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+command.CallbackToken {
			http.Error(w, "unauthorized", 401)
			return
		}
		var body struct {
			Status string `json:"status"`
		}
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
	for _, expected := range []string{"running", "completed"} {
		select {
		case status := <-statuses:
			require.Equal(t, expected, status)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for _, p := range []string{target.Executable, target.Config, "/etc/systemd/system/liaison-edge-" + target.InstanceID + ".service"} {
		_, err := os.Stat(p)
		require.True(t, os.IsNotExist(err), "target remains: %s", p)
	}
	// The second instance must retain both its identity and live process.
	plan, err := Check(ctx, other, NativeCommand)
	require.NoError(t, err)
	require.Equal(t, other.InstanceID, plan.InstanceID)
	t.Log("target removed; second instance and service untouched")
}

func TestNativeInstallAndUpgrade(t *testing.T) {
	if os.Getenv("LIAISON_LIFECYCLE_NATIVE_TEST") != "1" {
		t.Skip("explicit disposable systemd fixture opt-in required")
	}
	require.Equal(t, 0, os.Geteuid())
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	self, err := os.Executable()
	require.NoError(t, err)
	create := func() (Identity, Plan) {
		raw := make([]byte, 16)
		_, err := rand.Read(raw)
		require.NoError(t, err)
		id := Identity{OS: "linux", UID: 0, Home: "/root"}
		plan, err := InstallNew(ctx, id, self, InstallCredentials{Manager: "fixture.invalid:443", AccessKey: hex.EncodeToString(raw), SecretKey: "fixture-not-a-real-credential"})
		require.NoError(t, err)
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			_, err := NativeCommand(cleanup, "/usr/bin/systemctl", "disable", "--now", plan.Service)
			if err != nil {
				t.Errorf("stop fixture: %v", err)
				return
			}
			base := installationBase(plan)
			for _, path := range []string{plan.ServiceFile, plan.Config, plan.Executable, manifestPath(plan), plan.Executable + ".next", plan.Executable + ".previous", filepath.Join(base, "bin/.lifecycle.lock"), filepath.Join(base, "bin"), filepath.Join(base, "logs"), base, filepath.Dir(plan.Config)} {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					t.Errorf("fixture cleanup %s: %v", path, err)
				}
			}
			if _, err := NativeCommand(cleanup, "/usr/bin/systemctl", "daemon-reload"); err != nil {
				t.Error(err)
			}
		})
		id.InstanceID, id.Executable, id.Config = plan.InstanceID, plan.Executable, plan.Config
		id.PID, err = servicePID(ctx, id, plan, NativeCommand)
		require.NoError(t, err)
		return id, plan
	}
	id, plan := create()
	other, otherPlan := create()
	configSum, err := digestFile(plan.Config)
	require.NoError(t, err)
	serviceSum, err := digestFile(plan.ServiceFile)
	require.NoError(t, err)
	binarySum, err := digestFile(plan.Executable)
	require.NoError(t, err)
	require.NoError(t, UpgradeInstallation(ctx, id, self))
	// A real executable that immediately exits exercises native startup failure
	// and recovery, not a mocked service manager.
	require.Error(t, UpgradeInstallation(ctx, id, "/usr/bin/false"))
	got, err := digestFile(plan.Executable)
	require.NoError(t, err)
	require.Equal(t, binarySum, got)
	got, err = digestFile(plan.Config)
	require.NoError(t, err)
	require.Equal(t, configSum, got)
	got, err = digestFile(plan.ServiceFile)
	require.NoError(t, err)
	require.Equal(t, serviceSum, got)
	id.PID, err = servicePID(ctx, id, plan, NativeCommand)
	require.NoError(t, err)
	_, err = Check(ctx, id, NativeCommand)
	require.NoError(t, err)
	pid, err := servicePID(ctx, other, otherPlan, NativeCommand)
	require.NoError(t, err)
	require.Equal(t, other.PID, pid)
	t.Log("isolated installation, upgrade and failed-binary rollback passed; second instance unchanged")
}
