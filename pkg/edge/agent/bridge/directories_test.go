package bridge

import (
	"context"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryBrowseBoundary(t *testing.T) {
	home, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	outside, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(home, "project"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(home, "private-key"), []byte("never expose file contents"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(home, "escape")))
	r := browseDirectories(context.Background(), home, "", home)
	require.Equal(t, "ok", r.Status)
	require.Len(t, r.Directories, 1)
	require.Equal(t, "project", r.Directories[0].Name)
	require.Empty(t, r.ParentDirectory)
	for _, path := range []string{outside, filepath.Join(home, "escape"), filepath.Join(home, ".."), filepath.Join(home, "private-key"), "relative"} {
		require.Equal(t, "invalid_request", browseDirectories(context.Background(), home, "", path).Status, path)
	}
	empty := browseDirectories(context.Background(), home, "", filepath.Join(home, "project"))
	require.Equal(t, "ok", empty.Status)
	require.Empty(t, empty.Directories)
	require.Equal(t, home, empty.ParentDirectory)
	require.Equal(t, "ok", browseDirectories(context.Background(), home, outside, outside).Status)
}

func TestSessionWorkingDirectoryBoundary(t *testing.T) {
	b, _, req := setup(t)
	child := filepath.Join(req.Project, "child")
	require.NoError(t, os.Mkdir(child, 0700))
	req.WorkingDirectory = t.TempDir()
	require.Equal(t, "invalid_request", call(b, "alice", req).Status)
	req.WorkingDirectory = child
	r := call(b, "alice", req)
	require.Equal(t, "ok", r.Status)
	canonical, err := filepath.EvalSymlinks(child)
	require.NoError(t, err)
	require.Equal(t, canonical, r.Project)
	require.True(t, call(b, "alice", proto.EdgeAgentRequest{Action: "stop", SessionID: r.SessionID}).Closed)
}
