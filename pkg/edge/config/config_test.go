package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestRemoteUninstallDefaultAndExplicitOptOut(t *testing.T) {
	previousFile, previousConf := file, Conf
	t.Cleanup(func() { file, Conf = previousFile, previousConf })
	for _, tc := range []struct {
		name, input string
		want        bool
	}{
		{"omitted", "manager: {}\n", true},
		{"enabled", "allow_remote_uninstall: true\n", true},
		{"disabled", "allow_remote_uninstall: false\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file = filepath.Join(t.TempDir(), "edge.yaml")
			require.NoError(t, os.WriteFile(file, []byte(tc.input), 0600))
			require.NoError(t, initConf())
			require.Equal(t, tc.want, Conf.AllowRemoteUninstall)
			encoded, err := yaml.Marshal(Conf)
			require.NoError(t, err)
			require.Contains(t, string(encoded), "allow_remote_uninstall:")
			require.NoError(t, os.WriteFile(file, encoded, 0600))
			require.NoError(t, initConf())
			require.Equal(t, tc.want, Conf.AllowRemoteUninstall, "roundtrip must preserve explicit opt-out")
		})
	}
}
