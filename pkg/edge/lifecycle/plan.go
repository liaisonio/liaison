// Package lifecycle validates the local ownership boundary before any Edge
// lifecycle operation. It never treats a process name as proof of ownership.
package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ErrUnsupported = errors.New("installation cannot be safely managed remotely")

var instancePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Identity contains local runtime facts, not fields supplied by a remote caller.
type Identity struct {
	OS         string
	UID        int
	PID        int
	Home       string
	Executable string
	Config     string
	InstanceID string
}

// Service describes facts read from the native service manager.
type Service struct {
	Name      string
	File      string
	PID       int
	Arguments []string
}

// Plan is deliberately file-specific. Shared parent directories are never targets.
type Plan struct {
	InstanceID  string `json:"instance_id"`
	Service     string `json:"service"`
	ServiceFile string `json:"service_file"`
	Executable  string `json:"executable"`
	Config      string `json:"config"`
	Kind        string `json:"kind"`
	Legacy      bool   `json:"legacy"`
}

func absoluteClean(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && !strings.ContainsAny(path, "\x00\r\n")
}

// BuildPlan is pure: neither probes nor changes the machine. Only known layouts
// are accepted; arbitrary custom deployments remain usable but are not uninstallable.
func BuildPlan(id Identity, svc Service) (Plan, error) {
	plan, err := InstallationPlan(id)
	if err != nil {
		return Plan{}, err
	}
	if id.PID <= 0 || svc.PID != id.PID || id.Executable != plan.Executable || id.Config != plan.Config || svc.Name != plan.Service || svc.File != plan.ServiceFile {
		return Plan{}, ErrUnsupported
	}
	if len(svc.Arguments) != 3 || svc.Arguments[0] != id.Executable || svc.Arguments[1] != "-c" || svc.Arguments[2] != id.Config {
		return Plan{}, ErrUnsupported
	}
	return plan, nil
}

// InstallationPlan computes only fixed local layouts. It does not establish
// ownership or authorize uninstall; BuildPlan and Check provide those gates.
func InstallationPlan(id Identity) (Plan, error) {
	fail := func() (Plan, error) { return Plan{}, ErrUnsupported }
	if id.InstanceID != "" && !instancePattern.MatchString(id.InstanceID) {
		return fail()
	}
	var executable, config, service, file, kind string
	switch id.OS {
	case "linux":
		kind = "systemd"
		service = "liaison-edge.service"
		executable = "/usr/local/bin/liaison-edge"
		config = "/etc/liaison/liaison-edge.yaml"
		if id.InstanceID != "" {
			service = "liaison-edge-" + id.InstanceID + ".service"
			executable = filepath.Join("/opt/liaison/edges", id.InstanceID, "bin/liaison-edge")
			config = filepath.Join("/etc/liaison/edges", id.InstanceID, "config.yaml")
		}
		file = filepath.Join("/etc/systemd/system", service)
		if id.UID != 0 {
			if id.UID < 0 || id.InstanceID == "" || !absoluteClean(id.Home) || id.Home == "/" {
				return fail()
			}
			kind = "systemd-user"
			base := filepath.Join(id.Home, ".local/share/liaison/edges", id.InstanceID)
			executable = filepath.Join(base, "bin/liaison-edge")
			config = filepath.Join(base, "config.yaml")
			file = filepath.Join(id.Home, ".config/systemd/user", service)
		}
	case "darwin":
		if id.UID <= 0 || !absoluteClean(id.Home) || id.Home == "/" {
			return fail()
		}
		kind = "launchd"
		service = "com.liaison.edge"
		base := filepath.Join(id.Home, "Library/Application Support/liaison")
		if id.InstanceID != "" {
			service += "." + id.InstanceID
			base = filepath.Join(base, "edges", id.InstanceID)
		}
		executable = filepath.Join(base, "bin/liaison-edge")
		config = filepath.Join(base, "liaison-edge.yaml")
		if id.InstanceID != "" {
			config = filepath.Join(base, "config.yaml")
		}
		file = filepath.Join(id.Home, "Library/LaunchAgents", service+".plist")
	default:
		return fail()
	}
	if !absoluteClean(executable) || !absoluteClean(config) || !absoluteClean(file) {
		return fail()
	}
	instance := id.InstanceID
	if instance == "" {
		// Stable local binding for a verified legacy installation. This is not an
		// authorization credential, and contains no key or user-provided path.
		sum := sha256.Sum256([]byte(kind + "\x00" + file + "\x00" + executable + "\x00" + config))
		instance = "legacy-" + hex.EncodeToString(sum[:16])
	}
	return Plan{InstanceID: instance, Service: service, ServiceFile: file, Executable: executable, Config: config, Kind: kind, Legacy: id.InstanceID == ""}, nil
}

// ValidateFiles rejects symlinks at every component, non-regular leaves, hard
// links and group/world-writable paths. Runtime permission checks are conservative.
func ValidateFiles(plan Plan, uid int) error {
	for _, path := range []string{plan.ServiceFile, plan.Executable, plan.Config} {
		if err := validateFile(path, uid); err != nil {
			return fmt.Errorf("validate installation ownership: %w", err)
		}
	}
	return nil
}

func validateFile(path string, uid int) error {
	if !absoluteClean(path) {
		return ErrUnsupported
	}
	current := path
	leaf := true
	for {
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect installation path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
			return ErrUnsupported
		}
		if leaf && !info.Mode().IsRegular() {
			return ErrUnsupported
		}
		if !leaf && !info.IsDir() {
			return ErrUnsupported
		}
		if !ownedFile(info, uid, leaf) {
			return ErrUnsupported
		}
		if current == string(filepath.Separator) {
			break
		}
		current = filepath.Dir(current)
		leaf = false
	}
	return nil
}
