package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// systemd --user may disable process inspection even for its own user. Exempt
// only the service manager or its direct PAM helper in the manager's init.scope,
// proven through system service metadata and a root-owned systemd executable.
// A process name or a generic permission error is never sufficient evidence.
func verifiedUserManager(ctx context.Context, id Identity, pid string, run Command, read func(string) ([]byte, error), validate func(string, int) error) bool {
	if id.OS != "linux" || id.UID <= 0 {
		return false
	}
	if n, err := strconv.Atoi(pid); err != nil || n <= 0 {
		return false
	}
	data, err := run(ctx, "/usr/bin/systemctl", "show", fmt.Sprintf("user@%d.service", id.UID), "--property=MainPID,ExecStart,ControlGroup")
	if err != nil {
		return false
	}
	props := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok {
			props[k] = v
		}
	}
	managerPID, err := strconv.Atoi(props["MainPID"])
	if err != nil || managerPID <= 0 {
		return false
	}
	wantGroup := fmt.Sprintf("/user.slice/user-%d.slice/user@%d.service", id.UID, id.UID)
	if props["ControlGroup"] != wantGroup || strings.Count(props["ExecStart"], "argv[]=") != 1 {
		return false
	}
	trustedProgram := false
	for _, program := range []string{"/usr/lib/systemd/systemd", "/lib/systemd/systemd"} {
		if strings.Contains(props["ExecStart"], "path="+program+" ;") && strings.Contains(props["ExecStart"], "argv[]="+program+" --user ;") {
			if validate(program, 0) == nil {
				trustedProgram = true
			}
		}
	}
	if !trustedProgram {
		return false
	}
	data, err = read(filepath.Join("/proc", pid, "cgroup"))
	if err != nil {
		return false
	}
	inManagerScope := false
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && (parts[0] == "0" && parts[1] == "" || parts[1] == "name=systemd") && parts[2] == wantGroup+"/init.scope" {
			inManagerScope = true
		}
	}
	if !inManagerScope {
		return false
	}
	if pid == strconv.Itoa(managerPID) {
		return true
	}
	data, err = read(filepath.Join("/proc", pid, "status"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "PPid:"); ok {
			return strings.TrimSpace(value) == strconv.Itoa(managerPID)
		}
	}
	return false
}

func validateSystemProgram(path string, uid int) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	return validateFile(resolved, uid)
}
