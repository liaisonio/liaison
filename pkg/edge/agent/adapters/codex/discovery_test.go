package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

func TestLayouts(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			env := discovery.Environment{OS: platform, Home: t.TempDir(), NpmPrefix: filepath.Join(t.TempDir(), "npm"), AppData: t.TempDir(), LocalAppData: t.TempDir()}
			layout, err := (Adapter{}).Layout(env)
			if err != nil {
				t.Fatal(err)
			}
			if len(layout.Defaults) < 3 || len(layout.VersionRoots) == 0 {
				t.Fatalf("missing defaults %+v", layout)
			}
			if platform == "windows" && layout.Names[1] != "codex.cmd" {
				t.Fatal("npm wrapper missing")
			}
			found := false
			for _, p := range layout.Defaults {
				if strings.HasPrefix(p, env.NpmPrefix+string(filepath.Separator)) {
					found = true
				}
			}
			if !found {
				t.Fatal("npm prefix missing")
			}
		})
	}
	if _, err := (Adapter{}).Layout(discovery.Environment{OS: "unsupported"}); err == nil {
		t.Fatal("unsupported platform accepted")
	}
}

func TestCurrentHostDiscovery(t *testing.T) {
	if os.Getenv("LIAISON_TEST_AGENT_DISCOVERY") != "1" {
		t.Skip("opt-in, read-only host discovery")
	}
	env, err := discovery.CurrentEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	platform, err := discovery.NewNative(env)
	if err != nil {
		t.Fatal(err)
	}
	result, err := discovery.Find(context.Background(), platform, env, Adapter{}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, installation := range result.Installations {
		t.Logf("candidate (%s): %s", installation.Source, installation.Path)
	}
	t.Logf("%d unverified candidates; truncated=%v", len(result.Installations), result.Truncated)
}
