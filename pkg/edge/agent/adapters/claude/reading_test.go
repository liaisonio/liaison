package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/process"
)

func TestInteractiveToolsUseNativeDefaultPermissions(t *testing.T) {
	spec := interactiveSpec(process.Spec{Args: []string{"--tools", ""}})
	if spec.Args[1] != "Read,Glob,Grep,Bash,AskUserQuestion" {
		t.Fatal("unexpected tool capabilities")
	}
	joined := strings.Join(spec.Args, " ")
	if !strings.HasSuffix(joined, "--permission-mode default") {
		t.Fatal("native default permission mode missing")
	}
	for _, flag := range []string{"bypassPermissions", "dangerously-skip-permissions", "--allowedTools", "--settings", "acceptEdits"} {
		if strings.Contains(joined, flag) {
			t.Fatal("native permission checks bypassed")
		}
	}
}

// Opt-in: only reads a synthetic temporary file; no permission request is accepted.
func TestNativeStructuredReading(t *testing.T) {
	testNativeReading(t, "Read", "Use the Read tool to read only README.md in the current directory, then reply with its contents. Do not use Bash or access other files.")
}

func TestNativeBashReading(t *testing.T) {
	testNativeReading(t, "Bash", "Use Bash exactly once with command `cat README.md` in the current directory, then reply with its contents. Do not run other commands or access other files.")
}

func TestNativeModelCatalog(t *testing.T) {
	testNativeReading(t, "Read", "MODEL_PROBE")
}

func testNativeReading(t *testing.T, tool, prompt string) {
	t.Helper()
	if os.Getenv("LIAISON_TEST_CLAUDE_READING") != "1" {
		t.Skip("native reading probe disabled")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	env, err := discovery.CurrentEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	platform, err := discovery.NewNative(env)
	if err != nil {
		t.Fatal(err)
	}
	found, err := discovery.Find(ctx, platform, env, Adapter{}, "")
	if err != nil {
		t.Fatal("discovery failed")
	}
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "README.md"), []byte("READING_PROBE_ORCHID"), 0600); err != nil {
		t.Fatal(err)
	}
	var spec process.Spec
	for _, i := range found.Installations {
		if candidate, err := launchSpec(i, project); err == nil {
			spec = candidate
			break
		}
	}
	if spec.Executable == "" {
		t.Fatal("native installation missing")
	}
	d, err := startDriver(ctx, interactiveSpec(spec))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	if prompt == "MODEL_PROBE" {
		models := d.models()
		if len(models) == 0 {
			t.Fatal("missing native models")
		}
		if _, err := d.control(ctx, struct {
			Subtype string `json:"subtype"`
			Model   string `json:"model"`
		}{"set_model", models[0].ID}); err != nil {
			t.Fatal("native model switch rejected")
		}
		t.Logf("native catalog: %d models, %d commands; model control accepted", len(models), len(d.skills()))
		prompt = "Use the Read tool to read only README.md, then reply with the exact contents. Do not use other tools."
	}
	if err := d.Send(ctx, prompt); err != nil {
		t.Fatal(err)
	}
	usedRead, sawContents := false, false
	for {
		select {
		case event, ok := <-d.Events():
			if !ok {
				t.Fatal("closed before result")
			}
			if event.Type == "control_request" {
				t.Fatal("native reading unexpectedly required approval; no approval sent")
			}
			if event.Type == "assistant" && strings.Contains(string(event.Raw), `"name":"`+tool+`"`) {
				usedRead = true
			}
			if strings.Contains(string(event.Raw), "READING_PROBE_ORCHID") {
				sawContents = true
			}
			if event.Type == "result" {
				if !usedRead || !sawContents {
					t.Fatal("reading tool result not verified")
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("reading probe timeout")
		}
	}
}
