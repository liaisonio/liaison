package web

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/stretchr/testify/require"
)

func TestClickHouseOptions_ScopedTarget(t *testing.T) {
	s := &webDataSession{target: &controlplane.WebDataTarget{TargetHost: "db.example.test", TargetPort: 9000}}
	called := false
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		called = true
		require.Equal(t, "db.example.test:9000", address)
		return nil, errors.New("test tunnel unavailable")
	}
	options, err := clickHouseOptions(s, "secret", dial)
	require.NoError(t, err)
	require.Equal(t, "default", options.Auth.Database)
	require.Equal(t, "default", options.Auth.Username)
	_, err = options.DialContext(context.Background(), "other.example.test:9000")
	require.Error(t, err)
	require.False(t, called)
	_, err = options.DialContext(context.Background(), "db.example.test:9000")
	require.Error(t, err)
	require.True(t, called)
	s.connectionParams = "host=other"
	_, err = clickHouseOptions(s, "", dial)
	require.Error(t, err)
	s.connectionParams = ""
	s.tlsMode = "unsupported"
	_, err = clickHouseOptions(s, "", dial)
	require.Error(t, err)
}

func TestClickHouseIntegration_Workspace(t *testing.T) {
	address := os.Getenv("TEST_CLICKHOUSE_ADDRESS")
	if address == "" {
		t.Skip("set TEST_CLICKHOUSE_ADDRESS for real ClickHouse integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db := clickhouse.OpenDB(&clickhouse.Options{Addr: []string{address}, Auth: clickhouse.Auth{Database: "liaison_test", Username: "liaison_test", Password: os.Getenv("TEST_CLICKHOUSE_PASSWORD")}})
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.NoError(t, db.PingContext(ctx))
	s := &webDataSession{protocol: "clickhouse", database: "liaison_test", sqlDB: db}
	_, err := s.execute(ctx, `CREATE TABLE liaison_test.workspace_test (id UInt64, name String, value Nullable(Float64)) ENGINE=Memory`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DROP TABLE liaison_test.workspace_test`)
		require.NoError(t, err)
	})
	_, err = s.execute(ctx, `INSERT INTO liaison_test.workspace_test VALUES (1,'中文',NULL),(2,'hello',1.5)`)
	require.NoError(t, err)
	result, err := s.execute(ctx, `SELECT * FROM liaison_test.workspace_test ORDER BY id`)
	require.NoError(t, err)
	require.Len(t, result.Rows, 2)
	require.Equal(t, "中文", result.Rows[0]["name"])
	require.Nil(t, result.Rows[0]["value"])
	nodes, err := s.clickHouseMetadata(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, nodes)
	children, err := s.clickHouseMetadataChildren(ctx, webDataMetadataRequest{NodeType: "table", Database: "liaison_test", Name: "workspace_test"})
	require.NoError(t, err)
	require.Len(t, children, 3)
	detail, err := s.clickHouseObjectDetails(ctx, webDataObjectRequest{ObjectType: "table", Database: "liaison_test", Name: "workspace_test"})
	require.NoError(t, err)
	require.Contains(t, detail.DDL, "CREATE TABLE")
	result, err = s.execute(ctx, `EXPLAIN SELECT * FROM liaison_test.workspace_test LIMIT 1`)
	require.NoError(t, err)
	require.NotEmpty(t, result.Rows)
	_, err = s.execute(ctx, `SELECT missing_column FROM liaison_test.workspace_test`)
	require.Error(t, err)
}
