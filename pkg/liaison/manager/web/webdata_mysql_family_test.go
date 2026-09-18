package web

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMySQLFamily_QueryBoundaries(t *testing.T) {
	for _, protocol := range []string{"doris", "starrocks", "tidb"} {
		t.Run(protocol, func(t *testing.T) {
			require.True(t, webDataExecuteIsQuery(protocol, "SELECT 1"))
			require.True(t, webDataExecuteIsQuery(protocol, "SHOW DATABASES"))
			require.False(t, webDataExecuteIsQuery(protocol, "DELETE FROM accounts"))
			require.False(t, webDataExecuteIsQuery(protocol, "SELECT 1; DROP TABLE accounts"))
			require.Contains(t, webDataCapabilities(protocol), "sql")
		})
	}
	require.NotContains(t, webDataCapabilities("smb"), "execute")
}
