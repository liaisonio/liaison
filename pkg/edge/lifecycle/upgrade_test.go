package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeSelectedInstance(t *testing.T) {
	for _, failNew := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "rollback"}[failNew], func(t *testing.T) {
			i, id, source, _ := testInstaller(t)
			p, err := i.installNew(context.Background(), id, source, InstallCredentials{"manager.example:443", "key", "secret"})
			if err != nil {
				t.Fatal(err)
			}
			other, err := i.installNew(context.Background(), id, source, InstallCredentials{"manager.example:443", "other", "other-secret"})
			if err != nil {
				t.Fatal(err)
			}
			before := map[string]string{}
			for _, path := range []string{p.Config, p.ServiceFile, other.Config, other.ServiceFile, other.Executable} {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				before[path] = string(data)
			}
			if err = os.WriteFile(source, []byte("new executable"), 0700); err != nil {
				t.Fatal(err)
			}
			id.InstanceID = p.InstanceID
			i.check = func(_ context.Context, got Identity, _ Command) (Plan, error) {
				if got.InstanceID != p.InstanceID || got.PID != 123 {
					return Plan{}, ErrUnsupported
				}
				return p, nil
			}
			i.validate = func(string, int) error { return nil }
			i.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				if strings.Contains(joined, other.Service) {
					t.Fatal("touched other service")
				}
				if strings.Contains(joined, "--property=MainPID") {
					return []byte("123\n"), nil
				}
				return nil, nil
			}
			healthChecks := 0
			i.healthy = func(context.Context, Identity, Plan, Command) error {
				healthChecks++
				if failNew && healthChecks == 1 {
					return errors.New("new binary crashed")
				}
				return nil
			}
			err = i.upgrade(context.Background(), id, source)
			if failNew && err == nil || !failNew && err != nil {
				t.Fatalf("unexpected result: %v", err)
			}
			want := "new executable"
			if failNew {
				want = "test executable"
				if healthChecks != 2 {
					t.Fatal("did not verify rollback")
				}
			}
			data, err := os.ReadFile(p.Executable)
			if err != nil || string(data) != want {
				t.Fatalf("binary: %q %v", data, err)
			}
			for path, want := range before {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != want {
					t.Fatalf("modified preserved file: %s", path)
				}
			}
			for _, path := range []string{p.Executable + ".next", p.Executable + ".previous"} {
				if _, err = os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("unexpected leftover %s", path)
				}
			}
		})
	}
}

func TestUpgradeRefusesBusyInstance(t *testing.T) {
	i, id, source, _ := testInstaller(t)
	p, err := i.installNew(context.Background(), id, source, InstallCredentials{"manager.example:443", "key", "secret"})
	if err != nil {
		t.Fatal(err)
	}
	id.InstanceID = p.InstanceID
	i.check = func(context.Context, Identity, Command) (Plan, error) { return p, nil }
	i.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "stop") {
			t.Fatal("busy instance was stopped")
		}
		return []byte("123"), nil
	}
	if err = os.Mkdir(filepath.Join(filepath.Dir(p.Executable), ".lifecycle.lock"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = i.upgrade(context.Background(), id, source); err == nil {
		t.Fatal("busy instance accepted")
	}
	data, err := os.ReadFile(p.Executable)
	if err != nil || string(data) != "test executable" {
		t.Fatal("busy instance modified")
	}
}

func TestUpgradeRollbackStopFailureRetainsBackup(t *testing.T) {
	i, id, source, _ := testInstaller(t)
	p, err := i.installNew(context.Background(), id, source, InstallCredentials{"manager.example:443", "key", "secret"})
	if err != nil {
		t.Fatal(err)
	}
	id.InstanceID = p.InstanceID
	i.check = func(context.Context, Identity, Command) (Plan, error) { return p, nil }
	i.validate = func(string, int) error { return nil }
	stops := 0
	i.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--property=MainPID") {
			return []byte("123"), nil
		}
		if strings.Contains(joined, " stop ") {
			stops++
			if stops == 2 {
				return nil, errors.New("cannot stop replacement")
			}
		}
		return nil, nil
	}
	i.healthy = func(context.Context, Identity, Plan, Command) error { return errors.New("unhealthy") }
	if err = os.WriteFile(source, []byte("replacement"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = i.upgrade(context.Background(), id, source); err == nil || !strings.Contains(err.Error(), "backup retained") {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p.Executable + ".previous")
	if err != nil || string(data) != "test executable" {
		t.Fatal("lost recovery backup", err)
	}
	data, err = os.ReadFile(p.Executable)
	if err != nil || string(data) != "replacement" {
		t.Fatal("replaced a possibly live program", err)
	}
}
