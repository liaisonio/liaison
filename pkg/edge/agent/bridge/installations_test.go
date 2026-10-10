package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestInstallationUpgradePreservesIdentityAndAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installations.json")
	s, err := loadInstallationStore(path)
	require.NoError(t, err)
	i := discovery.Installation{Agent: "codex", Path: "/bin/codex", ResolvedPath: "/versions/v1/codex"}
	firstID := installationID(i)
	resolve := func() installationSet {
		t.Helper()
		got, err := s.resolve(context.Background(), discovery.Result{Installations: []discovery.Installation{i}})
		require.NoError(t, err)
		return got
	}
	require.Equal(t, firstID, resolve().canonical[installationKey(i)])
	i.ResolvedPath = "/versions/v2/codex"
	secondID := installationID(i)
	got := resolve()
	require.Equal(t, firstID, got.canonical[installationKey(i)])
	require.Equal(t, i, got.byID[firstID])
	require.Equal(t, i, got.byID[secondID])
	s, err = loadInstallationStore(path)
	require.NoError(t, err)
	i.ResolvedPath = "/versions/v3/codex"
	got = resolve()
	require.Equal(t, firstID, got.canonical[installationKey(i)])
	require.Equal(t, i, got.byID[firstID])
	require.Equal(t, i, got.byID[secondID])
	for _, change := range []string{"missing", "path", "provider", "wrapper"} {
		t.Run(change, func(t *testing.T) {
			other := i
			switch change {
			case "path":
				other.Path = "/other/codex"
			case "provider":
				other.Agent = "claude"
			case "wrapper":
				other.Wrapper = true
			}
			found := discovery.Result{}
			if change != "missing" {
				found.Installations = []discovery.Installation{other}
			}
			got, err := s.resolve(context.Background(), found)
			require.NoError(t, err)
			require.NotContains(t, got.byID, firstID)
		})
	}
	stat, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), stat.Mode().Perm())
}

func TestInstallationStoreConcurrentDiscovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installations.json")
	s, err := loadInstallationStore(path)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for n := 0; n < 16; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.resolve(context.Background(), discovery.Result{Installations: []discovery.Installation{{Agent: "codex", Path: "/bin/codex", ResolvedPath: "/versions/v1/codex"}}})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	reloaded, err := loadInstallationStore(path)
	require.NoError(t, err)
	require.Len(t, reloaded.items, 1)
}

func TestInstallationStoreFailsClosed(t *testing.T) {
	for _, raw := range []string{
		`{"version":2,"items":[]}`,
		`{"version":1,"items":[]} {}`,
		`{"version":1,"items":[{"id":"bad","kind":"codex","path":"/bin/codex"}]}`,
		`{"version":1,"items":[{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","kind":"codex","path":"relative"}]}`,
		`{"version":1,"items":[{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","kind":"codex","path":"/bin/codex"},{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","kind":"claude","path":"/bin/claude"}]}`,
	} {
		path := filepath.Join(t.TempDir(), "installations.json")
		require.NoError(t, os.WriteFile(path, []byte(raw), 0600))
		_, err := loadInstallationStore(path)
		require.Error(t, err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "blocked", "installations.json")
	s, err := loadInstallationStore(path)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "blocked"), nil, 0600))
	_, err = s.resolve(context.Background(), discovery.Result{Installations: []discovery.Installation{{Agent: "codex", Path: "/bin/codex", ResolvedPath: "/bin/codex"}}})
	require.Error(t, err)
	require.Empty(t, s.items, "未持久化的身份不能生效")
}

func TestStartWithSavedInstallationAfterUpgrade(t *testing.T) {
	dir := t.TempDir()
	i := discovery.Installation{Agent: "fake", Path: "/bin/agent", ResolvedPath: "/versions/v1/agent"}
	discover := func(context.Context) (discovery.Result, discovery.Environment, error) {
		return discovery.Result{Installations: []discovery.Installation{i}}, discovery.Environment{OS: "darwin", AccountID: "501", Home: dir}, nil
	}
	var latest *fakeAgent
	b, err := newBridge(context.Background(), factoryAdapter{latest: &latest}, discover, filepath.Join(dir, "bindings.json"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, b.Close()) })
	discovered := call(b, "alice", proto.EdgeAgentRequest{Action: "discover"})
	require.Equal(t, "ok", discovered.Status)
	require.Len(t, discovered.Installations, 1)
	id := discovered.Installations[0].ID
	i.ResolvedPath = "/versions/v2/agent"
	r := call(b, "alice", proto.EdgeAgentRequest{Action: "start", AccessID: strings.Repeat("a", 32), InstallationID: id, Project: dir})
	require.Equal(t, "ok", r.Status)
	require.NotEmpty(t, r.SessionID)
	unknown := call(b, "alice", proto.EdgeAgentRequest{Action: "start", InstallationID: strings.Repeat("f", 32), Project: dir})
	require.Equal(t, "not_found", unknown.Status)
}
