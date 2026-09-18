//go:build integration

package web

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

// Explicit disposable-service DSNs only. Skips are not evidence of compatibility.
// Exercises actual workspace SQL and metadata over each engine's wire protocol.
func TestMySQLFamily_RealMetadata(t *testing.T) {
	for _, protocol := range []string{"doris", "starrocks", "tidb"} {
		t.Run(protocol, func(t *testing.T) {
			dsn := os.Getenv("TEST_" + strings.ToUpper(protocol) + "_DSN")
			if dsn == "" {
				t.Skip("provide an isolated service DSN")
			}
			cfg, err := mysql.ParseDSN(dsn)
			require.NoError(t, err)
			cfg.ParseTime = true
			cfg.InterpolateParams = protocol != "tidb"
			db, err := sql.Open("mysql", cfg.FormatDSN())
			require.NoError(t, err)
			t.Cleanup(func() { db.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			s := &webDataSession{protocol: protocol, sqlDB: db, database: cfg.DBName}
			result, err := s.execute(ctx, "SELECT 1 AS adapter_check")
			require.NoError(t, err)
			require.Len(t, result.Rows, 1)
			nodes, err := s.metadata(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, nodes)
			_, err = s.execute(ctx, "SELECT nonexistent_liaison_test_column")
			require.Error(t, err)
		})
	}
}
