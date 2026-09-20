package lifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Command func(context.Context, string, ...string) ([]byte, error)

// Probe performs read-only native-service checks. It never runs sudo, stop,
// bootout, rm or an installation/uninstallation script.
func Probe(ctx context.Context, id Identity, run Command) (Plan, error) {
	var svc Service
	if id.InstanceID != "" && !instancePattern.MatchString(id.InstanceID) {
		return Plan{}, ErrUnsupported
	}
	switch id.OS {
	case "linux":
		name := "liaison-edge.service"
		if id.InstanceID != "" {
			name = "liaison-edge-" + id.InstanceID + ".service"
		}
		args := []string{"show", name, "--no-pager", "--property=MainPID,FragmentPath,DropInPaths,ExecStart"}
		if id.UID != 0 {
			args = append([]string{"--user"}, args...)
		}
		data, err := run(ctx, "/usr/bin/systemctl", args...)
		if err != nil {
			return Plan{}, fmt.Errorf("inspect systemd service: %w", err)
		}
		props := map[string]string{}
		for _, line := range strings.Split(string(data), "\n") {
			k, v, ok := strings.Cut(line, "=")
			if ok {
				props[k] = v
			}
		}
		pid, err := strconv.Atoi(props["MainPID"])
		if err != nil || props["DropInPaths"] != "" {
			return Plan{}, ErrUnsupported
		}
		// Accept the canonical, simple ExecStart only. Wrappers, extra commands,
		// drop-ins and unrecognized systemd output are intentionally unsupported.
		expected := "argv[]=" + id.Executable + " -c " + id.Config + " ;"
		if !strings.Contains(props["ExecStart"], expected) || strings.Count(props["ExecStart"], "argv[]=") != 1 {
			return Plan{}, ErrUnsupported
		}
		svc = Service{Name: name, File: props["FragmentPath"], PID: pid, Arguments: []string{id.Executable, "-c", id.Config}}
	case "darwin":
		if id.UID <= 0 {
			return Plan{}, ErrUnsupported
		}
		name := "com.liaison.edge"
		if id.InstanceID != "" {
			name += "." + id.InstanceID
		}
		file := filepath.Join(id.Home, "Library/LaunchAgents", name+".plist")
		data, err := run(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", file)
		if err != nil {
			return Plan{}, fmt.Errorf("inspect launch agent: %w", err)
		}
		var plist struct {
			Label            string
			Program          string
			ProgramArguments []string
		}
		if json.Unmarshal(data, &plist) != nil || plist.Label != name || plist.Program != "" {
			return Plan{}, ErrUnsupported
		}
		data, err = run(ctx, "/bin/launchctl", "print", fmt.Sprintf("gui/%d/%s", id.UID, name))
		if err != nil {
			return Plan{}, fmt.Errorf("inspect launchd job: %w", err)
		}
		matches := regexp.MustCompile(`(?m)^\s*pid = ([0-9]+)\s*$`).FindAllSubmatch(data, -1)
		if len(matches) != 1 {
			return Plan{}, ErrUnsupported
		}
		pid, err := strconv.Atoi(string(matches[0][1]))
		if err != nil {
			return Plan{}, ErrUnsupported
		}
		svc = Service{Name: name, File: file, PID: pid, Arguments: plist.ProgramArguments}
	default:
		return Plan{}, ErrUnsupported
	}
	return BuildPlan(id, svc)
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1024*1024 {
		return 0, fmt.Errorf("service output exceeds limit")
	}
	return b.Buffer.Write(p)
}

func NativeCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out limitedBuffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func RuntimeIdentity(configPath, instance string) (Identity, error) {
	exe, err := os.Executable()
	if err != nil {
		return Identity{}, err
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return Identity{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Identity{}, err
	}
	return Identity{OS: runtime.GOOS, UID: os.Geteuid(), PID: os.Getpid(), Home: home, Executable: exe, Config: configPath, InstanceID: instance}, nil
}

// Check rejects both service aliases and other live processes using the same
// executable. Lack of permission to inspect a candidate is not proof of safety.
func Check(ctx context.Context, id Identity, run Command) (Plan, error) {
	plan, err := Probe(ctx, id, run)
	if err != nil {
		return Plan{}, err
	}
	if err = ValidateFiles(plan, id.UID); err != nil {
		return Plan{}, err
	}
	if err = checkShared(ctx, id, plan, run); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func checkShared(ctx context.Context, id Identity, plan Plan, run Command) error {
	// A verified, private user installation is managed within its owner's
	// service domain. Legacy/shared layouts still require the global scan.
	privateUser := false
	if (id.OS == "darwin" || plan.Kind == "systemd-user") && !plan.Legacy {
		if err := validatePrivateInstallation(id, plan); err != nil {
			return err
		}
		privateUser = true
	}
	count := 0
	if id.OS == "linux" {
		entries, err := os.ReadDir("/proc")
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if _, err := strconv.Atoi(entry.Name()); err != nil {
				continue
			}
			if privateUser {
				info, err := os.Stat(filepath.Join("/proc", entry.Name()))
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return err
				}
				// Other users' processes are outside the private user service domain.
				// Permission errors for an in-scope process remain fatal below.
				if !ownedDirectory(info, id.UID) {
					continue
				}
			}
			path, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				if privateUser && os.IsPermission(err) && verifiedUserManager(ctx, id, entry.Name(), run, os.ReadFile, validateSystemProgram) {
					continue
				}
				return fmt.Errorf("inspect process executable: %w", err)
			}
			if path == plan.Executable {
				count++
			}
		}
	} else {
		data, err := run(ctx, "/bin/ps", "-axo", "pid=,comm=")
		if err != nil {
			return fmt.Errorf("inspect shared executable: %w", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			i := strings.IndexAny(line, " \t")
			if i < 0 {
				continue
			}
			if strings.TrimSpace(line[i:]) == plan.Executable {
				count++
			}
		}
	}
	if count != 1 {
		return ErrUnsupported
	}
	dirs := []string{"/etc/systemd/system", "/run/systemd/system", "/usr/lib/systemd/system", "/lib/systemd/system"}
	if plan.Kind == "systemd-user" {
		dirs = []string{filepath.Join(id.Home, ".config/systemd/user"), filepath.Join(id.Home, ".local/share/systemd/user"), fmt.Sprintf("/run/user/%d/systemd/user", id.UID), "/etc/systemd/user", "/usr/local/lib/systemd/user", "/usr/lib/systemd/user", "/lib/systemd/user"}
	}
	if id.OS == "darwin" {
		dirs = []string{filepath.Join(id.Home, "Library/LaunchAgents"), "/Library/LaunchAgents", "/Library/LaunchDaemons"}
		if privateUser {
			// /Library/LaunchAgents can also populate this user's GUI domain;
			// only system LaunchDaemons are outside the user installation boundary.
			dirs = dirs[:2]
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect sibling services: %w", err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			path := filepath.Join(dir, entry.Name())
			if path == plan.ServiceFile || entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(path, ".service") && !strings.HasSuffix(path, ".plist") {
				continue
			}
			// Service aliases pointing to our unit are a shared ownership ambiguity.
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			if resolved == plan.ServiceFile {
				return ErrUnsupported
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if bytes.Contains(data, []byte(plan.Executable)) || bytes.Contains(data, []byte(plan.Config)) {
				return ErrUnsupported
			}
			// Binary plists must be decoded before comparing their references.
			if id.OS == "darwin" && bytes.HasPrefix(data, []byte("bplist")) {
				decoded, err := run(ctx, "/usr/bin/plutil", "-convert", "xml1", "-o", "-", path)
				if err != nil {
					return err
				}
				if bytes.Contains(decoded, []byte(plan.Executable)) || bytes.Contains(decoded, []byte(plan.Config)) {
					return ErrUnsupported
				}
			}
		}
	}
	return nil
}
