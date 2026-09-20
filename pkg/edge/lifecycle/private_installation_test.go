package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateManifestBinding(t *testing.T) {
	id := Identity{OS: "darwin", UID: 501, Home: "/Users/test", InstanceID: strings.Repeat("a", 32)}
	plan, err := InstallationPlan(id)
	if err != nil {
		t.Fatal(err)
	}
	base := InstallManifest{Version: 1, InstanceID: id.InstanceID, Platform: id.OS, UID: id.UID, Service: plan.Service}
	for _, tc := range []struct {
		name   string
		change func(*InstallManifest)
		valid  bool
	}{
		{"matching", func(*InstallManifest) {}, true},
		{"other_instance", func(m *InstallManifest) { m.InstanceID = strings.Repeat("b", 32) }, false},
		{"other_user", func(m *InstallManifest) { m.UID++ }, false},
		{"other_platform", func(m *InstallManifest) { m.Platform = "linux" }, false},
		{"other_service", func(m *InstallManifest) { m.Service = "com.cloud.edge" }, false},
		{"unsupported_version", func(m *InstallManifest) { m.Version++ }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			tc.change(&m)
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if got := validatePrivateManifest(data, id, plan); (got == nil) != tc.valid {
				t.Fatalf("unexpected validation: %v", got)
			}
		})
	}
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append(append([]byte{}, data...), []byte("{}")...), []byte(`{"paths":["/etc"]}`), nil} {
		if err := validatePrivateManifest(bad, id, plan); err == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
}

func TestPrivateScopeRejectsLegacyAndSharedDirectory(t *testing.T) {
	id := Identity{OS: "darwin", UID: 501, Home: "/Users/test"}
	p, err := InstallationPlan(id)
	if err != nil {
		t.Fatal(err)
	}
	if err = validatePrivateInstallation(id, p); err == nil {
		t.Fatal("legacy scope was relaxed")
	}
	root := t.TempDir()
	base := filepath.Join(root, "instance")
	if err = os.MkdirAll(filepath.Join(base, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(base, 0755); err != nil {
		t.Fatal(err)
	}
	p.Legacy = false
	p.InstanceID = strings.Repeat("a", 32)
	p.Executable = filepath.Join(base, "bin/liaison-edge")
	if err = validatePrivateInstallation(id, p); err == nil {
		t.Fatal("shared-readable instance accepted")
	}
}
