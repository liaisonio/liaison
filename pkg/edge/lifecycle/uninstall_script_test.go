package lifecycle

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyUninstallScriptFailsClosed(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	script, err := filepath.Abs("../../../dist/edge/uninstall.sh")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(bash, script)
	// No external commands are available. The refusal must use builtins only.
	command.Env = []string{"PATH=" + t.TempDir()}
	output, err := command.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("expected safe refusal, got %v", err)
	}
	for _, expected := range []string{"legacy global Edge uninstaller is disabled", "No services, processes, configuration or project files were changed."} {
		if !strings.Contains(string(output), expected) {
			t.Fatalf("missing refusal guidance: %s", expected)
		}
	}
}
