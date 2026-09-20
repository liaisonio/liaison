package lifecycle

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	edgeconfig "github.com/liaisonio/liaison/pkg/edge/config"
	"gopkg.in/yaml.v2"
)

// InstallManifest is local bookkeeping, not authority. Paths are always derived
// from the fixed layout and independently checked. No credentials are stored here.
type InstallManifest struct {
	Version    int    `json:"version"`
	InstanceID string `json:"instance_id"`
	Platform   string `json:"platform"`
	UID        int    `json:"uid"`
	Enrollment string `json:"enrollment_fingerprint"`
	Service    string `json:"service"`
}

type InstallCredentials struct{ Manager, AccessKey, SecretKey string }

type installer struct {
	allowRemoteUninstall bool
	run                  Command
	validateDir          func(string, int) error
	check                func(context.Context, Identity, Command) (Plan, error)
	healthy              func(context.Context, Identity, Plan, Command) error
	validate             func(string, int) error
}

func newInstaller() installer {
	return installer{allowRemoteUninstall: true, run: NativeCommand, validateDir: validateDirectory, check: Check, healthy: waitServiceStable, validate: validateFile}
}

func validateDirectory(path string, uid int) error {
	if !absoluteClean(path) {
		return ErrUnsupported
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || !ownedFile(info, uid, false) {
			return ErrUnsupported
		}
		if current == string(filepath.Separator) {
			return nil
		}
	}
}

// ensureDirectory verifies existing ancestors before mkdir; it never chmods an
// existing directory or follows a user-supplied symlink to make an install fit.
func (i installer) ensureDirectory(path string, uid int) error {
	_, err := os.Lstat(path)
	if err == nil {
		return i.validateDir(path, uid)
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err = i.ensureDirectory(filepath.Dir(path), uid); err != nil {
		return err
	}
	if err = os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	return i.validateDir(path, uid)
}

func exclusiveFile(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	err = errors.Join(writeErr, syncErr, closeErr)
	if err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

func copyExclusive(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return ErrUnsupported
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	_, writeErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	err = errors.Join(writeErr, syncErr, closeErr)
	if err != nil {
		return errors.Join(err, os.Remove(target))
	}
	return nil
}

func enrollmentFingerprint(manager, key string) string {
	sum := sha256.Sum256([]byte(manager + "\x00" + key))
	return hex.EncodeToString(sum[:])
}

func validateCredentials(c InstallCredentials) error {
	host, port, err := net.SplitHostPort(c.Manager)
	if err != nil || host == "" {
		return errors.New("invalid Manager host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("invalid Manager port")
	}
	for _, s := range []string{c.Manager, c.AccessKey, c.SecretKey} {
		if s == "" || len(s) > 4096 || strings.ContainsAny(s, "\r\n\x00") {
			return errors.New("invalid installation credentials")
		}
	}
	return nil
}

func installationBase(plan Plan) string { return filepath.Dir(filepath.Dir(plan.Executable)) }
func manifestPath(plan Plan) string     { return filepath.Join(installationBase(plan), "install.json") }

func renderService(id Identity, plan Plan) ([]byte, error) {
	// systemd performs its own percent/environment expansion, even without a shell.
	if strings.ContainsAny(plan.Executable+plan.Config+plan.ServiceFile, "\x00\r\n%$\\\"") {
		return nil, ErrUnsupported
	}
	base := installationBase(plan)
	if plan.Kind == "launchd" {
		esc := func(s string) string {
			var b bytes.Buffer // Strings originate locally, XML is only emitted, never parsed here.
			if err := xml.EscapeText(&b, []byte(s)); err != nil {
				return ""
			}
			return b.String()
		}
		return []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>Label</key><string>` + esc(plan.Service) + `</string><key>ProgramArguments</key><array><string>` + esc(plan.Executable) + `</string><string>-c</string><string>` + esc(plan.Config) + `</string></array><key>WorkingDirectory</key><string>` + esc(base) + `</string><key>RunAtLoad</key><true/><key>KeepAlive</key><true/></dict></plist>`), nil
	}
	target := "multi-user.target"
	if plan.Kind == "systemd-user" {
		target = "default.target"
	}
	return []byte("[Unit]\nDescription=Liaison Edge instance " + plan.InstanceID + "\nAfter=network.target\n[Service]\nType=simple\nWorkingDirectory=" + base + "\nExecStart=" + strconv.Quote(plan.Executable) + " -c " + strconv.Quote(plan.Config) + "\nRestart=always\nRestartSec=5s\nUMask=0077\n[Install]\nWantedBy=" + target + "\n"), nil
}

func (i installer) start(ctx context.Context, id Identity, plan Plan, enable bool) error {
	if plan.Kind == "launchd" {
		_, err := i.run(ctx, "/bin/launchctl", "bootstrap", fmt.Sprintf("gui/%d", id.UID), plan.ServiceFile)
		return err
	}
	if _, err := i.run(ctx, "/usr/bin/systemctl", systemdArgs(plan, "daemon-reload")...); err != nil {
		return err
	}
	args := []string{"start", plan.Service}
	if enable {
		args = []string{"enable", "--now", plan.Service}
	}
	_, err := i.run(ctx, "/usr/bin/systemctl", systemdArgs(plan, args...)...)
	return err
}

func (i installer) stop(ctx context.Context, id Identity, plan Plan, disable bool) error {
	if plan.Kind == "launchd" {
		_, err := i.run(ctx, "/bin/launchctl", "bootout", fmt.Sprintf("gui/%d/%s", id.UID, plan.Service))
		return err
	}
	pid, err := servicePID(ctx, id, plan, i.run)
	if err != nil {
		return fmt.Errorf("inspect service before stopping: %w", err)
	}
	args := []string{"stop", plan.Service}
	if disable {
		args = []string{"disable", "--now", plan.Service}
	}
	if _, err = i.run(ctx, "/usr/bin/systemctl", systemdArgs(plan, args...)...); err != nil {
		return err
	}
	if pid > 0 {
		return waitProcessGone(ctx, pid, i.run)
	}
	return nil
}

func waitServiceStable(ctx context.Context, id Identity, plan Plan, run Command) error {
	first, err := servicePID(ctx, id, plan, run)
	if err != nil {
		return err
	}
	if first <= 0 {
		return errors.New("service did not start")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
	}
	second, err := servicePID(ctx, id, plan, run)
	if err != nil {
		return err
	}
	if first != second {
		return errors.New("service restarted during startup")
	}
	return nil
}

func (i installer) rejectDuplicate(base string, id Identity, c InstallCredentials) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !instancePattern.MatchString(entry.Name()) {
			continue
		}
		other := id
		other.InstanceID = entry.Name()
		plan, err := InstallationPlan(other)
		if err != nil {
			return err
		}
		// Config, not a stale manifest hash, remains authoritative after key rotation.
		if err = i.validateDir(filepath.Dir(plan.Config), id.UID); err != nil {
			return err
		}
		data, err := os.ReadFile(plan.Config)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var conf edgeconfig.Configuration
		if yaml.Unmarshal(data, &conf) != nil {
			return errors.New("existing instance configuration cannot be verified")
		}
		for _, addr := range conf.Manager.Dial.Addrs {
			if addr == c.Manager && conf.Manager.Auth.AccessKey == c.AccessKey {
				return fmt.Errorf("this connector is already installed as instance %s; use an explicit upgrade", entry.Name())
			}
		}
	}
	// Legacy installs are retained. Never silently install the same enrollment a second time.
	legacy := id
	legacy.InstanceID = ""
	if plan, err := InstallationPlan(legacy); err == nil {
		data, err := os.ReadFile(plan.Config)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			var conf edgeconfig.Configuration
			if yaml.Unmarshal(data, &conf) != nil {
				return errors.New("legacy configuration cannot be verified")
			}
			for _, addr := range conf.Manager.Dial.Addrs {
				if addr == c.Manager && conf.Manager.Auth.AccessKey == c.AccessKey {
					return errors.New("this connector has a legacy installation; use an explicit legacy upgrade")
				}
			}
		}
	}
	return nil
}

// InstallNew always creates a random, exclusive instance. It neither replaces a
// legacy installation nor enables remote uninstall as a side effect.
func InstallNew(ctx context.Context, id Identity, source string, c InstallCredentials) (Plan, error) {
	return newInstaller().installNew(ctx, id, source, c)
}

func (i installer) installNew(ctx context.Context, id Identity, source string, c InstallCredentials) (result Plan, resultErr error) {
	if err := validateCredentials(c); err != nil {
		return Plan{}, err
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Plan{}, err
	}
	id.InstanceID = hex.EncodeToString(raw[:])
	plan, err := InstallationPlan(id)
	if err != nil {
		return Plan{}, err
	}
	service, err := renderService(id, plan)
	if err != nil {
		return Plan{}, err
	}
	if plan.Kind == "launchd" {
		_, err = i.run(ctx, "/bin/launchctl", "print", fmt.Sprintf("gui/%d", id.UID))
	} else {
		_, err = i.run(ctx, "/usr/bin/systemctl", systemdArgs(plan, "show-environment")...)
	}
	if err != nil {
		return Plan{}, fmt.Errorf("service manager is unavailable for this installation identity: %w", err)
	}
	base := installationBase(plan)
	registry := filepath.Dir(base)
	if err = i.ensureDirectory(registry, id.UID); err != nil {
		return Plan{}, err
	}
	lock := filepath.Join(registry, ".install.lock")
	if err = os.Mkdir(lock, 0700); err != nil {
		return Plan{}, fmt.Errorf("installation is locked; inspect the prior attempt before retrying: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, os.Remove(lock)) }()
	if err = i.rejectDuplicate(registry, id, c); err != nil {
		return Plan{}, err
	}
	created := []string{}
	started := false
	success := false
	defer func() {
		if success {
			return
		}
		if started {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := i.stop(cleanupCtx, id, plan, true); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("stop failed; installation files retained: %w", err))
				return
			}
			if id.OS == "darwin" {
				if err := waitExecutableGone(cleanupCtx, plan.Executable, i.run); err != nil {
					resultErr = errors.Join(resultErr, err)
					return
				}
			}
		}
		for n := len(created) - 1; n >= 0; n-- {
			if err := os.Remove(created[n]); err != nil && !os.IsNotExist(err) {
				resultErr = errors.Join(resultErr, err)
			}
		}
	}()
	// Parent directories may be shared, so only newly created leaves are cleanup targets.
	for _, dir := range []string{base, filepath.Dir(plan.Config), filepath.Dir(plan.Executable), filepath.Join(base, "logs"), filepath.Dir(plan.ServiceFile)} {
		if err = i.ensureDirectory(dir, id.UID); err != nil {
			return Plan{}, err
		}
	}
	conf := edgeconfig.Configuration{InstanceID: id.InstanceID, AllowRemoteUninstall: i.allowRemoteUninstall, Log: edgeconfig.Log{Level: "info", File: filepath.Join(base, "logs/liaison-edge.log"), MaxSize: 100, MaxRolls: 10}}
	conf.Manager.Dial.Addrs = []string{c.Manager}
	conf.Manager.Dial.Network = "tcp"
	conf.Manager.Dial.TLS.Enable = true
	conf.Manager.Dial.TLS.InsecureSkipVerify = true
	conf.Manager.Auth = edgeconfig.Auth{AccessKey: c.AccessKey, SecretKey: c.SecretKey}
	configData, err := yaml.Marshal(conf)
	if err != nil {
		return Plan{}, err
	}
	manifest, err := json.Marshal(InstallManifest{Version: 1, InstanceID: id.InstanceID, Platform: id.OS, UID: id.UID, Service: plan.Service, Enrollment: enrollmentFingerprint(c.Manager, c.AccessKey)})
	if err != nil {
		return Plan{}, err
	}
	// Refuse any pre-existing destination, including symlinks and partial attempts.
	for _, file := range []string{plan.Executable, plan.Config, plan.ServiceFile, manifestPath(plan)} {
		if _, err := os.Lstat(file); !os.IsNotExist(err) {
			return Plan{}, errors.New("installation target already exists or cannot be inspected")
		}
	}
	if err = copyExclusive(source, plan.Executable); err != nil {
		return Plan{}, err
	}
	created = append(created, plan.Executable)
	for _, item := range []struct {
		path string
		data []byte
	}{{plan.Config, configData}, {manifestPath(plan), manifest}, {plan.ServiceFile, service}} {
		if err = exclusiveFile(item.path, item.data, 0600); err != nil {
			return Plan{}, err
		}
		created = append(created, item.path)
	}
	started = true
	if err = i.start(ctx, id, plan, true); err != nil {
		return Plan{}, err
	}
	if err = i.healthy(ctx, id, plan, i.run); err != nil {
		return Plan{}, err
	}
	success = true
	return plan, nil
}
