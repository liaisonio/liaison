package lifecycle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func linuxIdentity() (Identity, Service) {
	id := Identity{OS: "linux", UID: 0, PID: 123, Executable: "/usr/local/bin/liaison-edge", Config: "/etc/liaison/liaison-edge.yaml"}
	return id, Service{Name: "liaison-edge.service", File: "/etc/systemd/system/liaison-edge.service", PID: 123, Arguments: []string{id.Executable, "-c", id.Config}}
}

func TestBuildPlanLegacyAndIsolated(t *testing.T) {
	for _, platform := range []string{"linux", "darwin"} {
		for _, instance := range []string{"", "00112233445566778899aabbccddeeff"} {
			t.Run(platform+instance, func(t *testing.T) {
				id, svc := linuxIdentity()
				id.OS = platform
				id.InstanceID = instance
				if platform == "linux" && instance != "" {
					id.Executable = "/opt/liaison/edges/" + instance + "/bin/liaison-edge"
					id.Config = "/etc/liaison/edges/" + instance + "/config.yaml"
					svc.Name = "liaison-edge-" + instance + ".service"
					svc.File = "/etc/systemd/system/" + svc.Name
				}
				if platform == "darwin" {
					id.UID = 501
					id.Home = "/Users/example"
					base := id.Home + "/Library/Application Support/liaison"
					svc.Name = "com.liaison.edge"
					name := "liaison-edge.yaml"
					if instance != "" {
						base += "/edges/" + instance
						svc.Name += "." + instance
						name = "config.yaml"
					}
					id.Executable = base + "/bin/liaison-edge"
					id.Config = base + "/" + name
					svc.File = id.Home + "/Library/LaunchAgents/" + svc.Name + ".plist"
				}
				svc.Arguments = []string{id.Executable, "-c", id.Config}
				plan, err := BuildPlan(id, svc)
				require.NoError(t, err)
				require.Equal(t, instance == "", plan.Legacy)
				require.NotEmpty(t, plan.InstanceID)
				again, err := BuildPlan(id, svc)
				require.NoError(t, err)
				require.Equal(t, plan, again)
			})
		}
	}
}

func TestBuildPlanRejectsAmbiguousTargets(t *testing.T) {
	cases := map[string]func(*Identity, *Service){
		"wrong PID":        func(i *Identity, s *Service) { s.PID++ },
		"cloud service":    func(i *Identity, s *Service) { s.Name = "ongrid-edge.service" },
		"other executable": func(i *Identity, s *Service) { s.Arguments[0] = "/tmp/liaison-edge" },
		"other config":     func(i *Identity, s *Service) { s.Arguments[2] = "/etc/other.yaml" },
		"non root":         func(i *Identity, s *Service) { i.UID = 1000 },
		"unexpected flags": func(i *Identity, s *Service) { s.Arguments = append(s.Arguments, "--uninstall") },
		"traversal":        func(i *Identity, s *Service) { i.InstanceID = "../../" },
		"custom config":    func(i *Identity, s *Service) { i.Config = "/root/shared.yaml"; s.Arguments[2] = i.Config },
		"custom service":   func(i *Identity, s *Service) { s.File = "/tmp/liaison-edge.service" },
		"Windows":          func(i *Identity, s *Service) { i.OS = "windows" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			id, s := linuxIdentity()
			mutate(&id, &s)
			_, err := BuildPlan(id, s)
			require.ErrorIs(t, err, ErrUnsupported)
		})
	}
}

func TestValidateFileRejectsUnsafeFilesystem(t *testing.T) {
	// Tests only create files under t.TempDir. The complete ancestry check can
	// legitimately reject the temporary parent itself; all hostile paths must fail.
	dir := t.TempDir()
	file := filepath.Join(dir, "edge")
	require.NoError(t, os.WriteFile(file, []byte("fixture"), 0600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(file, link))
	require.Error(t, validateFile(link, os.Getuid()))
	require.NoError(t, os.Chmod(file, 0666))
	require.Error(t, validateFile(file, os.Getuid()))
	require.Error(t, validateFile(dir, os.Getuid()))
	require.Error(t, validateFile("relative", os.Getuid()))
	require.Error(t, validateFile(filepath.Join(dir, "missing"), os.Getuid()))
}
