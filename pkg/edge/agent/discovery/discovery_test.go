package discovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type testAdapter struct{ layout Layout }

func (testAdapter) Kind() string                         { return "fixture" }
func (a testAdapter) Layout(Environment) (Layout, error) { return a.layout, nil }

func program(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	// Invalid executable content intentionally proves discovery doesn't run it.
	if err := os.WriteFile(path, []byte("not an executable; do not run"), 0700); err != nil {
		t.Fatal(err)
	}
}
func native(t *testing.T, env Environment) *Native {
	t.Helper()
	p, err := NewNative(env)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFindPriorityAndNoExecution(t *testing.T) {
	root := t.TempDir()
	explicit := filepath.Join(root, "explicit")
	pathProgram := filepath.Join(root, "path", "agent")
	fallback := filepath.Join(root, "default")
	for _, p := range []string{explicit, pathProgram, fallback} {
		program(t, p)
	}
	env := Environment{OS: runtime.GOOS, Home: root, PathDirs: []string{"", ".", "relative", filepath.Dir(pathProgram)}}
	a := testAdapter{Layout{Names: []string{"agent"}, Defaults: []string{fallback, pathProgram}}}
	result, err := Find(context.Background(), native(t, env), env, a, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installations) != 3 {
		t.Fatalf("want 3 candidates, got %+v", result)
	}
	for i, want := range []string{"explicit", "path", "default"} {
		if result.Installations[i].Source != want {
			t.Fatalf("priority: %+v", result)
		}
	}
}

func TestFindSymlinksAndInvalidFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform privileges")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	alias := filepath.Join(root, "alias")
	bad := filepath.Join(root, "non-executable")
	program(t, target)
	program(t, bad)
	if err := os.Chmod(bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	env := Environment{OS: runtime.GOOS, Home: root}
	result, err := Find(context.Background(), native(t, env), env, testAdapter{Layout{Defaults: []string{target, root, bad}}}, alias)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Installations) != 1 || result.Installations[0].Path != alias || len(result.Warnings) != 2 {
		t.Fatalf("unexpected result %+v", result)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if _, err = Find(context.Background(), native(t, env), env, testAdapter{}, alias); err == nil {
		t.Fatal("broken explicit link accepted")
	}
}

func TestBoundedVersionSearch(t *testing.T) {
	root := t.TempDir()
	versions := filepath.Join(root, "versions")
	for i := 0; i < maxVersions+4; i++ {
		program(t, filepath.Join(versions, fmt.Sprintf("v%03d", i), "bin", "agent"))
	}
	env := Environment{OS: runtime.GOOS, Home: root}
	result, err := Find(context.Background(), native(t, env), env, testAdapter{Layout{Names: []string{"agent"}, VersionRoots: []VersionRoot{{versions, "bin"}}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncated || len(result.Installations) != maxVersions {
		t.Fatalf("unbounded result: %+v", result)
	}
}

func TestRelativePathCancellationAndLimits(t *testing.T) {
	env := Environment{OS: runtime.GOOS, Home: t.TempDir()}
	p := native(t, env)
	if _, err := Find(context.Background(), p, env, testAdapter{}, "./codex"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("relative path: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env.PathDirs = []string{env.Home}
	p = native(t, env)
	if _, err := Find(ctx, p, env, testAdapter{}, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	var defaults []string
	for i := 0; i < maxCandidates+2; i++ {
		defaults = append(defaults, filepath.Join(env.Home, fmt.Sprint(i)))
	}
	candidates, _, truncated, err := p.Candidates(context.Background(), Layout{Defaults: defaults}, "")
	if err != nil || !truncated || len(candidates) != maxCandidates {
		t.Fatalf("limit %d %v %v", len(candidates), truncated, err)
	}
}
