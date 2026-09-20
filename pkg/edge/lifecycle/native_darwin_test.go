//go:build darwin && integration

package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM)
		<-stop
		return
	}
	os.Exit(m.Run())
}

// This validates real user-level LaunchAgent installation and coexistence. It
// does not substitute for the separate, stricter remote-uninstall safety gate.
func TestNativeDarwinInstall(t *testing.T) {
	if os.Getenv("LIAISON_LIFECYCLE_NATIVE_TEST") != "1" {
		t.Skip("explicit temporary LaunchAgent opt-in required")
	}
	require.Positive(t, os.Geteuid())
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	self, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	create := func() (Identity, Plan) {
		raw := make([]byte, 16)
		_, err := rand.Read(raw)
		require.NoError(t, err)
		id := Identity{OS: "darwin", UID: os.Geteuid(), Home: home}
		plan, err := InstallNew(ctx, id, self, InstallCredentials{"fixture.invalid:443", hex.EncodeToString(raw), "fixture-not-a-real-credential"})
		require.NoError(t, err)
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
			defer stop()
			if _, statErr := os.Stat(plan.ServiceFile); statErr == nil {
				pid, pidErr := servicePID(cleanup, id, plan, NativeCommand)
				if err := newInstaller().stop(cleanup, id, plan, false); err != nil {
					t.Errorf("stop temporary LaunchAgent: %v", err)
					return
				}
				if pidErr == nil && pid > 0 {
					if err := waitProcessGone(cleanup, pid, NativeCommand); err != nil {
						t.Error(err)
						return
					}
				}
			} else if !os.IsNotExist(statErr) {
				t.Error(statErr)
				return
			}
			base := installationBase(plan)
			for _, path := range []string{plan.ServiceFile, plan.Config, plan.Executable, manifestPath(plan), plan.Executable + ".previous", plan.Executable + ".next", filepath.Join(base, "bin"), filepath.Join(base, "logs"), base} {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					t.Errorf("fixture cleanup %s: %v", path, err)
				}
			}
		})
		id.InstanceID, id.Executable, id.Config = plan.InstanceID, plan.Executable, plan.Config
		id.PID, err = servicePID(ctx, id, plan, NativeCommand)
		require.NoError(t, err)
		_, err = Probe(ctx, id, NativeCommand)
		require.NoError(t, err)
		require.NoError(t, ValidateFiles(plan, id.UID))
		return id, plan
	}
	first, firstPlan := create()
	second, secondPlan := create()
	require.NotEqual(t, firstPlan.Service, secondPlan.Service)
	pid, err := servicePID(ctx, first, firstPlan, NativeCommand)
	require.NoError(t, err)
	require.Equal(t, first.PID, pid)
	pid, err = servicePID(ctx, second, secondPlan, NativeCommand)
	require.NoError(t, err)
	require.Equal(t, second.PID, pid)
	_, err = Check(ctx, first, NativeCommand)
	require.NoError(t, err)
	// A same-user service referencing this program must still block lifecycle
	// operations, even though unrelated system daemons are outside this domain.
	alias := filepath.Join(home, "Library/LaunchAgents", firstPlan.Service+".fixture-reference.plist")
	require.NoError(t, exclusiveFile(alias, []byte(firstPlan.Executable), 0600))
	_, sharedErr := Check(ctx, first, NativeCommand)
	require.NoError(t, os.Remove(alias))
	require.Error(t, sharedErr)
	manifest, err := os.ReadFile(manifestPath(firstPlan))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath(firstPlan), []byte(`{"version":999}`), 0600))
	_, invalidManifestErr := Check(ctx, first, NativeCommand)
	require.NoError(t, os.WriteFile(manifestPath(firstPlan), manifest, 0600))
	require.Error(t, invalidManifestErr)
	require.NoError(t, os.Chmod(installationBase(firstPlan), 0755))
	_, sharedDirectoryErr := Check(ctx, first, NativeCommand)
	require.NoError(t, os.Chmod(installationBase(firstPlan), 0700))
	require.Error(t, sharedDirectoryErr)
	// Native replacement and rollback precede uninstall so the worker binds the
	// current PID, not a stale pre-upgrade identity.
	require.NoError(t, UpgradeInstallation(ctx, first, self))
	require.Error(t, UpgradeInstallation(ctx, first, "/usr/bin/false"))
	first.PID, err = servicePID(ctx, first, firstPlan, NativeCommand)
	require.NoError(t, err)
	_, err = Check(ctx, first, NativeCommand)
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
	command.InstanceID = first.InstanceID
	command.CallbackURL = server.URL + "/api/v1/edge-uninstall-results/" + command.TaskID
	require.NoError(t, Prepare(ctx, first, command, []string{strings.TrimPrefix(server.URL, "https://")}, true, NativeCommand))
	t.Cleanup(func() {
		// A completed one-shot submit job can remain registered in launchd.
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := NativeCommand(cleanup, "/bin/launchctl", "remove", "com.liaison.uninstall."+command.TaskID); err != nil {
			t.Logf("temporary helper already absent or remove failed: %v", err)
		}
	})
	for _, want := range []string{"running", "completed"} {
		select {
		case got := <-statuses:
			require.Equal(t, want, got)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	for _, path := range []string{firstPlan.ServiceFile, firstPlan.Config, firstPlan.Executable} {
		_, err := os.Stat(path)
		require.True(t, os.IsNotExist(err), "target remains: %s", path)
	}
	pid, err = servicePID(ctx, second, secondPlan, NativeCommand)
	require.NoError(t, err)
	require.Equal(t, second.PID, pid)
	_, err = Check(ctx, second, NativeCommand)
	require.NoError(t, err)
	t.Log("native upgrade, rollback and detached self-uninstall passed; second LaunchAgent untouched")
}
