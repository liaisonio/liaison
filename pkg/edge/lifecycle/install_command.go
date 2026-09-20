package lifecycle

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
)

// RunInstallCommand is an explicit local administrative entry point. Enrollment
// arrives on stdin, not the installed service's argv or environment.
func RunInstallCommand(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) != 1 && len(args) != 2 {
		return errors.New("use --edge-install-new or --edge-upgrade-instance <instance-id|legacy>")
	}
	newInstall := args[0] == "--edge-install-new" && (len(args) == 1 || args[1] == "--allow-remote-uninstall")
	upgrade := args[0] == "--edge-upgrade-instance" && len(args) == 2
	if !newInstall && !upgrade {
		return errors.New("invalid installation command")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	source, err := os.Executable()
	if err != nil {
		return err
	}
	id := Identity{OS: runtime.GOOS, UID: os.Geteuid(), Home: home}
	if upgrade {
		if args[1] != "legacy" {
			if !instancePattern.MatchString(args[1]) {
				return errors.New("invalid instance ID")
			}
			id.InstanceID = args[1]
		}
		if err = UpgradeInstallation(ctx, id, source); err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, "Selected Edge instance upgraded; configuration preserved.")
		return err
	}
	c, err := readEnrollment(input)
	if err != nil {
		return err
	}
	installer := newInstaller()
	plan, err := installer.installNew(ctx, id, source, c)
	if err != nil {
		return err
	}
	state := "disabled"
	if installer.allowRemoteUninstall {
		state = "enabled, subject to local ownership checks"
	}
	_, err = fmt.Fprintf(output, "Instance: %s\nService: %s\nConfiguration: %s\nRemote uninstall: %s\n", plan.InstanceID, plan.Service, plan.Config, state)
	return err
}

func readEnrollment(input io.Reader) (InstallCredentials, error) {
	scanner := bufio.NewScanner(io.LimitReader(input, 16385))
	scanner.Buffer(make([]byte, 1024), 4098)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 3 {
			return InstallCredentials{}, errors.New("expected exactly three enrollment fields")
		}
	}
	if scanner.Err() != nil || len(lines) != 3 {
		return InstallCredentials{}, errors.New("invalid enrollment input")
	}
	c := InstallCredentials{lines[0], lines[1], lines[2]}
	return c, validateCredentials(c)
}
