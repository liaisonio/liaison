//go:build integration

package web

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// DSNs must point to disposable test databases. This exercises the native SQL
// workspace, not the connector tunnel or browser; those require deployed E2E.
func TestSQLWorkspace_RealDatabases(t *testing.T) {
	for _, tc := range []struct{ protocol, driver, env string }{
		{"mariadb", "mysql", "TEST_MARIADB_DSN"},
		{"postgresql", "pgx", "TEST_POSTGRESQL_DSN"},
		{"sqlserver", "sqlserver", "TEST_SQLSERVER_DSN"},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if dsn == "" {
				t.Skip("set " + tc.env + " to a disposable database")
			}
			db, err := sql.Open(tc.driver, dsn)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			require.NoError(t, db.PingContext(ctx))
			var database string
			query := "SELECT DATABASE()"
			schema := "public"
			if tc.protocol == "sqlserver" {
				query = "SELECT DB_NAME()"
				schema = "dbo"
			}
			if tc.protocol == "postgresql" {
				query = "SELECT current_database()"
			}
			require.NoError(t, db.QueryRowContext(ctx, query).Scan(&database))
			session := &webDataSession{protocol: tc.protocol, sqlDB: db, database: database}
			name := fmt.Sprintf("liaison_test_%d", time.Now().UnixNano())
			_, err = session.execute(ctx, "CREATE TABLE "+name+" (id INTEGER PRIMARY KEY, value VARCHAR(32))")
			require.NoError(t, err)
			t.Cleanup(func() {
				cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				_, err := db.ExecContext(cleanup, "DROP TABLE "+name)
				require.NoError(t, err)
			})
			_, err = session.execute(ctx, "INSERT INTO "+name+" VALUES (1, 'initial')")
			require.NoError(t, err)
			_, err = session.execute(ctx, "UPDATE "+name+" SET value='verified' WHERE id=1")
			require.NoError(t, err)
			preview := "SELECT value FROM " + name + " LIMIT 1"
			if tc.protocol == "sqlserver" {
				preview = "SELECT TOP (1) value FROM " + name
			}
			result, err := session.execute(ctx, preview)
			require.NoError(t, err)
			require.Len(t, result.Rows, 1)
			require.Equal(t, "verified", result.Rows[0]["value"])
			nodes, err := session.metadata(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, nodes)
			children, err := session.metadataChildren(ctx, webDataMetadataRequest{NodeType: "table", Database: database, Schema: schema, Name: name})
			require.NoError(t, err)
			require.Len(t, children, 2)
			detail, err := session.objectDetails(ctx, webDataObjectRequest{ObjectType: "table", Database: database, Schema: schema, Name: name})
			require.NoError(t, err)
			require.Len(t, detail.Columns, 2)
			require.NotEmpty(t, detail.Indexes)
			_, err = session.execute(ctx, "SELECT invalid_column FROM "+name)
			require.Error(t, err)
		})
	}
}
