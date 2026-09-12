package web

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestWebSSHIntegrationDoesNotEnumeratePaths(t *testing.T) {
	if strings.Contains(webSSHBashIntegration, "LiaisonPaths") || strings.Contains(webSSHBashIntegration, "find .") {
		t.Fatal("AI completion must not retain the path enumeration provider")
	}
}

type shellStarterProbe struct {
	shell   bool
	command string
	err     error
}

func (p *shellStarterProbe) Shell() error               { p.shell = true; return p.err }
func (p *shellStarterProbe) Start(command string) error { p.command = command; return p.err }

func TestWebSSHShellFeatureFlag(t *testing.T) {
	for _, value := range []string{"", "false", "true", "TRUE"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("LIAISON_WEBSSH_SHELL_INTEGRATION", value)
			failure := errors.New("shell refused")
			probe := &shellStarterProbe{err: failure}
			if !errors.Is(startWebSSHShell(probe), failure) {
				t.Fatal("startup error was lost")
			}
			enabled := strings.EqualFold(value, "true")
			if probe.shell == enabled || (probe.command != "") != enabled {
				t.Fatal("unexpected shell startup path")
			}
		})
	}
}

func TestShellLiteral(t *testing.T) {
	for _, value := range []string{"plain", "a'b", "$(false)\n;\""} {
		out, err := exec.Command("/bin/sh", "-c", "printf %s "+shellLiteral(value)).Output()
		if err != nil || string(out) != value {
			t.Fatalf("literal round trip failed: %v", err)
		}
	}
}

func TestWebSSHShellSyntax(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-n")
	cmd.Stdin = strings.NewReader(webSSHShellBootstrap())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bootstrap syntax: %v: %s", err, output)
	}
	cmd = exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(webSSHBashIntegration)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("integration syntax: %v: %s", err, output)
	}
}
