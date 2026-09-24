package lifecycle

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDownloadScriptDispatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	script, err := filepath.Abs("../../../dist/edge/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"new", "new-with-uninstall", "upgrade", "binary-failure", "missing-mode"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			payload := filepath.Join(dir, "payload")
			for _, path := range []string{bin, payload} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, body string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(payload, "liaison-edge"), "#!/bin/bash\nprintf '%s\\n' \"$@\" > \"$FIXTURE_ARGS\"\nif [[ $1 == --edge-install-new ]]; then /bin/cat > \"$FIXTURE_INPUT\"; fi\nexit \"$FIXTURE_EXIT\"\n")
			archive := filepath.Join(dir, "edge.tar.gz")
			if data, err := exec.Command("tar", "-czf", archive, "-C", payload, "liaison-edge").CombinedOutput(); err != nil {
				t.Fatalf("archive: %v %s", err, data)
			}
			write(filepath.Join(bin, "curl"), "#!/bin/bash\nwhile [[ $# -gt 0 ]]; do if [[ $1 == -o ]]; then shift; /bin/cp \"$FIXTURE_ARCHIVE\" \"$1\"; fi; shift; done\nprintf 200\n")
			// Any accidental legacy fallthrough is an observable failure, never sudo.
			write(filepath.Join(bin, "sudo"), "#!/bin/bash\necho legacy-fallthrough >&2\nexit 97\n")
			args := []string{script, "--server-http-addr=fixture.invalid", "--server-edge-addr=fixture.invalid:443", "--access-key=fixture-ak", "--secret-key=fixture-sk"}
			switch mode {
			case "upgrade":
				args = append(args, "--upgrade-instance="+strings.Repeat("a", 32))
			case "new", "new-with-uninstall", "binary-failure":
				args = append(args, "--new-instance")
				if mode == "new-with-uninstall" {
					args = append(args, "--allow-remote-uninstall")
				}
			}
			code := "0"
			if mode == "binary-failure" {
				code = "88"
			}
			cmd := exec.Command("bash", args...)
			cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "FIXTURE_ARCHIVE="+archive, "FIXTURE_ARGS="+filepath.Join(dir, "args"), "FIXTURE_INPUT="+filepath.Join(dir, "input"), "FIXTURE_EXIT="+code)
			data, err := cmd.CombinedOutput()
			if strings.Contains(string(data), "legacy-fallthrough") || strings.Contains(string(data), "fixture-sk") {
				t.Fatalf("unsafe dispatch: %s", data)
			}
			if mode == "missing-mode" {
				if strings.Contains(string(data), "successfully") {
					t.Fatal("reported success without an installation mode")
				}
				if err == nil {
					t.Fatal("implicit mode accepted")
				}
				if _, e := os.Stat(filepath.Join(dir, "args")); !os.IsNotExist(e) {
					t.Fatal("binary executed without explicit mode")
				}
				return
			}
			if mode == "binary-failure" {
				if strings.Contains(string(data), "successfully") {
					t.Fatal("reported success after binary failure")
				}
				if cmd.ProcessState.ExitCode() != 88 {
					t.Fatalf("lost failure exit: %v %s", err, data)
				}
			} else if err != nil {
				t.Fatalf("dispatch: %v %s", err, data)
			} else {
				result := "Liaison Edge installed successfully."
				if mode == "upgrade" {
					result = "Liaison Edge upgraded successfully."
				}
				if !strings.Contains(string(data), result) || !strings.HasSuffix(strings.TrimSpace(string(data)), "Return to the console and confirm that this connector is online.") {
					t.Fatalf("missing installation result or next step: %s", data)
				}
			}
			got, err := os.ReadFile(filepath.Join(dir, "args"))
			if err != nil {
				t.Fatal(err)
			}
			want := "--edge-install-new\n"
			if mode == "upgrade" {
				want = "--edge-upgrade-instance\n" + strings.Repeat("a", 32) + "\n"
			}
			if string(got) != want {
				t.Fatalf("unexpected binary args: %q", got)
			}
			if mode != "upgrade" {
				got, err = os.ReadFile(filepath.Join(dir, "input"))
				if err != nil || string(got) != "fixture.invalid:443\nfixture-ak\nfixture-sk\n" {
					t.Fatal("enrollment stdin mismatch")
				}
			}
		})
	}
}
