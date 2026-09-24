package codex

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

func TestNativeLaunchDoesNotNeedNode(t *testing.T) {
	in := discovery.Installation{Path: "/bin/codex", ResolvedPath: "/bin/codex"}
	spec, err := launchSpec(in, "/project")
	if err != nil || spec.Executable != in.Path || len(spec.Args) != 3 {
		t.Fatalf("unexpected launch: %+v %v", spec, err)
	}
}

func TestNPMLaunchWithServicePath(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("Unix non-root launch test")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "codex.js")
	entry := filepath.Join(dir, "codex")
	node := filepath.Join(dir, "node")
	for _, p := range []string{script, node} {
		if err := os.WriteFile(p, []byte("not executed by test\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(script, entry); err != nil {
		t.Fatal(err)
	}
	resolvedScript, err := filepath.EvalSymlinks(script)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin:/bin")
	spec, err := launchSpec(discovery.Installation{Path: entry, ResolvedPath: resolvedScript}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Executable != node || spec.Args[0] != resolvedScript || spec.Args[1] != "app-server" {
		t.Fatalf("wrong interpreter plan: %+v", spec)
	}
	if err := os.Chmod(script, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := launchSpec(discovery.Installation{Path: entry, ResolvedPath: resolvedScript}, dir); err == nil {
		t.Fatal("unsafe script bypassed validation")
	}
}
