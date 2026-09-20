package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	edgeconfig "github.com/liaisonio/liaison/pkg/edge/config"
	"gopkg.in/yaml.v2"
)

func testInstaller(t *testing.T) (installer, Identity, string, *[]string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("test executable"), 0700); err != nil {
		t.Fatal(err)
	}
	calls := []string{}
	i := newInstaller()
	// These tests never invoke a service manager. Temporary ancestors are not
	// installation locations, so directory ownership is exercised separately.
	i.validateDir = func(string, int) error { return nil }
	i.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if strings.Contains(strings.Join(args, " "), "--property=MainPID") {
			return []byte("0"), nil
		}
		return nil, nil
	}
	i.healthy = func(context.Context, Identity, Plan, Command) error { return nil }
	return i, Identity{OS: "linux", UID: 501, Home: filepath.Join(root, "home")}, source, &calls
}

func TestInstallNewIsolated(t *testing.T) {
	for _, platform := range []string{"linux", "darwin"} {
		t.Run(platform, func(t *testing.T) {
			i, id, source, calls := testInstaller(t)
			id.OS = platform
			credentials := InstallCredentials{Manager: "manager.example:443", AccessKey: "key", SecretKey: "secret:#'\""}
			first, err := i.installNew(context.Background(), id, source, credentials)
			if err != nil {
				t.Fatal(err)
			}
			credentials.AccessKey = "different-key"
			second, err := i.installNew(context.Background(), id, source, credentials)
			if err != nil {
				t.Fatal(err)
			}
			if first.InstanceID == second.InstanceID || first.Executable == second.Executable || first.Service == second.Service {
				t.Fatal("instances share targets")
			}
			for _, plan := range []Plan{first, second} {
				data, err := os.ReadFile(plan.Config)
				if err != nil {
					t.Fatal(err)
				}
				var conf edgeconfig.Configuration
				if err = yaml.Unmarshal(data, &conf); err != nil {
					t.Fatal(err)
				}
				if conf.InstanceID != plan.InstanceID || conf.Manager.Auth.SecretKey != credentials.SecretKey || !conf.AllowRemoteUninstall {
					t.Fatalf("unexpected configuration: instance=%s uninstall=%v", conf.InstanceID, conf.AllowRemoteUninstall)
				}
				for _, path := range []string{plan.Config, plan.ServiceFile, manifestPath(plan)} {
					info, err := os.Stat(path)
					if err != nil || info.Mode().Perm() != 0600 {
						t.Fatalf("unsafe permissions for %s", path)
					}
				}
				data, err = os.ReadFile(manifestPath(plan))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), credentials.SecretKey) || strings.Contains(string(data), "manager.example") {
					t.Fatal("manifest leaked credentials")
				}
			}
			if platform == "linux" {
				for _, call := range *calls {
					if !strings.Contains(call, "systemctl --user ") {
						t.Fatalf("not user-scoped: %s", call)
					}
				}
			}
			if _, err = i.installNew(context.Background(), id, source, credentials); err == nil || !strings.Contains(err.Error(), "explicit upgrade") {
				t.Fatalf("duplicate enrollment allowed: %v", err)
			}
			if _, err = os.Stat(first.Executable); err != nil {
				t.Fatal("duplicate attempt damaged other instance", err)
			}
		})
	}
}

func TestInstallFailureCleanup(t *testing.T) {
	i, id, source, calls := testInstaller(t)
	var target Plan
	i.healthy = func(_ context.Context, _ Identity, p Plan, _ Command) error {
		target = p
		return errors.New("startup failed")
	}
	_, err := i.installNew(context.Background(), id, source, InstallCredentials{"manager.example:443", "key", "secret"})
	if err == nil {
		t.Fatal("expected startup failure")
	}
	for _, path := range []string{target.Executable, target.Config, target.ServiceFile, manifestPath(target)} {
		if _, err = os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("failed install left file %s: %v", path, err)
		}
	}
	if !strings.Contains(strings.Join(*calls, "\n"), "--user disable --now "+target.Service) {
		t.Fatal("did not stop exact service before cleanup")
	}
}

func TestInstallStopFailureRetainsFiles(t *testing.T) {
	i, id, source, _ := testInstaller(t)
	var target Plan
	i.healthy = func(_ context.Context, _ Identity, p Plan, _ Command) error {
		target = p
		return errors.New("startup failed")
	}
	i.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "disable") {
			return nil, errors.New("stop failed")
		}
		return nil, nil
	}
	_, err := i.installNew(context.Background(), id, source, InstallCredentials{"manager.example:443", "key", "secret"})
	if err == nil || !strings.Contains(err.Error(), "files retained") {
		t.Fatal(err)
	}
	for _, path := range []string{target.Executable, target.Config, target.ServiceFile} {
		if _, err = os.Stat(path); err != nil {
			t.Fatal("deleted potentially running installation", err)
		}
	}
}

func TestInstallUnavailableDoesNotWrite(t *testing.T) {
	i, id, source, _ := testInstaller(t)
	i.run = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("no user bus") }
	if _, err := i.installNew(context.Background(), id, source, InstallCredentials{"manager.example:443", "key", "secret"}); err == nil {
		t.Fatal("expected failure")
	}
	if _, err := os.Lstat(id.Home); !os.IsNotExist(err) {
		t.Fatal("wrote before preflight")
	}
}

func TestExclusiveFilesPreserveExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing")
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("new"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := exclusiveFile(path, []byte("replace"), 0600); !os.IsExist(err) {
		t.Fatal(err)
	}
	if err := copyExclusive(source, path); !os.IsExist(err) {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatal("existing file changed", err)
	}
}

func TestUserInstallationPlan(t *testing.T) {
	id := Identity{OS: "linux", UID: 1000, Home: "/home/alice", InstanceID: strings.Repeat("a", 32)}
	p, err := InstallationPlan(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "systemd-user" || !strings.HasPrefix(p.Config, "/home/alice/.local/share/liaison/edges/") || !strings.HasPrefix(p.ServiceFile, "/home/alice/.config/systemd/user/") {
		t.Fatalf("unexpected layout: %+v", p)
	}
	unit, err := renderService(id, p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(unit), "WorkingDirectory=\"") {
		t.Fatal("systemd WorkingDirectory must not contain literal quotes")
	}
	id.InstanceID = ""
	if _, err = InstallationPlan(id); err == nil {
		t.Fatal("unknown legacy user install accepted")
	}
	id.InstanceID = strings.Repeat("b", 32)
	id.Home = "/home/user%h"
	p, err = InstallationPlan(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = renderService(id, p); err == nil {
		t.Fatal("systemd specifier accepted")
	}
}

func TestEnrollmentInput(t *testing.T) {
	for _, input := range []string{"", "host:443\nkey\n", "host:443\nkey\nsecret\nextra\n", "host:443\nkey\n" + strings.Repeat("x", 5000), "host\nkey\nsecret\n"} {
		if _, err := readEnrollment(strings.NewReader(input)); err == nil {
			t.Fatal("invalid enrollment accepted")
		}
	}
	c, err := readEnrollment(strings.NewReader("host:443\nkey\nsecret:#'\"\n"))
	if err != nil || c.SecretKey != "secret:#'\"" {
		t.Fatal("enrollment roundtrip failed")
	}
}

func TestInstallRemoteUninstallEnabledByDefault(t *testing.T) {
	i, id, source, _ := testInstaller(t)
	plan, err := i.installNew(context.Background(), id, source, InstallCredentials{"manager.example:443", "key", "secret"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(plan.Config)
	if err != nil {
		t.Fatal(err)
	}
	var conf edgeconfig.Configuration
	if err = yaml.Unmarshal(data, &conf); err != nil {
		t.Fatal(err)
	}
	if !conf.AllowRemoteUninstall {
		t.Fatal("default remote uninstall setting was not persisted")
	}
}
