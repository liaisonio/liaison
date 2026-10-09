package webide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestLongStateRootUsesStablePrivateSocketPath(t *testing.T) {
	s := &Service{root: "/" + strings.Repeat("long-state/", 20)}
	id := "0123456789abcdef01234567"
	path := s.socketPath(id)
	require.LessOrEqual(t, len(path), 100)
	require.Equal(t, path, s.socketPath(id))
	require.NotEqual(t, path, (&Service{root: s.root + "other"}).socketPath(id))
	require.NotEqual(t, path, s.socketPath("abcdef0123456789abcdef01"))
}

func TestInstanceStateIsPrivateAndOwnerScoped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0700))
	s, err := New(dir)
	require.NoError(t, err)
	dir = s.root
	id := "0123456789abcdef01234567"
	s.records[id] = &record{Owner: "alice", WebIDEInstance: proto.WebIDEInstance{ID: id, AccessID: "a", InstallationID: "i", Status: "running"}, DataDir: filepath.Join(dir, id, "data"), Socket: filepath.Join(dir, id, "s")}
	s.records[id].ApplicationID = "owned-app"
	require.NoError(t, s.save())
	again, err := New(dir)
	require.NoError(t, err)
	q := proto.WebIDERequest{Version: 1, Action: "instances", OwnerID: "bob"}
	require.Empty(t, again.Handle(t.Context(), q).Instances)
	q.OwnerID = "alice"
	out := again.Handle(t.Context(), q)
	require.Len(t, out.Instances, 1)
	require.Equal(t, "owned-app", out.Instances[0].ApplicationID)
	require.Equal(t, "stopped", out.Instances[0].Status)
	_, err = again.Dial(t.Context(), "bob", "a", id)
	require.ErrorIs(t, err, ErrUnavailable)
	q.Action = "stop"
	q.OwnerID = "bob"
	q.AccessID = "a"
	q.InstanceID = id
	require.Equal(t, "not_found", again.Handle(t.Context(), q).Status)
	i, err := os.Stat(filepath.Join(dir, "instances.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), i.Mode().Perm())
}
func TestProcessEnvironmentDoesNotInheritCredentials(t *testing.T) {
	t.Setenv("PASSWORD", "secret")
	t.Setenv("GITHUB_TOKEN", "secret")
	t.Setenv("OPENAI_API_KEY", "secret")
	t.Setenv("CODE_SERVER_CONFIG", "/untrusted")
	for _, v := range processEnvironment() {
		require.NotContains(t, v, "secret")
		require.NotContains(t, v, "CODE_SERVER_CONFIG")
	}
}
func TestDirectoryListingAndCancellation(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "project"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), nil, 0600))
	out := directories(t.Context(), dir)
	require.Equal(t, "ok", out.Status)
	require.Len(t, out.Directories, 1)
	require.Equal(t, "project", out.Directories[0].Name)
	require.Equal(t, "invalid_request", directories(t.Context(), "relative").Status)
}
