//go:build darwin || linux

package webide

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func canLaunch() bool              { return os.Geteuid() != 0 }
func prepareProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func privateDirectory(path string) error {
	i, err := os.Lstat(path)
	if err != nil {
		return err
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	if !ok || !i.IsDir() || s.Uid != uint32(os.Geteuid()) || i.Mode().Perm()&0077 != 0 {
		return errors.New("IDE state requires a private owned directory")
	}
	return nil
}
func trustedProgram(path string) error {
	i, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !i.Mode().IsRegular() || i.Mode().Perm()&0111 == 0 {
		return errors.New("IDE executable invalid")
	}
	for p := path; ; p = filepath.Dir(p) {
		i, err = os.Stat(p)
		if err != nil {
			return err
		}
		s, ok := i.Sys().(*syscall.Stat_t)
		if !ok || (s.Uid != 0 && s.Uid != uint32(os.Geteuid())) || i.Mode().Perm()&0022 != 0 {
			return errors.New("IDE executable path is not trusted")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}
func stopProcess(ctx context.Context, r *record) error {
	if !alive(r) {
		return nil
	}
	// A recorded start time and exact profile argument are required before any
	// signal, protecting against PID reuse after reconnect/restart.
	group, err := syscall.Getpgid(int(r.PID))
	if err != nil {
		return err
	}
	if group != int(r.PID) {
		return errors.New("IDE process group changed")
	}
	if err = syscall.Kill(-group, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return err
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if !alive(r) {
				return nil
			}
		case <-timer.C:
			if alive(r) {
				if err = syscall.Kill(-group, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
					return err
				}
			}
			return nil
		}
	}
}
