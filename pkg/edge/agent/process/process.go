// Package process owns only child processes explicitly started by this Edge.
package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

type Spec struct {
	Executable, ResolvedExecutable, Directory string
	Args                                      []string
}

// Child provides protocol pipes and deterministic lifetime. It does not attach
// to or signal a process discovered by name/PID.
type Child struct {
	Input   io.WriteCloser
	Output  io.ReadCloser
	cmd     *exec.Cmd
	cleanup func() error
}

func Validate(spec Spec) error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	if u.Uid == "0" || strings.HasSuffix(u.Uid, "-18") {
		return errors.New("agent launch requires a non-system user Edge")
	}
	if !filepath.IsAbs(spec.Executable) || !filepath.IsAbs(spec.Directory) {
		return errors.New("absolute program and project paths required")
	}
	resolved, err := filepath.EvalSymlinks(spec.Executable)
	if err != nil {
		return err
	}
	if resolved != spec.ResolvedExecutable {
		return errors.New("agent executable changed; rediscover before launching")
	}
	if err := validateProgram(resolved); err != nil {
		return err
	}
	info, err := os.Stat(spec.Directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("project must be a directory")
	}
	return nil
}

func Start(ctx context.Context, spec Spec) (*Child, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := Validate(spec); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, spec.ResolvedExecutable, spec.Args...)
	cmd.Dir = spec.Directory
	cmd.Env = localEnvironment()
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 3 * time.Second
	cleanup, err := prepare(cmd)
	if err != nil {
		return nil, err
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		if closeErr := cleanup(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Join(err, in.Close(), cleanup())
	}
	if err = cmd.Start(); err != nil {
		return nil, errors.Join(err, in.Close(), out.Close(), cleanup())
	}
	if err = afterStart(cmd); err != nil {
		killErr := cmd.Process.Kill()
		waitErr := cmd.Wait()
		return nil, errors.Join(err, killErr, waitErr, in.Close(), out.Close(), cleanup())
	}
	return &Child{Input: in, Output: out, cmd: cmd, cleanup: cleanup}, nil
}

func (c *Child) Wait() error { return errors.Join(c.cmd.Wait(), c.cleanup()) }

// Environment propagation is an allowlist, not a copy of Edge's service
// environment. Agent configuration/credentials are read by the Agent locally.
func localEnvironment() []string {
	allowed := map[string]bool{"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "TMPDIR": true, "TMP": true, "TEMP": true, "SYSTEMROOT": true, "WINDIR": true, "APPDATA": true, "LOCALAPPDATA": true, "USERPROFILE": true, "LANG": true, "LC_ALL": true, "CODEX_HOME": true, "OPENAI_API_KEY": true, "OPENAI_BASE_URL": true}
	var env []string
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok && allowed[strings.ToUpper(key)] {
			env = append(env, item)
		}
	}
	return env
}

func protocolError(message string) error { return fmt.Errorf("agent process: %s", message) }
