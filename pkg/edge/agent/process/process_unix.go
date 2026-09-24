//go:build darwin || linux

package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func validateProgram(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return protocolError("program is not executable")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err = os.Stat(current)
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || (stat.Uid != 0 && stat.Uid != uint32(os.Geteuid())) {
			return protocolError("program ownership is not trusted")
		}
		if info.Mode().Perm()&0002 != 0 {
			return protocolError("world-writable program path")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return nil
}

func prepare(cmd *exec.Cmd) (func() error, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	return func() error {
		// Reap descendants still holding the owned process group after the
		// protocol leader exits. Never enumerate or signal external processes.
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}, nil
}
func afterStart(*exec.Cmd) error { return nil }
