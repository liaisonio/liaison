package webide

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

// Opt-in integration test using an official release archive, without downloading
// or relying on a user's existing code-server installation or profile.
func TestNativeRuntime(t *testing.T) {
	archive := os.Getenv("LIAISON_WEBIDE_TEST_ARCHIVE")
	parent := os.Getenv("LIAISON_WEBIDE_TEST_ROOT")
	if archive == "" || parent == "" {
		t.Skip("native archive and private test parent not configured")
	}
	require.True(t, canLaunch(), "native IDE test requires a non-root account")
	root, err := os.MkdirTemp(parent, ".ide-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(root)) })
	stateRoot := filepath.Join(root, "Application Support", strings.Repeat("long-state-", 10))
	s, err := New(stateRoot)
	require.NoError(t, err)
	f, err := os.Open(archive)
	require.NoError(t, err)
	defer f.Close()
	_, err = InstallOffline(t.Context(), filepath.Join(stateRoot, "packages"), f)
	require.NoError(t, err)
	discovered := s.Handle(t.Context(), proto.WebIDERequest{Version: 1, Action: "discover", OwnerID: "native-test"})
	require.Len(t, discovered.Installations, 1)
	q := proto.WebIDERequest{Version: 1, Action: "start", OwnerID: "native-test", AccessID: "test-access", InstallationID: discovered.Installations[0].ID}
	out := s.Handle(t.Context(), q)
	// Register cleanup even when startup fails after creating a process.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, r := range s.records {
			require.NoError(t, stopProcess(ctx, r))
			require.NoError(t, os.RemoveAll(filepath.Dir(r.Socket)))
		}
	})
	require.Equal(t, "ok", out.Status)
	require.Len(t, out.Instances, 1)
	id := out.Instances[0].ID
	// The upstream default IPC path can exceed sockaddr_un's limit under a
	// normal home directory. The managed runtime must use its short socket.
	var ipc net.Conn
	require.Eventually(t, func() bool {
		ipc, err = net.DialTimeout("unix", filepath.Join(filepath.Dir(s.records[id].Socket), "i"), time.Second)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)
	require.NoError(t, ipc.Close())
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return s.Dial(ctx, q.OwnerID, q.AccessID, id)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost/healthz")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_, err = s.Dial(t.Context(), "other-owner", q.AccessID, id)
	require.ErrorIs(t, err, ErrUnavailable)
	again, err := New(stateRoot)
	require.NoError(t, err)
	s = again // Cleanup must track the new PID if the restored instance restarts.
	resumed := again.Handle(t.Context(), q)
	require.Equal(t, "ok", resumed.Status)
	require.Equal(t, id, resumed.Instances[0].ID)
	stopped := again.Handle(t.Context(), proto.WebIDERequest{Version: 1, Action: "stop", OwnerID: q.OwnerID, AccessID: q.AccessID, InstanceID: id})
	require.Equal(t, "ok", stopped.Status)
	_, err = again.Dial(t.Context(), q.OwnerID, q.AccessID, id)
	require.ErrorIs(t, err, ErrUnavailable)
	settings := filepath.Join(stateRoot, id, "data", "User", "settings.json")
	custom := []byte("// preserve user settings when restarting\n{\"editor.fontSize\":14}\n")
	require.NoError(t, os.WriteFile(settings, custom, 0600))
	restarted := again.Handle(t.Context(), q)
	require.Equal(t, "ok", restarted.Status)
	require.Equal(t, id, restarted.Instances[0].ID)
	saved, err := os.ReadFile(settings)
	require.NoError(t, err)
	require.Equal(t, custom, saved)
	q.Project = root
	otherProject := again.Handle(t.Context(), q)
	require.Equal(t, "ok", otherProject.Status)
	require.NotEqual(t, id, otherProject.Instances[0].ID)
	require.NotEqual(t, s.records[id].DataDir, s.records[otherProject.Instances[0].ID].DataDir)
	require.Empty(t, s.records[id].Project, "opening another project must not mutate the existing runtime")
	reusedProject := again.Handle(t.Context(), q)
	require.Equal(t, otherProject.Instances[0].ID, reusedProject.Instances[0].ID)
	stopped = again.Handle(t.Context(), proto.WebIDERequest{Version: 1, Action: "stop", OwnerID: q.OwnerID, AccessID: q.AccessID, InstanceID: id})
	require.Equal(t, "ok", stopped.Status)
}
