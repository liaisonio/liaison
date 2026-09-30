package claude

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

func TestLayouts(t *testing.T) {
	for _, osName := range []string{"darwin", "linux", "windows"} {
		t.Run(osName, func(t *testing.T) {
			l, err := (Adapter{}).Layout(discovery.Environment{OS: osName, Home: "/home/test", AppData: "/appdata", LocalAppData: "/local", NpmPrefix: "/npm"})
			if err != nil || len(l.Names) == 0 || len(l.VersionRoots) == 0 {
				t.Fatal("incomplete layout")
			}
			found := false
			for _, p := range l.Defaults {
				if strings.Contains(p, filepath.Join(".local", "bin")) {
					found = true
				}
				if strings.Contains(strings.ToLower(p), "codex") {
					t.Fatal("wrong provider default")
				}
			}
			if !found {
				t.Fatal("native install missing")
			}
		})
	}
	if _, err := (Adapter{}).Layout(discovery.Environment{OS: "unsupported"}); !errors.Is(err, discovery.ErrUnsupported) {
		t.Fatal(err)
	}
}

func TestLaunchProbeIsRestricted(t *testing.T) {
	i := discovery.Installation{Agent: "claude", Path: "/bin/claude", ResolvedPath: "/bin/claude"}
	s, err := launchSpec(i, "/project")
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(s.Args, "|")
	for _, required := range []string{"--safe-mode", "--tools||", "--strict-mcp-config", "--no-session-persistence", "--permission-prompt-tool|stdio"} {
		if !strings.Contains(args, required) {
			t.Fatalf("missing %s", required)
		}
	}
	if strings.Contains(args, "bypassPermissions") || strings.Contains(args, "dangerously") {
		t.Fatal("permissions bypassed")
	}
	i.ResolvedPath = "/bin/cli.js"
	if _, err := launchSpec(i, "/project"); err == nil {
		t.Fatal("untested npm interpreter accepted")
	}
	i.Agent = "codex"
	if _, err := launchSpec(i, "/project"); err == nil {
		t.Fatal("wrong agent accepted")
	}
}
