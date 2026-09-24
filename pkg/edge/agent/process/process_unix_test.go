//go:build darwin || linux

package process

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestOwnedProcessHelper(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "liaison-owned-process-helper" {
		return
	}
	fmt.Fprintln(os.Stdout, "ready")
	time.Sleep(time.Minute)
	os.Exit(0)
}

func TestOwnedProcessCancellation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("launch intentionally disabled for root")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		t.Fatal(err)
	}
	// Go may place test binaries under a world-writable /tmp. Launch validation
	// correctly rejects those; use an explicit trusted GOTMPDIR for this smoke.
	if err := validateProgram(resolved); err != nil {
		t.Skipf("test binary is not in a trusted launch directory: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	child, err := Start(ctx, Spec{Executable: exe, ResolvedExecutable: resolved, Directory: t.TempDir(), Args: []string{"-test.run=^TestOwnedProcessHelper$", "--", "liaison-owned-process-helper"}})
	if err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(child.Output).ReadString('\n')
	if err != nil {
		cancel()
		_ = child.Wait()
		t.Fatal(err)
	}
	if line != "ready\n" {
		cancel()
		_ = child.Wait()
		t.Fatalf("unexpected readiness: %q", line)
	}
	pid := child.cmd.Process.Pid
	cancel()
	if err := child.Wait(); err == nil {
		t.Fatal("terminated process reported success")
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("owned process not reaped: %v", err)
	}
}

func TestReplacedExecutableRejected(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("launch intentionally disabled for root")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(Spec{Executable: exe, ResolvedExecutable: "/not/the/discovered/program", Directory: t.TempDir()}); err == nil {
		t.Fatal("changed executable accepted")
	}
}
