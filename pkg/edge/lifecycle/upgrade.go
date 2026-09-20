package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// UpgradeInstallation replaces only an explicitly selected, verified running
// instance. An empty InstanceID means the known legacy installation, not "all".
func UpgradeInstallation(ctx context.Context, id Identity, source string) error {
	return newInstaller().upgrade(ctx, id, source)
}

func (i installer) upgrade(ctx context.Context, id Identity, source string) (resultErr error) {
	plan, err := InstallationPlan(id)
	if err != nil {
		return err
	}
	id.Executable, id.Config = plan.Executable, plan.Config
	id.PID, err = servicePID(ctx, id, plan, i.run)
	if err != nil || id.PID <= 0 {
		return errors.New("upgrade requires a running, identifiable service")
	}
	verified, err := i.check(ctx, id, i.run)
	if err != nil {
		return err
	}
	if verified != plan {
		return ErrUnsupported
	}
	lock := filepath.Join(filepath.Dir(plan.Executable), ".lifecycle.lock")
	if err = os.Mkdir(lock, 0700); err != nil {
		return errors.New("another lifecycle operation or unfinished attempt requires inspection")
	}
	defer func() { resultErr = errors.Join(resultErr, os.Remove(lock)) }()
	// Recheck under the lock; uninstall uses this same lock.
	verified, err = i.check(ctx, id, i.run)
	if err != nil {
		return err
	}
	if verified != plan {
		return ErrUnsupported
	}
	sums := map[string]string{}
	for _, path := range []string{plan.Executable, plan.Config, plan.ServiceFile} {
		if err = i.validate(path, id.UID); err != nil {
			return err
		}
		sums[path], err = digestFile(path)
		if err != nil {
			return err
		}
	}
	next, backup := plan.Executable+".next", plan.Executable+".previous"
	// Never overwrite a prior recovery artifact, even if it looks obsolete.
	for _, path := range []string{next, backup} {
		if _, err = os.Lstat(path); !os.IsNotExist(err) {
			return errors.New("upgrade recovery files already exist; inspect before retrying")
		}
	}
	if err = copyExclusive(source, next); err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(next); err != nil && !os.IsNotExist(err) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	if err = copyExclusive(plan.Executable, backup); err != nil {
		return err
	}
	retainBackup := false
	defer func() {
		if !retainBackup {
			if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
				resultErr = errors.Join(resultErr, err)
			}
		}
	}()
	backupSum, err := digestFile(backup)
	if err != nil {
		return err
	}
	if backupSum != sums[plan.Executable] {
		return ErrUnsupported
	}
	if err = i.stop(ctx, id, plan, false); err != nil {
		return fmt.Errorf("stop selected service: %w", err)
	}
	// From this point any failure restores the original program and attempts to
	// restart it. A failed rollback retains the verified backup for recovery.
	replaced := false
	success := false
	defer func() {
		if success {
			return
		}
		recovery, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		for _, path := range []string{plan.Config, plan.ServiceFile} {
			if err := i.validate(path, id.UID); err != nil {
				retainBackup = true
				resultErr = errors.Join(resultErr, errors.New("configuration ownership changed; manual recovery required"))
				return
			}
			sum, err := digestFile(path)
			if err != nil || sum != sums[path] {
				retainBackup = true
				resultErr = errors.Join(resultErr, errors.New("configuration changed; manual recovery required"))
				return
			}
		}
		if replaced {
			if err := i.stop(recovery, id, plan, false); err != nil {
				retainBackup = true
				resultErr = errors.Join(resultErr, errors.New("rollback could not stop new service; backup retained"))
				return
			}
			if id.OS == "darwin" {
				if err := waitExecutableGone(recovery, plan.Executable, i.run); err != nil {
					retainBackup = true
					resultErr = errors.Join(resultErr, err)
					return
				}
			}
			if err := i.validate(backup, id.UID); err != nil {
				retainBackup = true
				resultErr = errors.Join(resultErr, err)
				return
			}
			sum, err := digestFile(backup)
			if err != nil || sum != backupSum {
				retainBackup = true
				resultErr = errors.Join(resultErr, errors.New("rollback backup changed; manual recovery required"))
				return
			}
			if err = os.Rename(backup, plan.Executable); err != nil {
				retainBackup = true
				resultErr = errors.Join(resultErr, err)
				return
			}
		}
		if err := i.start(recovery, id, plan, false); err != nil {
			retainBackup = true
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback restart failed: %w", err))
			return
		}
		if err := i.healthy(recovery, id, plan, i.run); err != nil {
			retainBackup = true
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback health check failed: %w", err))
		}
	}()
	if err = waitProcessGone(ctx, id.PID, i.run); err != nil {
		return err
	}
	for path, want := range sums {
		if err = i.validate(path, id.UID); err != nil {
			return err
		}
		got, err := digestFile(path)
		if err != nil {
			return err
		}
		if got != want {
			return errors.New("installation changed during upgrade")
		}
	}
	if err = i.validate(next, id.UID); err != nil {
		return err
	}
	if err = os.Rename(next, plan.Executable); err != nil {
		return err
	}
	replaced = true
	if err = i.start(ctx, id, plan, false); err != nil {
		return fmt.Errorf("new service start failed: %w", err)
	}
	if err = i.healthy(ctx, id, plan, i.run); err != nil {
		return fmt.Errorf("new service health check failed: %w", err)
	}
	success = true
	return nil
}
