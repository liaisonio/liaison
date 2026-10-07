package webide

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

type testInstaller func(context.Context, string) (string, error)

func TestDiscoveryAfterOfflineRepair(t *testing.T) {
	for _, kind := range []string{"usable", "missing", "non-executable", "outside-package", "installing", "state-failed"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.Chmod(root, 0700))
			s, err := NewWithInstaller(root, testInstaller(func(context.Context, string) (string, error) {
				t.Error("discovery must not invoke installer")
				return "", errors.New("unexpected installation")
			}))
			require.NoError(t, err)
			s.installStatus = "install_failed"
			require.NoError(t, s.saveInstallStatus(s.installStatus))
			before, err := os.ReadFile(filepath.Join(root, "installation.json"))
			require.NoError(t, err)
			program := filepath.Join(root, "packages", packageName(), "bin", "code-server")
			if kind != "missing" {
				require.NoError(t, os.MkdirAll(filepath.Dir(program), 0700))
				if kind == "outside-package" {
					require.NoError(t, os.Symlink("/bin/sh", program))
				} else {
					mode := os.FileMode(0700)
					if kind == "non-executable" {
						mode = 0600
					}
					require.NoError(t, os.WriteFile(program, []byte("#!/bin/sh\nexit 99\n"), mode))
				}
			}
			want := "install_failed"
			switch kind {
			case "usable":
				want = "ok"
			case "installing":
				s.installing = true
				want = "installing"
			case "state-failed":
				s.installStatus = "state_failed"
				want = "state_failed"
			}
			out := s.Handle(t.Context(), proto.WebIDERequest{Version: 1, OwnerID: "test-owner", Action: "discover"})
			require.Equal(t, want, out.Status)
			after, err := os.ReadFile(filepath.Join(root, "installation.json"))
			require.NoError(t, err)
			require.Equal(t, before, after, "read-only discovery preserves installation history")
		})
	}
}

func (f testInstaller) Install(ctx context.Context, root string) (string, error) { return f(ctx, root) }

func TestInstallationStateSurvivesRestartWithoutReplay(t *testing.T) {
	for _, status := range []string{"ok", "install_failed", "installing"} {
		t.Run(status, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.Chmod(root, 0700))
			s, err := New(root)
			require.NoError(t, err)
			require.NoError(t, s.saveInstallStatus(status))
			var calls atomic.Int32
			again, err := NewWithInstaller(root, testInstaller(func(context.Context, string) (string, error) { calls.Add(1); return "", nil }))
			require.NoError(t, err)
			want := status
			if want == "installing" {
				want = "install_failed"
			}
			require.Equal(t, want, again.installStatus)
			require.False(t, again.installing)
			require.Zero(t, calls.Load())
			info, err := os.Stat(filepath.Join(root, "installation.json"))
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		})
	}
}

func TestInstallationRejectsCorruptState(t *testing.T) {
	for _, content := range []string{`{"version":9,"status":"ok"}`, `{"version":1,"status":"unknown"}`, `{} {}`, `{"version":1,"status":"ok"}` + strings.Repeat(" ", 4096)} {
		root := t.TempDir()
		require.NoError(t, os.Chmod(root, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "installation.json"), []byte(content), 0600))
		_, err := New(root)
		require.Error(t, err)
	}
}

func TestConcurrentInstallDoesNotStartDuplicateTask(t *testing.T) {
	if !canLaunch() {
		t.Skip("installation requires a non-root supported platform")
	}
	var calls atomic.Int32
	release := make(chan struct{})
	root := t.TempDir()
	require.NoError(t, os.Chmod(root, 0700))
	s, err := NewWithInstaller(root, testInstaller(func(ctx context.Context, root string) (string, error) {
		calls.Add(1)
		select {
		case <-release:
			return "", errors.New("simulated failure")
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}))
	require.NoError(t, err)
	q := proto.WebIDERequest{Version: 1, OwnerID: "test-owner", Action: "install"}
	for range 5 {
		require.Equal(t, "installing", s.Handle(t.Context(), q).Status)
	}
	close(release)
	require.Eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.installing }, time.Second, time.Millisecond)
	require.EqualValues(t, 1, calls.Load())
	again, err := New(s.root)
	require.NoError(t, err)
	require.Equal(t, "install_failed", again.installStatus)
}

func TestOfflinePackageNeverFallsBackOnInvalidArchive(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "offline", packageName()+".tar.gz")
	require.NoError(t, os.MkdirAll(filepath.Dir(cache), 0700))
	require.NoError(t, os.WriteFile(cache, []byte("invalid archive"), 0600))
	_, err := (PackageInstaller{}).Install(t.Context(), root)
	require.ErrorContains(t, err, "checksum mismatch")
	_, err = os.Stat(filepath.Join(root, packageName()))
	require.True(t, errors.Is(err, os.ErrNotExist))
	_, err = InstallOffline(t.Context(), root, nil)
	require.Error(t, err)
}

func TestInstallRejectsBrokenExistingPackageWithoutOverwriting(t *testing.T) {
	if !canLaunch() {
		t.Skip("installation requires a non-root supported platform")
	}
	for _, kind := range []string{"directory", "non-executable", "broken-link", "outside-package"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.Chmod(root, 0700))
			program := filepath.Join(root, "packages", packageName(), "bin", "code-server")
			require.NoError(t, os.MkdirAll(filepath.Dir(program), 0700))
			switch kind {
			case "directory":
				require.NoError(t, os.Mkdir(program, 0700))
			case "non-executable":
				require.NoError(t, os.WriteFile(program, []byte("not an executable"), 0600))
			case "broken-link":
				require.NoError(t, os.Symlink("missing", program))
			case "outside-package":
				require.NoError(t, os.Symlink("/bin/sh", program))
			}
			before, err := os.Lstat(program)
			require.NoError(t, err)
			var calls atomic.Int32
			s, err := NewWithInstaller(root, testInstaller(func(context.Context, string) (string, error) {
				calls.Add(1)
				return "", nil
			}))
			require.NoError(t, err)
			out := s.Handle(t.Context(), proto.WebIDERequest{Version: 1, OwnerID: "test-owner", Action: "install"})
			require.Equal(t, "install_failed", out.Status)
			require.Zero(t, calls.Load())
			after, err := os.Lstat(program)
			require.NoError(t, err)
			require.True(t, os.SameFile(before, after))
			again, err := New(root)
			require.NoError(t, err)
			require.Equal(t, "install_failed", again.installStatus)
		})
	}
}

func TestInstallerSuccessRequiresUsablePackage(t *testing.T) {
	if !canLaunch() {
		t.Skip("installation requires a non-root supported platform")
	}
	root := t.TempDir()
	require.NoError(t, os.Chmod(root, 0700))
	s, err := NewWithInstaller(root, testInstaller(func(context.Context, string) (string, error) {
		return "/bin/sh", nil
	}))
	require.NoError(t, err)
	require.Equal(t, "installing", s.Handle(t.Context(), proto.WebIDERequest{Version: 1, OwnerID: "test-owner", Action: "install"}).Status)
	require.Eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.installing }, time.Second, time.Millisecond)
	again, err := New(root)
	require.NoError(t, err)
	require.Equal(t, "install_failed", again.installStatus)
}
