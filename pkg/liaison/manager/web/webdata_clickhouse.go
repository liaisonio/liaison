package web

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func clickHouseOptions(s *webDataSession, password string, dial func(context.Context, string, string) (net.Conn, error)) (*clickhouse.Options, error) {
	if strings.TrimSpace(s.connectionParams) != "" {
		return nil, errors.New("custom ClickHouse connection parameters are not supported")
	}
	var tlsConfig *tls.Config
	switch s.tlsMode {
	case "", "disable":
	case "require", "true", "skip-verify":
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.target.TargetHost, InsecureSkipVerify: s.tlsMode == "skip-verify"} // Explicit compatibility option.
	default:
		return nil, errors.New("unsupported ClickHouse TLS mode")
	}
	address := net.JoinHostPort(s.target.TargetHost, strconv.Itoa(s.target.TargetPort))
	return &clickhouse.Options{
		Addr:        []string{address},
		Auth:        clickhouse.Auth{Database: firstNonEmpty(s.database, "default"), Username: firstNonEmpty(s.username, "default"), Password: password},
		DialTimeout: 15 * time.Second, ReadTimeout: 30 * time.Second,
		Settings: clickhouse.Settings{"max_execution_time": 30, "max_result_rows": 1001, "result_overflow_mode": "break"},
		DialContext: func(ctx context.Context, requested string) (net.Conn, error) {
			if requested != address {
				return nil, errors.New("ClickHouse target redirection is not supported")
			}
			conn, err := dial(ctx, "tcp", address)
			if err != nil || tlsConfig == nil {
				return conn, err
			}
			// The driver does not apply TLS when a custom connector dialer is used.
			secure := tls.Client(conn, tlsConfig)
			if err := secure.HandshakeContext(ctx); err != nil {
				conn.Close()
				return nil, err
			}
			return secure, nil
		},
	}, nil
}

func (web *web) openWebDataClickHouse(ctx context.Context, s *webDataSession, password string) error {
	opts, err := clickHouseOptions(s, password, web.webDataDeadlineSafeDialContext(s.proxyID, "clickhouse"))
	if err != nil {
		return err
	}
	db := clickhouse.OpenDB(opts)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(webDataSessionTTL)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return err
	}
	s.sqlDB = db
	s.database = opts.Auth.Database
	return nil
}

func (s *webDataSession) clickHouseMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	databaseRows, err := querySQLRowsAsMaps(ctx, s.sqlDB, `SELECT name FROM system.databases ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var databases []string
	seen := map[string]bool{}
	for _, row := range databaseRows {
		name := fmt.Sprint(row["name"])
		databases = append(databases, name)
		seen[name] = true
	}
	rows, err := s.sqlDB.QueryContext(ctx, `SELECT database, name, engine FROM system.tables ORDER BY database, name LIMIT 10000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []webDataTableRef
	for rows.Next() {
		var table webDataTableRef
		if err := rows.Scan(&table.Namespace, &table.Name, &table.Kind); err != nil {
			return nil, err
		}
		tables = append(tables, table)
		if !seen[table.Namespace] {
			seen[table.Namespace] = true
			databases = append(databases, table.Namespace)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return buildMySQLMetadata(databases, tables), nil
}

func (s *webDataSession) clickHouseColumns(ctx context.Context, database, name string) ([]map[string]any, error) {
	return querySQLRowsAsMaps(ctx, s.sqlDB, `SELECT name AS column_name, type AS data_type,
 if(startsWith(type, 'Nullable('), 'YES', 'NO') AS is_nullable,
 default_expression AS column_default, default_kind,
 if(default_kind IN ('MATERIALIZED','ALIAS'), 1, 0) AS is_computed,
 is_in_primary_key FROM system.columns WHERE database=? AND table=? ORDER BY position`, database, name)
}

func (s *webDataSession) clickHouseMetadataChildren(ctx context.Context, req webDataMetadataRequest) ([]webDataMetadataNode, error) {
	if req.NodeType != "table" || req.Name == "" {
		return nil, errors.New("select a ClickHouse table or view")
	}
	database := firstNonEmpty(req.Database, s.database)
	columns, err := s.clickHouseColumns(ctx, database, req.Name)
	if err != nil {
		return nil, err
	}
	nodes := make([]webDataMetadataNode, 0, len(columns))
	for _, col := range columns {
		name := fmt.Sprint(col["column_name"])
		nodes = append(nodes, webDataMetadataNode{Key: "clickhouse-column-" + database + "-" + req.Name + "-" + name, Title: name, Type: "column", Value: fmt.Sprint(col["data_type"]), Meta: map[string]string{"database": database, "name": req.Name, "column": name}})
	}
	return nodes, nil
}

func (s *webDataSession) clickHouseObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	if req.ObjectType != "table" || req.Name == "" {
		return nil, errors.New("select a ClickHouse table or view")
	}
	database := firstNonEmpty(req.Database, s.database)
	columns, err := s.clickHouseColumns(ctx, database, req.Name)
	if err != nil {
		return nil, err
	}
	var ddl string
	if err := s.sqlDB.QueryRowContext(ctx, `SELECT create_table_query FROM system.tables WHERE database=? AND name=?`, database, req.Name).Scan(&ddl); err != nil {
		return nil, err
	}
	return &webDataObjectResponse{ObjectType: "table", Database: database, Name: req.Name, Columns: columns, DDL: ddl, Message: database + "." + req.Name}, nil
}
