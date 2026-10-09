//go:build darwin

package webide

import (
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProcessArgumentsPreserveSpacesAndExcludeEnvironment(t *testing.T) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 4)
	data = append(data, []byte("/App Support/node\x00\x00node\x00--user-data-dir\x00/App Support/IDE/data\x00\x00SECRET=hidden\x00")...)
	args, err := parseProcessArgs(data)
	require.NoError(t, err)
	require.Equal(t, []string{"node", "--user-data-dir", "/App Support/IDE/data", ""}, args)
	_, err = parseProcessArgs(data[:8])
	require.Error(t, err)
}
